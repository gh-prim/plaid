package playback

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
)

// The scenes sit in the first third, after the opening credits.
func TestPreviewScenesAt(t *testing.T) {
	d := 100 * time.Minute
	got := previewScenesAt(d)
	if len(got) != previewScenes {
		t.Fatalf("%d scenes, want %d", len(got), previewScenes)
	}
	if got[0] != 8*time.Minute {
		t.Errorf("first scene at %v, want 8m", got[0])
	}
	for i, at := range got {
		if at+previewSceneLength > 35*time.Minute {
			t.Errorf("scene %d at %v ends past the first third", i, at)
		}
		if i > 0 && at <= got[i-1] {
			t.Errorf("scenes out of order: %v", got)
		}
	}
}

// One scene per run, one input, threads bounded; its sound faded when
// the file has one; tone mapped from HDR only when the build can. The
// join copies.
func TestPreviewSceneArgs(t *testing.T) {
	src := previewSource{path: "/m/a.mkv", duration: time.Hour, audio: 2}
	full := &FFmpeg{Filters: map[string]bool{"zscale": true, "tonemap": true}}
	args := previewSceneArgs(src, full, 8*time.Minute, "/t/scene0.mp4", previewThreadsAhead)
	joined := strings.Join(args, " ")
	if n := strings.Count(joined, "-i "); n != 1 {
		t.Errorf("%d inputs, want 1: %s", n, joined)
	}
	for _, want := range []string{"-filter_threads 2", "-threads 2 -ss 480.000 -t 6.000 -i /m/a.mkv",
		"-map 0:2", "afade=t=out:st=5.600", "-c:a aac", "libx264", "/t/scene0.mp4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args lack %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "tonemap") || strings.Contains(joined, "faststart") {
		t.Errorf("SDR scene: %s", joined)
	}

	src.audio = -1
	if args := previewSceneArgs(src, full, 0, "/t/s.mp4", "4"); !slices.Contains(args, "-an") ||
		strings.Contains(strings.Join(args, " "), "aformat") {
		t.Errorf("silent file: %v", args)
	}
	src.hdr = true
	if args := previewSceneArgs(src, full, 0, "/t/s.mp4", "4"); !strings.Contains(strings.Join(args, " "), "tonemap=hable") {
		t.Error("HDR source isn't tone mapped")
	}
	if args := previewSceneArgs(src, &FFmpeg{}, 0, "/t/s.mp4", "2"); strings.Contains(strings.Join(args, " "), "tonemap") {
		t.Error("tone mapping asked of a build without zscale")
	}

	src = previewSource{path: "/m/a.mkv", duration: time.Hour, audio: 1, copy: true}
	joined = strings.Join(previewSceneArgs(src, full, time.Minute, "/t/s.mp4", "2"), " ")
	for _, want := range []string{"-noaccurate_seek -ss 60.000", "-c:v copy", "-c:a aac", "-avoid_negative_ts make_zero"} {
		if !strings.Contains(joined, want) {
			t.Errorf("copied scene lacks %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "libx264") || strings.Contains(joined, "scale=") {
		t.Errorf("copied scene encodes: %s", joined)
	}

	join := strings.Join(previewJoinArgs("/t/scenes.txt", "/c/x.mp4"), " ")
	if !strings.Contains(join, "-f concat -safe 0 -i /t/scenes.txt -c copy -movflags +faststart") {
		t.Errorf("join: %s", join)
	}
}

// The track in the first language the file has (a commentary only when
// nothing else is), else the default, else the first; none for a silent
// file.
func TestPreviewAudio(t *testing.T) {
	v := library.MediaStream{Index: 0, Type: "video"}
	por := library.MediaStream{Index: 1, Type: "audio", Language: "por", Default: true}
	frc := library.MediaStream{Index: 2, Type: "audio", Language: "fre", Title: "Commentaire du réalisateur"}
	fr := library.MediaStream{Index: 3, Type: "audio", Language: "fre"}
	en := library.MediaStream{Index: 4, Type: "audio", Language: "eng"}
	sub := library.MediaStream{Index: 5, Type: "subtitle", Language: "fre", Default: true}
	all := []library.MediaStream{v, por, frc, fr, en, sub}
	for _, c := range []struct {
		name    string
		streams []library.MediaStream
		langs   []string
		want    int
	}{
		{"the profile's language first", all, []string{"eng", "fr"}, 4},
		{"then the library's, 639-1 as well", all, []string{"", "fr"}, 3},
		{"an unknown language is skipped", all, []string{"xx", "fr"}, 3},
		{"a commentary only when it's all there is", []library.MediaStream{v, por, frc}, []string{"fre"}, 2},
		{"no match: the default", all, []string{"jpn", "de"}, 1},
		{"no default: the first", []library.MediaStream{v, fr, en, sub}, nil, 3},
		{"silent", []library.MediaStream{v, sub}, []string{"fre"}, -1},
	} {
		if got := previewAudio(c.streams, c.langs...); got != c.want {
			t.Errorf("%s: previewAudio = %d, want %d", c.name, got, c.want)
		}
	}
}

// A new recipe deletes the previews of the old one, and only those.
func TestDropOldPreviews(t *testing.T) {
	root := t.TempDir()
	dir := previewDir(root)
	for _, f := range []string{
		filepath.Join(root, "previews", "old.mp4"),
		filepath.Join(root, "previews", "v0", "a.mp4"),
		filepath.Join(dir, "keep.mp4"),
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if n := dropOldPreviews(dir); n != 2 {
		t.Errorf("dropped %d, want 2", n)
	}
	left, _ := filepath.Glob(filepath.Join(root, "previews", "*"))
	if !slices.Equal(left, []string{dir}) {
		t.Errorf("left %v, want only %s", left, dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.mp4")); err != nil {
		t.Error(err)
	}
	if n := dropOldPreviews(filepath.Join(t.TempDir(), "previews", previewVersion)); n != 0 {
		t.Errorf("no folder yet: dropped %d", n)
	}
}

// Only light, browser-ready H.264 is copied.
func TestPreviewCopy(t *testing.T) {
	file := func(v library.Stream, bitrate int64) library.PlayableFile {
		f := library.PlayableFile{Probe: &library.ProbeResult{Streams: []library.Stream{v}}}
		f.Bitrate = bitrate
		return f
	}
	h264 := library.Stream{Type: "video", Codec: "h264", Width: 1920, Height: 800, PixFmt: "yuv420p"}
	if !previewCopy(file(h264, 9_000_000), previewCopyMaxBitrate) {
		t.Error("1080p H.264 at 9 Mb/s isn't copied")
	}
	heavy, hevc, uhd, tenBit, hdr := h264, h264, h264, h264, h264
	hevc.Codec = "hevc"
	uhd.Width, uhd.Height = 3840, 2160
	tenBit.PixFmt = "yuv420p10le"
	hdr.HDR = "hdr10"
	for name, f := range map[string]library.PlayableFile{
		"over the bitrate": file(heavy, 30_000_000), "HEVC": file(hevc, 5_000_000), "4K": file(uhd, 9_000_000),
		"10-bit": file(tenBit, 5_000_000), "HDR": file(hdr, 5_000_000), "unknown bitrate": file(h264, 0),
		"not analyzed": {},
	} {
		if previewCopy(f, previewCopyMaxBitrate) {
			t.Errorf("%s is copied", name)
		}
	}
	if previewCopy(file(h264, 9_000_000), -1) {
		t.Error("copied though copies are off")
	}
}
