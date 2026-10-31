package handlers_test

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The hero preview: 202 while it's cut, then a silent MP4 served with
// Range; a short file or an unknown item has none.
func TestItemPreview(t *testing.T) {
	e := newHLSEnv(t)
	long := "/items/" + e.items["Long"] + "/preview"

	r := e.do("GET", long, nil, bearer(e.admin))
	if r.Code != http.StatusAccepted || r.Header().Get("Retry-After") == "" {
		t.Fatalf("first ask: %d %s, want 202 with Retry-After", r.Code, r.Body)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for r.Code == http.StatusAccepted && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		r = e.do("GET", long, nil, bearer(e.admin))
	}
	if r.Code != http.StatusOK || r.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("preview: %d %q", r.Code, r.Header().Get("Content-Type"))
	}
	clip := filepath.Join(t.TempDir(), "p.mp4")
	if err := os.WriteFile(clip, r.Body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_type:format=duration",
		"-of", "default=nw=1", clip).Output()
	if err != nil {
		t.Fatal(err)
	}
	if s := string(out); strings.Contains(s, "audio") || !strings.Contains(s, "codec_type=video") {
		t.Errorf("preview streams: %s", s)
	}

	r = e.do("GET", long, nil, bearer(e.admin), header("Range", "bytes=0-99"))
	if r.Code != http.StatusPartialContent || r.Body.Len() != 100 {
		t.Errorf("range: %d, %d bytes", r.Code, r.Body.Len())
	}
	e.expect(e.do("GET", "/items/"+e.items["Subs"]+"/preview", nil, bearer(e.admin)), http.StatusNotFound, "")
	e.expect(e.do("GET", "/items/nope/preview", nil, bearer(e.admin)), http.StatusNotFound, "")
	e.expect(e.do("GET", long, nil), http.StatusUnauthorized, "")
}

// With previews cut ahead of time, a title's preview is ready before
// anyone asks: the first request is already a 200.
func TestItemPreviewAhead(t *testing.T) {
	e := newHLSEnv(t, func(o *libOpts) { o.previewsAhead = true })
	long := "/items/" + e.items["Long"] + "/preview"
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if previewCached(t, e) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if r := e.do("HEAD", long, nil, bearer(e.admin)); r.Code != http.StatusOK {
		t.Fatalf("first ask after the cut ahead: %d, want 200", r.Code)
	}
}

// previewCached reports whether a preview file is in the cache yet,
// without asking for one (which would cut it urgently).
func previewCached(t *testing.T, e *hlsEnv) bool {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(e.play.CacheDir(), "previews", "*.mp4"))
	return len(m) > 0
}
