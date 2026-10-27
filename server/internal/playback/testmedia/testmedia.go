// Package testmedia generates short media files with FFmpeg for playback
// tests (F006). Clips are built once into a shared folder under the
// system temp directory and reused by every test package and run; a test
// skips when FFmpeg (or a needed encoder) is missing.
package testmedia

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Clip names.
const (
	MP4     = "h264-aac.mp4"      // 60 s H.264/AAC MP4 (B-frames, edit list)
	MKV     = "h264-ac3-subs.mkv" // 60 s H.264/AC-3 (French) MKV with an SRT (fre) and an ASS (eng) track
	HEVCHDR = "hevc-main10-hdr10.mkv"
	TS      = "h264.ts"  // 30 s MPEG-TS: no keyframe index
	Long    = "long.mkv" // 20 min 240p H.264/AAC MKV, keyframes every 2 s
	ExtSRT  = "external.fr.forced.srt"
	// Performance clips (-tags perf only, large).
	Perf1080p = "perf-1080p-h264.mkv" // 60 s 1080p24 H.264/AAC, 8 Mbps
	Perf2160p = "perf-2160p-hevc.mkv" // 30 s 4K HEVC Main 10/AAC, 20 Mbps
	GOPEvery  = 2 * time.Second       // keyframe interval of every clip
)

// recipe builds one clip. args go after "ffmpeg -y"; {out} and {dir} are
// replaced.
type recipe struct {
	args    []string
	encoder string // skip when missing
}

var src = []string{"-f", "lavfi", "-i", "testsrc=size=640x360:rate=24"}
var tone = []string{"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000"}

