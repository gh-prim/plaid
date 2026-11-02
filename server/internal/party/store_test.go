package party

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/db"
)

var t0 = time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC)

// fixture opens a database with two users (alex, mia), a profile each and a
// movie.
func fixture(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	now := ts(t0)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := d.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"alex", "mia", "sam"} {
		exec(`INSERT INTO users (id, username, display_name, password_hash, role, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?)`, u, u, u, "x", "member", now, now)
		exec(`INSERT INTO profiles (id, user_id, name, is_default, created_at, updated_at)
			VALUES (?,?,?,1,?,?)`, "p-"+u, u, u, now, now)
	}
	exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('lib','Films','movies',?,?)`, now, now)
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('nosferatu','lib','movie','Nosferatu',?,?)`, now, now)
	return NewStore(d)
}

func TestStoreCreateAndCode(t *testing.T) {
	ctx := context.Background()
	s := fixture(t)
	p := Party{ID: NewID(), Code: "K7P4QX", Status: StatusLive, ItemID: "nosferatu",
		OrganizerUserID: "alex", OrganizerProfileID: "p-alex", ControlMode: ControlHost, CreatedAt: t0}
	if err := s.Create(ctx, p, []string{"mia", "mia"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ByCode(ctx, "k7p-4qx")
	if err != nil || got.ID != p.ID || got.ItemID != "nosferatu" || !got.CreatedAt.Equal(t0) {
		t.Fatalf("ByCode = %+v, %v", got, err)
	}
	// The code is unique among parties not over…
	dup := p
	dup.ID = NewID()
	if err := s.Create(ctx, dup, nil); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("duplicate live code: %v", err)
	}
	// …and free again once the party ended.
	p.Status = StatusEnded
	end := t0.Add(2 * time.Hour)
	p.EndedAt = &end
	if err := s.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ByCode(ctx, "K7P4QX"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ended party found by code: %v", err)
	}
	if err := s.Create(ctx, dup, nil); err != nil {
		t.Fatalf("code reuse after end: %v", err)
	}
}

func TestStoreInvitesAndAnswers(t *testing.T) {
	ctx := context.Background()
	s := fixture(t)
	sched := t0.Add(24 * time.Hour)
	p := Party{ID: NewID(), Code: NewCode(), Status: StatusScheduled, ItemID: "nosferatu",
		OrganizerUserID: "alex", ControlMode: ControlEveryone, ScheduledAt: &sched, Note: "Popcorn", CreatedAt: t0}
	if err := s.Create(ctx, p, []string{"mia"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Invite(ctx, p.ID, []string{"mia", "sam"}, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.Answer(ctx, p.ID, "mia", AnswerYes, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.Answer(ctx, p.ID, "alex", AnswerYes, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("answer without invitation: %v", err)
	}
	inv, err := s.Invites(ctx, p.ID)
	if err != nil || len(inv) != 2 {
		t.Fatalf("invites = %+v, %v", inv, err)
	}
	if inv[0].UserID != "mia" || inv[0].Answer != AnswerYes || inv[1].Answer != AnswerPending {
		t.Errorf("invites = %+v", inv)
	}
	for _, u := range []string{"alex", "mia", "sam"} {
		list, err := s.ForUser(ctx, u)
		if err != nil || len(list) != 1 || list[0].Note != "Popcorn" || !list[0].ScheduledAt.Equal(sched) {
			t.Errorf("ForUser(%s) = %+v, %v", u, list, err)
		}
	}
	if ok, _ := s.IsInvited(ctx, p.ID, "sam"); !ok {
		t.Error("sam not invited")
	}
}

func TestStoreGuestsAndRemoval(t *testing.T) {
	ctx := context.Background()
	s := fixture(t)
	p := Party{ID: NewID(), Code: NewCode(), Status: StatusLive, ItemID: "nosferatu",
		OrganizerUserID: "alex", ControlMode: ControlHost, CreatedAt: t0}
	if err := s.Create(ctx, p, nil); err != nil {
		t.Fatal(err)
	}
	g, token, err := s.AddGuest(ctx, Guest{PartyID: p.ID, FirstName: "Camille", AvatarColor: 3, CreatedAt: t0})
	if err != nil || len(token) != 45 {
		t.Fatalf("AddGuest = %q, %v", token, err)
	}
	got, err := s.GuestByToken(ctx, token)
	if err != nil || got.ID != g.ID || got.FirstName != "Camille" || got.PartyID != p.ID {
		t.Fatalf("GuestByToken = %+v, %v", got, err)
	}
	if _, err := s.GuestByToken(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown token: %v", err)
	}
	if err := s.Remove(ctx, p.ID, "", g.ID, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GuestByToken(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed guest's token still works: %v", err)
	}
	if err := s.Remove(ctx, p.ID, "mia", "", t0); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.IsRemoved(ctx, p.ID, "mia"); !ok {
		t.Error("mia not removed")
	}

	// A guest's token dies with the party.
	_, token2, _ := s.AddGuest(ctx, Guest{PartyID: p.ID, FirstName: "Hugo", CreatedAt: t0})
	if err := s.EndLive(ctx, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GuestByToken(ctx, token2); !errors.Is(err, ErrNotFound) {
		t.Errorf("guest token outlived the party: %v", err)
	}
}

func TestCodes(t *testing.T) {
	for i := 0; i < 200; i++ {
		c := NewCode()
		if len(c) != 6 || NormalizeCode(c) != c {
			t.Fatalf("NewCode = %q", c)
		}
	}
	if got := NormalizeCode(" k7p-4qx "); got != "K7P4QX" {
		t.Errorf("NormalizeCode = %q", got)
	}
	if got := FormatCode("K7P4QX"); got != "K7P-4QX" {
		t.Errorf("FormatCode = %q", got)
	}
}
