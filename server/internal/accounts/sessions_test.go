package accounts

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
	"github.com/gh-prim/plaid/server/internal/db"
)

// TestSessionActive covers what F006 checks before serving a stream: every
// way a session ends makes it inactive (R7).
func TestSessionActive(t *testing.T) {
	ctx := clientinfo.WithInfo(context.Background(), clientinfo.Info{IP: netip.MustParseAddr("127.0.0.1")})
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	s := New(d, Options{Now: func() time.Time { return now }})
	if _, err := s.PrepareSetupCode(ctx); err != nil {
		t.Fatal(err)
	}
	admin, err := s.CreateFirstAdmin(ctx, SetupInput{
		NewUser: NewUser{DisplayName: "Ada", Username: "ada", Password: "correct horse battery", Role: auth.RoleAdmin, Locale: "en"},
		Device:  Device{Name: "Laptop", Type: "web"},
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Login(ctx, "ada", "correct horse battery", Device{Name: "TV", Type: "tvos"})
	if err != nil {
		t.Fatal(err)
	}
	active := func(id string) bool {
		t.Helper()
		ok, err := s.SessionActive(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !active(admin.Session.ID) || !active(other.Session.ID) {
		t.Fatal("fresh sessions inactive")
	}
	if active("nope") {
		t.Error("unknown session active")
	}

	p, err := s.Lookup(ctx, admin.Token)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.SessionOwner(ctx, p)
	if err != nil || owner.Username != "ada" || owner.DeviceName != "Laptop" || owner.ProfileName != "Ada" {
		t.Errorf("owner = %+v (%v)", owner, err)
	}

	if err := s.RevokeSession(ctx, p.UserID, other.Session.ID); err != nil {
		t.Fatal(err)
	}
	if active(other.Session.ID) {
		t.Error("revoked session still active")
	}
	now = now.Add(91 * 24 * time.Hour)
	if active(admin.Session.ID) {
		t.Error("expired session still active")
	}
}
