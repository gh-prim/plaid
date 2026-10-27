package playback

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
)

// Session lifetimes (R8, spec watch-out "tokens in URLs").
const (
	IdleTimeout   = 24 * time.Hour
	MaxSessionAge = 7 * 24 * time.Hour
	reapEvery     = 30 * time.Second
)

// ErrSessionNotFound is returned for unknown, ended and other users'
// sessions, which callers can't tell apart.
var ErrSessionNotFound = errors.New("playback_session_not_found")

// Session is one playback: who, what, how, and where it's at. The manager
// hands out copies; only the manager changes the live ones.
type Session struct {
	ID            string
	UserID        string
	AuthSessionID string // the F003 session it belongs to (R7)
	ProfileID     string
	ItemID        string
	ItemTitle     string
	Owner         Owner

	Decision Decision
	Request  Request
	Profile  Profile
	ClientIP netip.Addr
	Remote   bool

	CreatedAt       time.Time
	LastActivity    time.Time
	StartPositionMs int64
	PositionMs      int64
	State           string

	tokenHash string
	token     string // kept only to rebuild the session's URLs
	// Watch state bookkeeping (R32–R34).
	lastWrite time.Time
	counted   bool
	reported  bool
}

// Owner names who a session belongs to, for the admin view (R29).
type Owner struct {
	Username    string
	ProfileName string
	DeviceName  string
}

// Token is the session's playback token, which its URLs carry (R25).
func (s Session) Token() string { return s.token }

// Manager holds the live playback sessions in memory (R8).
type Manager struct {
	// Now is the clock; tests replace it.
	Now func() time.Time
	// Alive reports whether an F003 session can still be used; sessions
	// of dead ones end (R7). Nil keeps everything alive.
	Alive func(ctx context.Context, authSessionID string) (bool, error)
	// OnEnd runs, outside the lock, for every session that ends.
	OnEnd func(Session)
	Log   *slog.Logger

	mu      sync.Mutex
	byID    map[string]*Session
	byToken map[string]*Session
}

// NewManager returns an empty manager.
func NewManager() *Manager {
	return &Manager{Now: time.Now, Log: slog.Default(), byID: map[string]*Session{}, byToken: map[string]*Session{}}
}

func tokenKey(token string) string { return hex.EncodeToString(auth.HashToken(token)) }

// Add registers a new session, gives it an id and a token, and returns it.
func (m *Manager) Add(s Session) Session {
	now := m.Now()
	s.ID = newID()
	s.token = auth.NewToken()
	s.tokenHash = tokenKey(s.token)
	s.CreatedAt, s.LastActivity = now, now
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[s.ID] = &s
	m.byToken[s.tokenHash] = &s
	return s
}

// Get returns a session p may see: their own, or anyone's for an admin.
func (m *Manager) Get(p auth.Principal, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[id]
	if !ok || !m.live(s) || (s.UserID != p.UserID && !p.IsAdmin()) {
		return Session{}, ErrSessionNotFound
	}
	return *s, nil
}

// ByToken returns the session a stream request belongs to and counts the
// request as activity.
func (m *Manager) ByToken(token string) (Session, error) {
	key := tokenKey(token)
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byToken[key]
	if !ok || !m.live(s) {
		return Session{}, ErrSessionNotFound
	}
	s.LastActivity = m.Now()
	return *s, nil
}

// live reports whether a session is within its lifetimes. The reaper ends
// the others; until it runs, they're already unreachable.
func (m *Manager) live(s *Session) bool {
	now := m.Now()
	return now.Sub(s.LastActivity) < IdleTimeout && now.Sub(s.CreatedAt) < MaxSessionAge
}

// Update changes a session p may see, under the lock, and returns the
// result.
func (m *Manager) Update(p auth.Principal, id string, change func(*Session)) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byID[id]
	if !ok || !m.live(s) || (s.UserID != p.UserID && !p.IsAdmin()) {
		return Session{}, ErrSessionNotFound
	}
	change(s)
	s.LastActivity = m.Now()
	return *s, nil
}

// End ends a session p may see. A last change (the final position) is
// applied first.
func (m *Manager) End(p auth.Principal, id string, change func(*Session)) error {
	m.mu.Lock()
	s, ok := m.byID[id]
	if !ok || (s.UserID != p.UserID && !p.IsAdmin()) {
		m.mu.Unlock()
		return ErrSessionNotFound
	}
	if change != nil {
		change(s)
	}
	m.remove(s)
	ended := *s
	m.mu.Unlock()
	m.ended(ended, "stopped")
	return nil
}

func (m *Manager) remove(s *Session) {
	delete(m.byID, s.ID)
	delete(m.byToken, s.tokenHash)
}

func (m *Manager) ended(s Session, why string) {
	m.Log.Info("playback ended", "playback_id", s.ID, "user_id", s.UserID, "item_id", s.ItemID,
		"method", s.Decision.Method, "reason", why, "position_ms", s.PositionMs,
		"duration_s", int(m.Now().Sub(s.CreatedAt).Seconds()))
	if m.OnEnd != nil {
		m.OnEnd(s)
	}
}

// List returns every live session, oldest first.
func (m *Manager) List() []Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Session, 0, len(m.byID))
	for _, s := range m.byID {
		if m.live(s) {
			out = append(out, *s)
		}
	}
	slices.SortFunc(out, func(a, b Session) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out
}

// Reap ends sessions past their lifetimes and those whose F003 session
// is gone (revoked, expired, account disabled).
func (m *Manager) Reap(ctx context.Context) {
	m.mu.Lock()
	var expired []Session
	check := map[string][]*Session{}
	for _, s := range m.byID {
		if !m.live(s) {
			m.remove(s)
			expired = append(expired, *s)
			continue
		}
		check[s.AuthSessionID] = append(check[s.AuthSessionID], s)
	}
	m.mu.Unlock()
	for _, s := range expired {
		m.ended(s, "expired")
	}
	if m.Alive == nil {
		return
	}
	for authID, sessions := range check {
		ok, err := m.Alive(ctx, authID)
		if err != nil || ok {
			continue // a database error never ends playback
		}
		for _, s := range sessions {
			m.mu.Lock()
			_, still := m.byID[s.ID]
			if still {
				m.remove(s)
			}
			ended := *s
			m.mu.Unlock()
			if still {
				m.ended(ended, "signed_out")
			}
		}
	}
}

// EndAuthSession ends every playback of an F003 session at once, when a
// stream request finds it gone.
func (m *Manager) EndAuthSession(authID string) {
	m.mu.Lock()
	var ended []Session
	for _, s := range m.byID {
		if s.AuthSessionID == authID {
			m.remove(s)
			ended = append(ended, *s)
		}
	}
	m.mu.Unlock()
	for _, s := range ended {
		m.ended(s, "signed_out")
	}
}

// Run reaps every 30 s until ctx is done, then ends every session.
func (m *Manager) Run(ctx context.Context) {
	tick := time.NewTicker(reapEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			m.CloseAll()
			return
		case <-tick.C:
			m.Reap(ctx)
		}
	}
}

// CloseAll ends every session (server shutdown).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	all := make([]Session, 0, len(m.byID))
	for _, s := range m.byID {
		m.remove(s)
		all = append(all, *s)
	}
	m.mu.Unlock()
	for _, s := range all {
		m.ended(s, "shutdown")
	}
}
