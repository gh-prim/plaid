package playback

import (
	"slices"
	"strings"
	"testing"
	"time"
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

// One input per scene, silent H.264 that starts before it's loaded, tone
// mapped from HDR only when the build can.
func TestPreviewArgs(t *testing.T) {
	src := previewSource{path: "/m/a.mkv", duration: time.Hour}
	full := &FFmpeg{Filters: map[string]bool{"zscale": true, "tonemap": true}}
	args := previewArgs(src, full, "/c/x.mp4", true)
	if n := strings.Count(strings.Join(args, " "), "-i /m/a.mkv"); n != previewScenes {
		t.Errorf("%d inputs, want %d", n, previewScenes)
	}
	for _, want := range []string{"-an", "libx264", "+faststart", "/c/x.mp4"} {
		if !slices.Contains(args, want) {
			t.Errorf("args lack %q: %v", want, args)
		}
	}
	graph := args[slices.Index(args, "-filter_complex")+1]
	if !strings.Contains(graph, "concat=n=4:v=1:a=0[out]") || strings.Contains(graph, "tonemap") {
		t.Errorf("SDR graph: %s", graph)
	}
	src.hdr = true
	if graph := previewArgs(src, full, "/c/x.mp4", true); !strings.Contains(strings.Join(graph, " "), "tonemap=hable") {
		t.Error("HDR source isn't tone mapped")
	}
	if graph := previewArgs(src, &FFmpeg{}, "/c/x.mp4", false); strings.Contains(strings.Join(graph, " "), "tonemap") {
		t.Error("tone mapping asked of a build without zscale")
	}
}