var recipes = map[string]recipe{
	MP4: {args: cat(src, tone, []string{"-t", "60", "-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-bf", "2", "-g", "48", "-crf", "32", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "64k", "{out}"})},
	MKV: {args: cat(src, tone, []string{"-i", "{dir}/subs.srt", "-i", "{dir}/subs.ass", "-t", "60",
		"-map", "0:v", "-map", "1:a", "-map", "2:s", "-map", "3:s",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "48", "-crf", "32", "-pix_fmt", "yuv420p",
		"-c:a", "ac3", "-b:a", "192k", "-ac", "6", "-c:s:0", "srt", "-c:s:1", "ass",
		"-metadata:s:a:0", "language=fre", "-metadata:s:s:0", "language=fre", "-metadata:s:s:1", "language=eng", "{out}"})},
	HEVCHDR: {encoder: "libx265", args: cat(src, tone, []string{"-t", "60", "-map", "0:v", "-map", "1:a",
		"-c:v", "libx265", "-preset", "ultrafast", "-pix_fmt", "yuv420p10le", "-x265-params",
		"log-level=error:keyint=48:min-keyint=48:hdr10=1:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:" +
			"master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,1):max-cll=1000,400",
		"-c:a", "aac", "-b:a", "64k", "{out}"})},
	TS: {args: cat(src, tone, []string{"-t", "30", "-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "48", "-crf", "32", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "64k", "{out}"})},
	Perf1080p: {args: cat([]string{"-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=24"}, tone, []string{"-t", "60",
		"-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-g", "48", "-b:v", "8M", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k", "{out}"})},
	Perf2160p: {encoder: "libx265", args: cat([]string{"-f", "lavfi", "-i", "testsrc2=size=3840x2160:rate=24"}, tone, []string{"-t", "30",
		"-map", "0:v", "-map", "1:a", "-c:v", "libx265", "-preset", "ultrafast", "-b:v", "20M", "-pix_fmt", "yuv420p10le",
		"-x265-params", "log-level=error:keyint=48:min-keyint=48", "-c:a", "aac", "-b:a", "128k", "{out}"})},
	Long: {args: cat([]string{"-f", "lavfi", "-i", "testsrc=size=426x240:rate=12"}, tone, []string{"-t", "1200",
		"-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "24", "-crf", "36", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "24k", "{out}"})},
}

// Subtitle sources muxed into MKV, and the external file.
const (
	srtText = "1\n00:00:01,000 --> 00:00:04,000\nBonjour\n\n2\n00:00:30,500 --> 00:00:33,250\nAu revoir\n"
	assText = "[Script Info]\nScriptType: v4.00+\nPlayResX: 640\nPlayResY: 360\n\n[V4+ Styles]\n" +
		"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,1,0,2,10,10,10,1\n\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:02.00,0:00:05.50,Default,,0,0,0,,{\\i1}Hello{\\i0}\n" +
		"Dialogue: 0,0:00:40.00,0:00:42.00,Default,,0,0,0,,Goodbye\n"
	extText = "1\n00:00:00,500 --> 00:00:02,000\nPanneau\n"
)

var (
	mu   sync.Mutex
	path string
)

// Dir returns the shared clip folder, creating it.
func Dir(t testing.TB) string {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	if path != "" {
		return path
	}
	h := sha256.New()
	_, _ = fmt.Fprint(h, recipes, srtText, assText, extText)
	dir := filepath.Join(os.TempDir(), "plaid-testmedia-"+hex.EncodeToString(h.Sum(nil))[:12])
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"subs.srt": srtText, "subs.ass": assText, ExtSRT: extText} {
		if err := writeOnce(filepath.Join(dir, name), []byte(text)); err != nil {
			t.Fatal(err)
		}
	}
	path = dir
	return dir
}

// FFmpeg returns the ffmpeg path, or skips the test.
func FFmpeg(t testing.TB) string {
	t.Helper()
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	return p
}

var building sync.Map // clip name → *sync.Mutex, so one process builds each clip once

// Clip returns the path of a generated clip, building it on first use.
func Clip(t testing.TB, name string) string {
	t.Helper()
	ffmpeg := FFmpeg(t)
	r, ok := recipes[name]
	if !ok {
		t.Fatalf("no clip %q", name)
	}
	dir := Dir(t)
	out := filepath.Join(dir, name)
	l, _ := building.LoadOrStore(name, &sync.Mutex{})
	l.(*sync.Mutex).Lock()
	defer l.(*sync.Mutex).Unlock()
	if _, err := os.Stat(out); err == nil {
		return out
	}
	if r.encoder != "" && !HasEncoder(t, r.encoder) {
		t.Skipf("ffmpeg has no %s encoder", r.encoder)
	}
	// Another test process may build the same clip: write to a private
	// name, then rename.
	tmp := filepath.Join(dir, fmt.Sprintf(".%d-%s", os.Getpid(), name))
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}
	for _, a := range r.args {
		a = strings.ReplaceAll(a, "{dir}", dir)
		args = append(args, strings.ReplaceAll(a, "{out}", tmp))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil { //nolint:gosec // test helper
		_ = os.Remove(tmp)
		t.Fatalf("building %s: %v\n%s", name, err, out)
	}
	if err := os.Rename(tmp, out); err != nil {
		t.Fatal(err)
	}
	return out
}

// HasEncoder reports whether ffmpeg lists an encoder.
func HasEncoder(t testing.TB, name string) bool {
	t.Helper()
	out, err := exec.CommandContext(context.Background(), FFmpeg(t), "-hide_banner", "-encoders").Output() //nolint:gosec // test helper
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[1] == name {
			return true
		}
	}
	return false
}

// Copy copies a clip to dir (for tests that change or rename it).
func Copy(t testing.TB, clip, dst string) string {
	t.Helper()
	b, err := os.ReadFile(clip) //nolint:gosec // test helper
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o600); err != nil { //nolint:gosec // test helper, paths from the test
		t.Fatal(err)
	}
	return dst
}

func writeOnce(p string, b []byte) error {
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	tmp := fmt.Sprintf("%s.%d.tmp", p, os.Getpid())
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func cat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
