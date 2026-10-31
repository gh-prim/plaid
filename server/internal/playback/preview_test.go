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

// One input per scene, H.264 that starts before it's loaded, its sound
// faded and joined when the file has one, tone mapped from HDR only when
// the build can.
func TestPreviewArgs(t *testing.T) {
	src := previewSource{path: "/m/a.mkv", duration: time.Hour, audio: 2}
	full := &FFmpeg{Filters: map[string]bool{"zscale": true, "tonemap": true}}
	args := previewArgs(src, full, "/c/x.mp4", true)
	if n := strings.Count(strings.Join(args, " "), "-i /m/a.mkv"); n != previewScenes {
		t.Errorf("%d inputs, want %d", n, previewScenes)
	}
	for _, want := range []string{"[aout]", "aac", "libx264", "+faststart", "/c/x.mp4"} {
		if !slices.Contains(args, want) {
			t.Errorf("args lack %q: %v", want, args)
		}
	}
	graph := args[slices.Index(args, "-filter_complex")+1]
	if !strings.Contains(graph, "[0:2]aformat=") || !strings.Contains(graph, "afade=t=out:st=5.600") ||
		!strings.Contains(graph, "[v3][a3]concat=n=4:v=1:a=1[out][aout]") || strings.Contains(graph, "tonemap") {
		t.Errorf("SDR graph with sound: %s", graph)
	}

	src.audio = -1
	args = previewArgs(src, full, "/c/x.mp4", true)
	graph = args[slices.Index(args, "-filter_complex")+1]
	if !slices.Contains(args, "-an") || !strings.Contains(graph, "concat=n=4:v=1:a=0[out]") || strings.Contains(graph, "aformat") {
		t.Errorf("silent file: %v", args)
	}

	src.hdr = true
	if graph := previewArgs(src, full, "/c/x.mp4", true); !strings.Contains(strings.Join(graph, " "), "tonemap=hable") {
		t.Error("HDR source isn't tone mapped")
	}
	if graph := previewArgs(src, &FFmpeg{}, "/c/x.mp4", false); strings.Contains(strings.Join(graph, " "), "tonemap") {
		t.Error("tone mapping asked of a build without zscale")
	}
}

// The default sound track, else the first; none for a silent file.
func TestPreviewAudio(t *testing.T) {
	v := library.MediaStream{Index: 0, Type: "video"}
	en := library.MediaStream{Index: 1, Type: "audio", Language: "eng"}
	fr := library.MediaStream{Index: 2, Type: "audio", Language: "fre", Default: true}
	sub := library.MediaStream{Index: 3, Type: "subtitle", Default: true}
	for _, c := range []struct {
		streams []library.MediaStream
		want    int
	}{
		{[]library.MediaStream{v, en, fr, sub}, 2},
		{[]library.MediaStream{v, en, sub}, 1},
		{[]library.MediaStream{v, sub}, -1},
	} {
		if got := previewAudio(c.streams); got != c.want {
			t.Errorf("previewAudio(%v) = %d, want %d", c.streams, got, c.want)
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
