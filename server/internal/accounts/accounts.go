// Package accounts owns users, profiles, sessions and TV pairing (F003).
//
// Methods return *apierr.Error for anything the client caused (validation,
// conflicts, wrong credentials); other errors are bugs or I/O failures.
package accounts

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gh-prim/plaid/server/internal/auth"
)

// PasswordHasher is the part of *auth.Hasher the service uses. Tests wrap it
// to count hashing work.
type PasswordHasher interface {
	Hash(ctx context.Context, secret string) (string, error)
	Verify(ctx context.Context, secret, encoded string) (bool, error)
	VerifyDummy(ctx context.Context, secret string) error
	NeedsRehash(encoded string) bool
}

// Options configures a Service. Zero values get production defaults.
type Options struct {
	Hasher PasswordHasher
	Now    func() time.Time
	Logger *slog.Logger
}

// Service implements accounts on top of SQLite.
type Service struct {
	db     *sql.DB
	hasher PasswordHasher
	now    func() time.Time
	logger *slog.Logger

	logins   *auth.Backoff // keys "u:<username>" and "ip:<addr>"
	pins     *auth.Backoff // keys "<session or user>:<profile>"
	claims   *auth.Backoff // keys "u:<user id>"
	pairings *auth.Backoff // keys "ip:<addr>"

	mu        sync.Mutex
	setupCode string
	// pairTokens holds the session token of a claimed pairing until the TV
	// polls it, so the token is never stored in clear (F003 data model).
	pairTokens map[string]pendingToken
}

type pendingToken struct {
	token   string
	expires time.Time
}

// New returns the accounts service.
func New(db *sql.DB, opts Options) *Service {
	if opts.Hasher == nil {
		opts.Hasher = auth.NewHasher(auth.DefaultParams)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	// Every time the service stores or returns is UTC, whatever the host's zone.
	clock := opts.Now
	opts.Now = func() time.Time { return clock().UTC() }
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		db:         db,
		hasher:     opts.Hasher,
		now:        opts.Now,
		logger:     opts.Logger,
		logins:     auth.NewBackoff(auth.LoginBackoff, opts.Now),
		pins:       auth.NewBackoff(auth.PINBackoff, opts.Now),
		claims:     auth.NewBackoff(auth.LoginBackoff, opts.Now),
		pairings:   auth.NewBackoff(auth.PairingCreateBackoff, opts.Now),
		pairTokens: map[string]pendingToken{},
	}
}

// User is an account.
type User struct {
	ID                 string
	Username           string
	DisplayName        string
	Role               auth.Role
	Locale             string
	MustChangePassword bool
	Disabled           bool
	CreatedAt          time.Time
}

// Profile is one person of an account.
type Profile struct {
	ID           string
	UserID       string
	Name         string
	AvatarColor  int
	HasPIN       bool
	MaxAge       *int
	AllowUnrated bool
	Locale       *string
	IsDefault    bool
}

// Device describes the client a session belongs to.
type Device struct {
	Name          string
	Type          string
	ClientVersion string
}

// Session is a signed-in device.
type Session struct {
	ID            string
	UserID        string
	ProfileID     string
	DeviceName    string
	DeviceType    string
	ClientVersion string
	LastIP        string
	CreatedAt     time.Time
	LastSeenAt    time.Time
	ExpiresAt     time.Time
}

// Me is the signed-in account as the client sees it.
type Me struct {
	User     User
	Profiles []Profile
	// Active is the session's profile, nil until one is picked.
	Active *Profile
}

// AuthResult is what sign-in, setup and pairing return.
type AuthResult struct {
	// Token is the clear session token. It exists only in this value.
	Token   string
	Session Session
	Me
}

// timeLayout is fixed-width RFC 3339 UTC, so stored times sort as text.
const timeLayout = "2006-01-02T15:04:05.000Z"

func ts(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

// newID returns a UUIDv7 (F003-R24).
func newID() string { return uuid.Must(uuid.NewV7()).String() }

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx runs fn in a transaction. With _txlock=immediate it holds the write
// lock from the start, so check-then-write sequences are atomic.
func (s *Service) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func isUniqueViolation(err error, column string) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: "+column)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
