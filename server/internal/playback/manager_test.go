package playback

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestManager() (*Manager, *fakeClock, *[]string) {
	clock := &fakeClock{now: time.Date(2026, 10, 27, 20, 0, 0, 0, time.UTC)}
	m := NewManager()
	m.Now = clock.Now
	var ended []string
	var mu sync.Mutex
	m.OnEnd = func(s Session) { mu.Lock(); ended = append(ended, s.ID); mu.Unlock() }
	return m, clock, &ended
}

var (
	ada   = auth.Principal{UserID: "u-ada", SessionID: "a-laptop", ProfileID: "p-ada", Role: auth.RoleMember}
	bob   = auth.Principal{UserID: "u-bob", SessionID: "b-phone", ProfileID: "p-bob", Role: auth.RoleMember}
	admin = auth.Principal{UserID: "u-root", SessionID: "r-1", ProfileID: "p-root", Role: auth.RoleAdmin}
)

func TestManagerOwnership(t *testing.T) {
	m, _, _ := newTestManager()
	s := m.Add(Session{UserID: ada.UserID, AuthSessionID: ada.SessionID, ItemID: "i1"})
	if s.ID == "" || s.Token() == "" {
		t.Fatal("no id or token")
	}
	if _, err := m.Get(ada, s.ID); err != nil {
		t.Errorf("owner: %v", err)
	}
	if _, err := m.Get(bob, s.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("another user: %v, want not found", err)
	}
	if _, err := m.Get(admin, s.ID); err != nil {
		t.Errorf("admin: %v", err)
	}
	if err := m.End(bob, s.ID, nil); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("another user ending it: %v", err)
	}
	if got, err := m.ByToken(s.Token()); err != nil || got.ID != s.ID {
		t.Errorf("by token: %v", err)
	}
	if _, err := m.ByToken("forged"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("forged token: %v", err)
	}
	if err := m.End(admin, s.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ByToken(s.Token()); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("token after the end: %v", err)
	}
}

func TestManagerLifetimes(t *testing.T) {
	m, clock, ended := newTestManager()
	idle := m.Add(Session{UserID: ada.UserID, AuthSessionID: ada.SessionID})
	daily := m.Add(Session{UserID: ada.UserID, AuthSessionID: ada.SessionID})

	// Used every day: alive until the 7-day cap.
	for day := 1; day <= 6; day++ {
		clock.Add(23 * time.Hour)
		if _, err := m.ByToken(daily.Token()); err != nil {
			t.Fatalf("day %d: %v", day, err)
		}
	}
	if _, err := m.ByToken(idle.Token()); !errors.Is(err, ErrSessionNotFound) {
		t.Error("a session idle for more than 24 h still works")
	}
	clock.Add(24 * time.Hour) // 6×23 h + 24 h > 7 days since creation
	if _, err := m.ByToken(daily.Token()); !errors.Is(err, ErrSessionNotFound) {
		t.Error("a 7-day-old session still works")
	}
	m.Reap(context.Background())
	if len(*ended) != 2 || len(m.List()) != 0 {
		t.Errorf("ended %v, %d left", *ended, len(m.List()))
	}
}

func TestManagerEndsSessionsOfSignedOutDevices(t *testing.T) {
	m, _, ended := newTestManager()
	gone := map[string]bool{}
	m.Alive = func(_ context.Context, id string) (bool, error) {
		if id == "db-down" {
			return false, errors.New("database is locked")
		}
		return !gone[id], nil
	}
	a := m.Add(Session{UserID: ada.UserID, AuthSessionID: ada.SessionID})
	b := m.Add(Session{UserID: bob.UserID, AuthSessionID: bob.SessionID})
	c := m.Add(Session{UserID: bob.UserID, AuthSessionID: "db-down"})
	gone[ada.SessionID] = true
	m.Reap(context.Background())
	if _, err := m.ByToken(a.Token()); !errors.Is(err, ErrSessionNotFound) {
		t.Error("playback of a revoked session survived the reaper")
	}
	for _, s := range []Session{b, c} {
		if _, err := m.ByToken(s.Token()); err != nil {
			t.Errorf("session %s ended: %v", s.AuthSessionID, err)
		}
	}
	m.EndAuthSession(bob.SessionID)
	if _, err := m.ByToken(b.Token()); !errors.Is(err, ErrSessionNotFound) {
		t.Error("EndAuthSession left the session")
	}
	if len(*ended) != 2 {
		t.Errorf("ended = %v", *ended)
	}
}

// TestManagerGuestOwnership covers F010-R5: a guest sees their own party
// sessions only, never an account's nor another guest's, and accounts
// never see a guest's (both have an empty UserID).
func TestManagerGuestOwnership(t *testing.T) {
	m := NewManager()
	cam := m.Add(Session{GuestID: "cam", AuthSessionID: guestAuthPrefix + "cam", PartyID: "p1"})
	hugo := m.Add(Session{GuestID: "hugo", AuthSessionID: guestAuthPrefix + "hugo", PartyID: "p1"})
	mine := m.Add(Session{UserID: "alex", AuthSessionID: "s1"})
	guest := auth.Principal{GuestID: "cam", PartyID: "p1"}
	if _, err := m.Get(guest, cam.ID); err != nil {
		t.Errorf("guest can't see own session: %v", err)
	}
	for _, id := range []string{hugo.ID, mine.ID} {
		if _, err := m.Get(guest, id); err == nil {
			t.Errorf("guest sees %s", id)
		}
	}
	if _, err := m.Get(auth.Principal{UserID: "", SessionID: "x"}, cam.ID); err == nil {
		t.Error("an empty account sees a guest's session")
	}
	if err := m.End(auth.Principal{UserID: "alex"}, cam.ID, nil); err == nil {
		t.Error("an account ended a guest's session")
	}
	if _, err := m.Get(auth.Principal{UserID: "root", Role: auth.RoleAdmin}, cam.ID); err != nil {
		t.Errorf("admin can't see guest session: %v", err)
	}
}
