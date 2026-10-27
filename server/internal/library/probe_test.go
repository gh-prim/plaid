package library

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fixtureProber answers like ffprobe from testdata/probe/<name>.json and,
// for -show_frames, <name>.frames.json.
func fixtureProber(t *testing.T, name string) *FFprobe {
	t.Helper()
	return &FFprobe{Log: slog.Default(), run: func(_ context.Context, args ...string) ([]byte, error) {
		file := "testdata/probe/" + name + ".json"
		if slices.Contains(args, "-show_frames") {
			file = "testdata/probe/" + name + ".frames.json"
			if _, err := os.Stat(file); err != nil {
				return []byte(`{"frames": []}`), nil
			}
		}
		return os.ReadFile(file)
	}}
}

// streamSummary keeps the fields a reviewer can check at a glance.
func streamSummary(s Stream) string {
	var b strings.Builder
	b.WriteString(s.Type + ":" + s.Codec + ":" + s.Language)
	switch s.Type {
	case "video":
		b.WriteString(":" + itoa(s.Width) + "x" + itoa(s.Height) + ":" + itoa(s.BitDepth) + "bit")
		if s.HDR != "" {
			b.WriteString(":" + s.HDR)
		}
		if s.DVProfile > 0 {
			b.WriteString(":dv" + itoa(s.DVProfile) + "." + itoa(s.DVCompat))
		}
	case "audio":
		b.WriteString(":" + itoa(s.Channels) + "ch:" + s.Layout)
	case "subtitle":
		if s.Image {
			b.WriteString(":image")
		}
	}
	for _, f := range []struct {
		name string
		on   bool
	}{{":default", s.Default}, {":forced", s.Forced}, {":sdh", s.SDH}} {
		if f.on {
			b.WriteString(f.name)
		}
	}
	if s.Title != "" {
		b.WriteString(":" + s.Title)
	}
	return b.String()
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestProbeFixtures(t *testing.T) {
	cases := []struct {
		name      string
		container string
		duration  int64
		streams   []string
	}{
		{"sdr_h264_stereo", "matroska,webm", 1021, []string{
			"video:h264:und:1280x720:8bit",
			"audio:aac:eng:2ch:stereo",
		}},
		{"hevc_hdr10_multiaudio", "matroska,webm", 1021, []string{
			"video:hevc:und:3840x2160:10bit:hdr10",
			"audio:ac3:eng:6ch:5.1(side):default:Surround 5.1",
			"audio:aac:fre:2ch:stereo",
			"subtitle:subrip:fre:forced",
		}},
		{"hdr10plus", "matroska,webm", 1021, nil},
		{"dolby_vision_p81", "matroska,webm", 1021, nil},
		{"hlg", "mov,mp4,m4a,3gp,3g2,mj2", 1000, []string{
			"video:hevc:und:1920x1080:10bit:hlg:default",
		}},
		{"pgs_subtitles", "matroska,webm", 1021, []string{
			"video:h264:und:1280x720:8bit",
			"audio:aac:eng:2ch:stereo",
			"subtitle:hdmv_pgs_subtitle:ger:image:default:sdh:Deutsch SDH",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := fixtureProber(t, c.name).Probe(context.Background(), "/media/x.mkv")
			if err != nil {
				t.Fatal(err)
			}
			if res.Container != c.container || res.DurationMs != c.duration || res.Bitrate == 0 || len(res.Raw) == 0 {
				t.Errorf("format: container=%q duration=%d bitrate=%d", res.Container, res.DurationMs, res.Bitrate)
			}
			if c.streams == nil {
				return
			}
			var got []string
			for _, s := range res.Streams {
				got = append(got, streamSummary(s))
			}
			if !slices.Equal(got, c.streams) {
				t.Errorf("streams\ngot  %q\nwant %q", got, c.streams)
			}
		})
	}
}

func TestProbeHDRFormats(t *testing.T) {
	for name, want := range map[string]string{
		"sdr_h264_stereo":       "",
		"hevc_hdr10_multiaudio": HDR10,
		"hdr10plus":             HDR10Plus,
		"dolby_vision_p81":      DolbyVision,
		"hlg":                   HLG,
	} {
		res, err := fixtureProber(t, name).Probe(context.Background(), "/x")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		v := res.Video()
		if v.HDR != want {
			t.Errorf("%s: hdr = %q, want %q", name, v.HDR, want)
		}
		if name == "dolby_vision_p81" && (v.DVProfile != 8 || v.DVCompat != 1) {
			t.Errorf("dolby vision profile = %d.%d, want 8.1", v.DVProfile, v.DVCompat)
		}
	}
}

func TestProbeBrokenFile(t *testing.T) {
	if _, err := fixtureProber(t, "broken").Probe(context.Background(), "/x"); err == nil {
		t.Error("broken file probed without error")
	}
	failing := &FFprobe{Log: slog.Default(), run: func(context.Context, ...string) ([]byte, error) {
		return nil, errors.New("exit status 1: Invalid data found when processing input")
	}}
	if _, err := failing.Probe(context.Background(), "/x"); err == nil {
		t.Error("ffprobe error swallowed")
	}
}

func TestFindFFprobeMissing(t *testing.T) {
	_, err := FindFFprobe(context.Background(), filepath.Join(t.TempDir(), "ffprobe"), nil)
	if !errors.Is(err, ErrFFprobeMissing) {
		t.Errorf("err = %v, want ErrFFprobeMissing", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := FindFFprobe(context.Background(), "", nil); !errors.Is(err, ErrFFprobeMissing) {
		t.Errorf("empty PATH: err = %v, want ErrFFprobeMissing", err)
	}
}

// fakeFFprobe writes a shell script standing in for ffprobe.
func fakeFFprobe(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ffprobe")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil { //nolint:gosec // test script
		t.Fatal(err)
	}
	return p
}

func TestProbeTimeoutKillsFFprobe(t *testing.T) {
	path := fakeFFprobe(t, `if [ "$1" = "-version" ]; then echo "ffprobe version 9.9-fake"; exit 0; fi
exec sleep 30`)
	p, err := FindFFprobe(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != "9.9-fake" {
		t.Errorf("version = %q", p.Version)
	}
	p.Timeout = 200 * time.Millisecond
	start := time.Now()
	_, err = p.Probe(context.Background(), "/x.mkv")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("err = %v, want a timeout", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("probe took %v: the process wasn't killed", d)
	}
}

func TestProbeRealFile(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	p, err := FindFFprobe(context.Background(), "", nil)
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	file := filepath.Join(t.TempDir(), "sample.mkv")
	cmd := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=f=440", "-t", "1", "-c:v", "libx264", "-c:a", "aac",
		"-metadata:s:a:0", "language=fre", file)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg can't generate a sample: %v: %s", err, out)
	}
	res, err := p.Probe(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	v := res.Video()
	if v == nil || v.Codec != "h264" || v.Width != 320 || v.Height != 240 {
		t.Errorf("video = %+v", v)
	}
	if res.DurationMs < 900 || res.DurationMs > 1200 {
		t.Errorf("duration = %d ms", res.DurationMs)
	}
	var audio []string
	for _, s := range res.Streams {
		if s.Type == "audio" {
			audio = append(audio, s.Language)
		}
	}
	if !slices.Equal(audio, []string{"fre"}) {
		t.Errorf("audio languages = %v", audio)
	}
}
