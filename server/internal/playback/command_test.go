package playback

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gh-prim/plaid/server/internal/library"
)

var update = flag.Bool("update", false, "rewrite golden files")

func goldenMedia() Media {
	return Media{
		FileID: "f1", Path: "/media/Movies/Film (2001)/Film.mkv", Container: "mkv", DurationMs: 7_200_000, Bitrate: 12_000_000,
		Video: &library.Stream{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080},
		Audio: []library.Stream{
			{Index: 1, Type: "audio", Codec: "dts", Channels: 6, Language: "eng", Default: true},
			{Index: 2, Type: "audio", Codec: "ac3", Channels: 2, Language: "fre"},
		},
		Subtitles: []Subtitle{
			{Index: 3, Codec: "subrip", Language: "fre"},
			{Index: 4, Codec: "hdmv_pgs_subtitle", Language: "eng", Image: true},
			{Index: 5, Codec: "ass", Language: "eng"},
			{Index: 1000, Codec: "subrip", Language: "fre", Forced: true, ExternalPath: "/media/Movies/Film (2001)/Film.fr.forced.srt"},
		},
	}
}

func TestCommandGoldens(t *testing.T) {
	m := goldenMedia()
	tricky := m
	tricky.Path = "/media/It's: a, [test]; film\\.mkv"
	keyframes := make([]int64, 3600)
	for i := range keyframes {
		keyframes[i] = int64(i) * 2002
	}
	ds := StreamLayout(keyframes, m.DurationMs)
	tc := TranscodeLayout(m.DurationMs)
	transcode := func(mm Media, sub *Subtitle, delivery string) Decision {
		return Decision{Method: Transcode, Media: mm, Audio: &mm.Audio[0], AudioCodec: "aac", AudioChannels: 2,
			Subtitle: sub, SubtitleDelivery: delivery, Width: 1280, Height: 720, VideoBitrate: 4_000_000, VideoCodec: "h264"}
	}
	for _, c := range []struct {
		name string
		spec RunSpec
	}{
		{"direct-stream-copy-audio-start", RunSpec{Layout: ds, StartPiece: 0, Decision: Decision{Method: DirectStream, Media: m, VideoCopy: true, Audio: &m.Audio[1], AudioCopy: true, AudioCodec: "ac3", AudioChannels: 2}}},
		{"direct-stream-aac-seek", RunSpec{Layout: ds, StartPiece: 1530, Decision: Decision{Method: DirectStream, Media: m, VideoCopy: true, Audio: &m.Audio[0], AudioCodec: "aac", AudioChannels: 6}}},
		{"direct-stream-hevc", RunSpec{Layout: ds, Decision: func() Decision {
			h := m
			h.Video = &library.Stream{Index: 0, Type: "video", Codec: "hevc", Width: 3840, Height: 2160}
			return Decision{Method: DirectStream, Media: h, VideoCopy: true, Audio: &h.Audio[0], AudioCodec: "eac3", AudioChannels: 6}
		}()}},
		{"transcode-720p", RunSpec{Layout: tc, StartPiece: 0, Preset: PresetFast, Decision: transcode(m, nil, "")}},
		{"transcode-seek-balanced", RunSpec{Layout: tc, StartPiece: 1020, RunID: 3, Preset: PresetBalanced, Decision: transcode(m, nil, "")}},
		{"transcode-4k-quality", RunSpec{Layout: tc, Preset: PresetQuality, Decision: func() Decision {
			d := transcode(m, nil, "")
			d.Width, d.Height, d.VideoBitrate, d.AudioChannels = 3840, 2160, 20_000_000, 6
			return d
		}()}},
		{"burn-pgs", RunSpec{Layout: tc, Decision: transcode(m, &m.Subtitles[1], DeliveryBurnIn)}},
		{"burn-embedded-ass", RunSpec{Layout: tc, Decision: transcode(m, &m.Subtitles[2], DeliveryBurnIn)}},
		{"burn-external-srt", RunSpec{Layout: tc, Decision: transcode(m, &m.Subtitles[3], DeliveryBurnIn)}},
		{"burn-tricky-name", RunSpec{Layout: tc, Decision: transcode(tricky, &tricky.Subtitles[0], DeliveryBurnIn)}},
		{"no-audio", RunSpec{Layout: tc, Decision: func() Decision {
			d := transcode(m, nil, "")
			d.Audio = nil
			return d
		}()}},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.spec.Dir = "/var/lib/plaid/transcodes/plaid-0193"
			got := strings.Join(c.spec.Args(), "\n") + "\n"
			if strings.Contains(got, "/stream/") {
				t.Fatal("a command line must never carry a stream URL")
			}
			path := filepath.Join("testdata", "commands", c.name+".txt")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path) //nolint:gosec // test golden
			if err != nil {
				t.Fatalf("%v (run with -update to create it)", err)
			}
			if got != string(want) {
				t.Fatalf("args differ from %s:\n%s", path, got)
			}
		})
	}
}

func TestEscapeFilterValue(t *testing.T) {
	for in, want := range map[string]string{
		"/m/a.mkv":     "/m/a.mkv",
		"/m/It's.mkv":  `/m/It\\\'s.mkv`,
		"/m/a: b.mkv":  `/m/a\\: b.mkv`,
		"/m/a, b.mkv":  `/m/a\, b.mkv`,
		"/m/[x];y.mkv": `/m/\[x\]\;y.mkv`,
		`C:\m\a.mkv`:   `C\\:\\\\m\\\\a.mkv`,
	} {
		if got := escapeFilterValue(in); got != want {
			t.Errorf("escapeFilterValue(%q) = %q, want %q", in, got, want)
		}
	}
}
