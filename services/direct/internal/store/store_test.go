package store

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateRetriesOnCollision(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	now := time.Unix(1_800_000_000, 0).UTC()

	first := &Server{Slug: "a", TokenHash: []byte{1}, Port: 1, CreatedAt: now, LastSeen: now}
	if err := s.Create(ctx, first, func() string { return "aaaaaa" }); err != nil {
		t.Fatal(err)
	}
	ids := []string{"aaaaaa", "aaaaaa", "bbbbbb"}
	second := &Server{Slug: "b", TokenHash: []byte{2}, Port: 2, CreatedAt: now, LastSeen: now}
	if err := s.Create(ctx, second, func() string { id := ids[0]; ids = ids[1:]; return id }); err != nil {
		t.Fatal(err)
	}
	if second.ID != "bbbbbb" {
		t.Fatalf("id = %q, want bbbbbb", second.ID)
	}
	if err := s.Create(ctx, &Server{Slug: "c", TokenHash: []byte{3}}, func() string { return "aaaaaa" }); err == nil {
		t.Fatal("want an error when every id collides")
	}
}

func TestRoundTripAndPurge(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	now := time.Unix(1_800_000_000, 0).UTC()

	srv := &Server{Slug: "living-room", TokenHash: []byte("hash"), Port: 32501, Version: "0.3.0", CreatedAt: now}
	srv.Seen(netip.MustParseAddr("203.0.113.7"), now)
	srv.Seen(netip.MustParseAddr("2001:db8::1"), now)
	if err := s.Create(ctx, srv, func() string { return "k3x7qa" }); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "k3x7qa")
	if err != nil {
		t.Fatal(err)
	}
	if got.Label() != "living-room-k3x7qa" || got.IPv4.String() != "203.0.113.7" || got.IPv6.String() != "2001:db8::1" ||
		!got.LastSeen.Equal(now) || got.Port != 32501 || got.Version != "0.3.0" {
		t.Fatalf("got %+v", got)
	}

	if err := s.SetACME(ctx, "k3x7qa", []string{"one", "two"}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	vals, _ := s.ACMEValues(ctx, "k3x7qa", now)
	if !slices.Equal(vals, []string{"one", "two"}) {
		t.Fatalf("values = %v", vals)
	}
	if vals, _ := s.ACMEValues(ctx, "k3x7qa", now.Add(time.Hour)); len(vals) != 0 {
		t.Fatalf("expired values still served: %v", vals)
	}

	// IPv4 heartbeats only: after a day the IPv6 address is no longer published.
	later := now.Add(25 * time.Hour)
	got.Seen(netip.MustParseAddr("203.0.113.8"), later)
	if err := s.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(ctx, "k3x7qa")
	if a, ok := got.A(); !ok || a.String() != "203.0.113.8" {
		t.Fatalf("A = %v %v", a, ok)
	}
	if _, ok := got.AAAA(); ok {
		t.Fatal("stale AAAA still published")
	}

	n, v, err := s.Purge(ctx, later, later)
	if err != nil || n != 0 || v != 2 {
		t.Fatalf("purge = %d %d %v, want 0 2", n, v, err)
	}
	n, _, err = s.Purge(ctx, later.Add(time.Second), later)
	if err != nil || n != 1 {
		t.Fatalf("purge = %d %v, want 1", n, err)
	}
	if _, err := s.Get(ctx, "k3x7qa"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
