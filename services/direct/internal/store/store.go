// Package store keeps registered servers and their ACME values in SQLite.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gh-prim/plaid/services/direct/internal/slug"

	_ "modernc.org/sqlite" // pure Go driver, keeps the binary CGO-free and static
)

//go:embed migrations/*.sql
var migrations embed.FS

// ErrNotFound is returned when no server has the requested id.
var ErrNotFound = errors.New("server not found")

// FamilyStaleAfter is how far behind lastSeen an address family may be and
// still be served. A dual-stack server heartbeats over one family at a time;
// past this, the other address is likely stale and is no longer published.
const FamilyStaleAfter = 24 * time.Hour

// Server is one registered PLAID server.
type Server struct {
	ID        string
	Slug      string
	TokenHash []byte
	IPv4      netip.Addr
	IPv4Seen  time.Time
	IPv6      netip.Addr
	IPv6Seen  time.Time
	Port      int
	Version   string
	CreatedAt time.Time
	LastSeen  time.Time
}

// Label is the server's first DNS label, "<slug>-<id>".
func (s Server) Label() string { return slug.Label(s.Slug, s.ID) }

// Seen records a request from ip at now: it refreshes lastSeen and the
// address of ip's family, leaving the other family as it was.
func (s *Server) Seen(ip netip.Addr, now time.Time) {
	ip = ip.Unmap()
	if ip.Is4() {
		s.IPv4, s.IPv4Seen = ip, now
	} else if ip.Is6() {
		s.IPv6, s.IPv6Seen = ip, now
	}
	s.LastSeen = now
}

// A returns the IPv4 address to publish, if any.
func (s Server) A() (netip.Addr, bool) {
	return s.IPv4, s.IPv4.IsValid() && s.LastSeen.Sub(s.IPv4Seen) <= FamilyStaleAfter
}

// AAAA returns the IPv6 address to publish, if any.
func (s Server) AAAA() (netip.Addr, bool) {
	return s.IPv6, s.IPv6.IsValid() && s.LastSeen.Sub(s.IPv6Seen) <= FamilyStaleAfter
}

// Store wraps the database.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) dataDir/direct.db in WAL mode and runs pending migrations.
func Open(ctx context.Context, dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, "direct.db")
	// _txlock=immediate takes the write lock up front, so busy_timeout covers
	// the whole transaction instead of failing on a read-to-write upgrade.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Ping checks the database answers, for health checks.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var applied int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE name = ?", name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied > 0 {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
	}
	return nil
}

// Create inserts srv under a fresh id from newID, retrying on collision, and
// sets srv.ID.
func (s *Store) Create(ctx context.Context, srv *Server, newID func() string) error {
	for range 10 {
		id := newID()
		res, err := s.db.ExecContext(ctx, `INSERT INTO servers
			(id, slug, token_hash, ipv4, ipv4_seen, ipv6, ipv6_seen, port, version, created_at, last_seen)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO NOTHING`,
			id, srv.Slug, srv.TokenHash, addrString(srv.IPv4), unix(srv.IPv4Seen), addrString(srv.IPv6), unix(srv.IPv6Seen),
			srv.Port, srv.Version, unix(srv.CreatedAt), unix(srv.LastSeen))
		if err != nil {
			return fmt.Errorf("insert server: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			srv.ID = id
			return nil
		}
	}
	return errors.New("insert server: no free id after 10 attempts")
}

// Get returns the server with this id, or ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (Server, error) {
	var (
		srv                   Server
		ipv4, ipv6            string
		v4Seen, v6Seen, c, ls int64
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, slug, token_hash, ipv4, ipv4_seen, ipv6, ipv6_seen, port, version, created_at, last_seen
		FROM servers WHERE id = ?`, id).
		Scan(&srv.ID, &srv.Slug, &srv.TokenHash, &ipv4, &v4Seen, &ipv6, &v6Seen, &srv.Port, &srv.Version, &c, &ls)
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	if err != nil {
		return Server{}, fmt.Errorf("get server: %w", err)
	}
	srv.IPv4, _ = netip.ParseAddr(ipv4)
	srv.IPv6, _ = netip.ParseAddr(ipv6)
	srv.IPv4Seen, srv.IPv6Seen = fromUnix(v4Seen), fromUnix(v6Seen)
	srv.CreatedAt, srv.LastSeen = fromUnix(c), fromUnix(ls)
	return srv, nil
}

// Update writes every mutable field of srv.
func (s *Store) Update(ctx context.Context, srv Server) error {
	res, err := s.db.ExecContext(ctx, `UPDATE servers SET slug = ?, ipv4 = ?, ipv4_seen = ?, ipv6 = ?, ipv6_seen = ?,
		port = ?, version = ?, last_seen = ? WHERE id = ?`,
		srv.Slug, addrString(srv.IPv4), unix(srv.IPv4Seen), addrString(srv.IPv6), unix(srv.IPv6Seen),
		srv.Port, srv.Version, unix(srv.LastSeen), srv.ID)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a server and its ACME values.
func (s *Store) Delete(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM servers WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	return nil
}

// SetACME replaces the server's ACME values; they expire at expires.
func (s *Store) SetACME(ctx context.Context, id string, values []string, expires time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM acme_values WHERE server_id = ?", id); err != nil {
		return fmt.Errorf("clear acme values: %w", err)
	}
	for _, v := range values {
		if _, err := tx.ExecContext(ctx, "INSERT INTO acme_values (server_id, value, expires_at) VALUES (?, ?, ?)", id, v, unix(expires)); err != nil {
			return fmt.Errorf("insert acme value: %w", err)
		}
	}
	return tx.Commit()
}

// ClearACME removes the server's ACME values.
func (s *Store) ClearACME(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM acme_values WHERE server_id = ?", id); err != nil {
		return fmt.Errorf("clear acme values: %w", err)
	}
	return nil
}

// ACMEValues returns the server's ACME values that haven't expired at now.
func (s *Store) ACMEValues(ctx context.Context, id string, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT value FROM acme_values WHERE server_id = ? AND expires_at > ? ORDER BY rowid", id, unix(now))
	if err != nil {
		return nil, fmt.Errorf("list acme values: %w", err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

// Purge deletes servers last seen before staleBefore and ACME values
// expired at now. It returns how many of each it removed.
func (s *Store) Purge(ctx context.Context, staleBefore, now time.Time) (servers, values int64, err error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM servers WHERE last_seen < ?", unix(staleBefore))
	if err != nil {
		return 0, 0, fmt.Errorf("purge servers: %w", err)
	}
	servers, _ = res.RowsAffected()
	res, err = s.db.ExecContext(ctx, "DELETE FROM acme_values WHERE expires_at <= ?", unix(now))
	if err != nil {
		return servers, 0, fmt.Errorf("purge acme values: %w", err)
	}
	values, _ = res.RowsAffected()
	return servers, values, nil
}

func addrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnix(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}
