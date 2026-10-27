package settings

import (
	"context"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
)

func TestSettings(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := New(d)
	s.Now = func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }

	if _, ok, err := s.Get(ctx, TMDBAPIKey); ok || err != nil {
		t.Fatalf("unset key: ok=%v err=%v", ok, err)
	}
	for _, v := range []string{"first", "second"} {
		if err := s.Set(ctx, TMDBAPIKey, v); err != nil {
			t.Fatal(err)
		}
		if got, ok, err := s.Get(ctx, TMDBAPIKey); !ok || err != nil || got != v {
			t.Errorf("Get = %q, %v, %v; want %q", got, ok, err, v)
		}
	}
	var updated string
	if err := d.QueryRowContext(ctx, `SELECT updated_at FROM settings WHERE key = ?`, TMDBAPIKey).Scan(&updated); err != nil || updated != "2026-10-27T12:00:00Z" {
		t.Errorf("updated_at = %q (%v)", updated, err)
	}
	if err := s.Delete(ctx, TMDBAPIKey); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, TMDBAPIKey); err != nil {
		t.Errorf("second delete: %v", err)
	}
	if _, ok, _ := s.Get(ctx, TMDBAPIKey); ok {
		t.Error("deleted key still set")
	}
}
