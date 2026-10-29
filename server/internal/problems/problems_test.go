package problems

import (
	"context"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
)

func newRecorder(t *testing.T) (*Recorder, *time.Time) {
	t.Helper()
	d, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	r := New(d, nil)
	r.Now = func() time.Time { return now }
	return r, &now
}

func TestReportDedupesAndReopens(t *testing.T) {
	ctx := context.Background()
	r, now := newRecorder(t)
	nfs := map[string]string{"path": "/mnt/nas/Films"}

	r.Report(FolderUnavailable, nfs)
	*now = now.Add(time.Hour)
	r.Report(FolderUnavailable, map[string]string{"path": "/mnt/nas/Films"})
	r.Report(FolderUnavailable, map[string]string{"path": "/mnt/nas/Séries"})

	list, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d problems, want 2", len(list))
	}
	var films Problem
	for _, p := range list {
		if p.Params["path"] == "/mnt/nas/Films" {
			films = p
		}
	}
	if films.Count != 2 || !films.LastSeen.Equal(*now) || films.FirstSeen.Equal(*now) {
		t.Errorf("films = %+v, want count 2, last seen now, first seen an hour ago", films)
	}

	r.Resolve(FolderUnavailable, nfs)
	list, _ = r.List(ctx)
	for _, p := range list {
		if p.Params["path"] == "/mnt/nas/Films" && p.ResolvedAt == nil {
			t.Error("resolve: still open")
		}
	}
	r.Report(FolderUnavailable, nfs)
	list, _ = r.List(ctx)
	for _, p := range list {
		if p.Params["path"] == "/mnt/nas/Films" && (p.ResolvedAt != nil || p.Count != 3) {
			t.Errorf("reopen: %+v, want open with count 3", p)
		}
	}
}

func TestResolvedProblemsExpire(t *testing.T) {
	ctx := context.Background()
	r, now := newRecorder(t)
	r.Report(HWFallback, map[string]string{"accel": "vaapi"})
	r.Resolve(HWFallback, map[string]string{"accel": "vaapi"})
	r.Report(WatchLimit, map[string]string{"path": "/srv/media"})

	*now = now.Add(Keep + time.Minute)
	list, _ := r.List(ctx)
	if len(list) != 1 || list[0].Code != WatchLimit {
		t.Fatalf("after 7 days: %+v, want only the open watch_limit", list)
	}
	if err := r.Purge(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM problems`).Scan(&n); err != nil || n != 1 {
		t.Errorf("rows after purge = %d (%v), want 1", n, err)
	}
}

func TestSecretParamsAreDropped(t *testing.T) {
	ctx := context.Background()
	r, _ := newRecorder(t)
	r.Report(FFmpegRestarted, map[string]string{"title": "Dune", "token": "abc", "streamURL": "/stream/abc/x", "apiKey": "k"})
	list, _ := r.List(ctx)
	if len(list) != 1 || len(list[0].Params) != 1 || list[0].Params["title"] != "Dune" {
		t.Errorf("params = %v, want only title", list[0].Params)
	}
}

func TestOfNil(_ *testing.T) {
	Of(nil).Report(HWFallback, nil) // must not panic
	Of(nil).Resolve(HWFallback, nil)
}
