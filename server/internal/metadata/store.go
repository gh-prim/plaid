// Package metadata matches F004's items to TMDB and stores what it finds
// (F005). This file is the queue side of the store: which items need work
// and who is working on them.
package metadata

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"strings"
	"time"
)

// Store reads and writes the F005 tables.
type Store struct {
	db *sql.DB
	// Now is the clock; tests replace it.
	Now func() time.Time
}

// NewStore returns a store over an open database.
func NewStore(db *sql.DB) *Store { return &Store{db: db, Now: time.Now} }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func (s *Store) now() time.Time { return s.Now().UTC() }

// refreshBy brings an item's scheduled refresh forward to at, never back.
func (s *Store) refreshBy(ctx context.Context, itemID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE item_metadata SET refresh_at = ?
		WHERE item_id = ? AND (refresh_at IS NULL OR refresh_at > ?)`, ts(at), itemID, ts(at))
	return err
}

// AddPending creates a pending item_metadata row for each item that has
// none (R1). Existing rows are left alone.
func (s *Store) AddPending(ctx context.Context, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return nil
	}
	now := ts(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range itemIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO item_metadata (item_id, state, next_attempt_at, updated_at)
			SELECT id, 'pending', ?, ? FROM items WHERE id = ? ON CONFLICT (item_id) DO NOTHING`, now, now, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Sweep adds pending rows for every item without one, so items from a scan
// that failed half-way (or created before F005) get matched too. It returns
// how many rows it added.
func (s *Store) Sweep(ctx context.Context) (int, error) {
	now := ts(s.now())
	res, err := s.db.ExecContext(ctx, `INSERT INTO item_metadata (item_id, state, next_attempt_at, updated_at)
		SELECT id, 'pending', ?, ? FROM items i
		WHERE NOT EXISTS (SELECT 1 FROM item_metadata m WHERE m.item_id = i.id)`, now, now)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// Job is one movie or show to match or refresh. Seasons and episodes are
// handled with their show.
type Job struct {
	ItemID    string
	LibraryID string
	Kind      string // movie or show
	State     string
	Attempts  int
	// Refresh is true for a scheduled refresh of a matched item (R31).
	Refresh bool
	lease   string
}

// Priorities (R2): new items, then retries, then refreshes.
const dueQuery = `SELECT m.item_id, i.library_id, i.kind, m.state, m.attempts, COALESCE(m.next_attempt_at, ''),
	m.state <> 'pending' AS refresh
	FROM item_metadata m JOIN items i ON i.id = m.item_id
	WHERE i.kind IN ('movie', 'show') AND i.hidden = 0 AND (
		(m.state = 'pending' AND m.next_attempt_at <= ?)
		OR (m.state <> 'pending' AND m.refresh_at <= ? AND (m.next_attempt_at IS NULL OR m.next_attempt_at <= ?)))
	ORDER BY CASE WHEN m.state = 'pending' AND m.attempts = 0 THEN 0 WHEN m.state = 'pending' THEN 1 ELSE 2 END,
		COALESCE(m.next_attempt_at, m.refresh_at), m.item_id
	LIMIT 8`

// Claim takes the next due job and leases it for lease, so no other worker
// takes it. ok is false when nothing is due.
func (s *Store) Claim(ctx context.Context, lease time.Duration) (job Job, ok bool, err error) {
	now := s.now()
	n := ts(now)
	rows, err := s.db.QueryContext(ctx, dueQuery, n, n, n)
	if err != nil {
		return Job{}, false, err
	}
	var due []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ItemID, &j.LibraryID, &j.Kind, &j.State, &j.Attempts, &j.lease, &j.Refresh); err != nil {
			_ = rows.Close()
			return Job{}, false, err
		}
		due = append(due, j)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return Job{}, false, err
	}
	until := ts(now.Add(lease))
	for _, j := range due {
		// Compare-and-set on next_attempt_at: a worker that read the same
		// row first wins, the others try the next one.
		res, err := s.db.ExecContext(ctx, `UPDATE item_metadata SET next_attempt_at = ?
			WHERE item_id = ? AND COALESCE(next_attempt_at, '') = ?`, until, j.ItemID, j.lease)
		if err != nil {
			return Job{}, false, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			j.lease = until
			return j, true, nil
		}
	}
	return Job{}, false, nil
}

// Backoff after a failed attempt (R33): 1 min doubling up to 6 h, ±20 %.
func Backoff(attempts int) time.Duration {
	d := time.Minute << min(max(attempts-1, 0), 9)
	d = min(d, 6*time.Hour)
	jitter := 0.8 + 0.4*rand.Float64() //nolint:gosec // jitter, not security
	return time.Duration(float64(d) * jitter)
}

// Fail records a failed attempt and schedules the next one. Nothing else
// about the item changes (R33: errors never touch stored data).
func (s *Store) Fail(ctx context.Context, j Job, cause error) error {
	now := s.now()
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	attempts := j.Attempts + 1
	_, err := s.db.ExecContext(ctx, `UPDATE item_metadata SET attempts = ?, next_attempt_at = ?, last_error = ?, updated_at = ?
		WHERE item_id = ?`, attempts, ts(now.Add(Backoff(attempts))), msg, ts(now), j.ItemID)
	return err
}

// Release gives a job back without counting an attempt, for a later try:
// the provider is paused (no key, invalid key) rather than failing.
func (s *Store) Release(ctx context.Context, j Job, after time.Duration) error {
	_, err := s.db.ExecContext(ctx, `UPDATE item_metadata SET next_attempt_at = ? WHERE item_id = ?`,
		ts(s.now().Add(after)), j.ItemID)
	return err
}

// Unpause makes every waiting pending job due now: a key is there again,
// so the ones paused without it (and the ones backing off) go at once.
func (s *Store) Unpause(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE item_metadata SET next_attempt_at = ?
		WHERE state = 'pending' AND next_attempt_at > ?`, ts(s.now()), ts(s.now()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Counters are a library's metadata progress (R35).
type Counters struct {
	Pending, Matched, NeedsReview, Unmatched int
	// Refreshing are the looked-up titles whose refresh is due (V2 activity).
	Refreshing int
}

// Counts returns the counters of a library's movies and shows.
func (s *Store) Counts(ctx context.Context, libraryID string) (Counters, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT m.state, COUNT(*) FROM item_metadata m JOIN items i ON i.id = m.item_id
		WHERE i.library_id = ? AND i.kind IN ('movie', 'show') AND i.hidden = 0 GROUP BY m.state`, libraryID)
	if err != nil {
		return Counters{}, err
	}
	defer rows.Close()
	var c Counters
	for rows.Next() {
		var (
			state string
			n     int
		)
		if err := rows.Scan(&state, &n); err != nil {
			return Counters{}, err
		}
		switch state {
		case "pending":
			c.Pending = n
		case "matched":
			c.Matched = n
		case "needs_review":
			c.NeedsReview = n
		case "unmatched":
			c.Unmatched = n
		}
	}
	if err := rows.Err(); err != nil {
		return Counters{}, err
	}
	now := ts(s.now())
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_metadata m JOIN items i ON i.id = m.item_id
		WHERE i.library_id = ? AND i.kind IN ('movie', 'show') AND i.hidden = 0
		AND m.state <> 'pending' AND m.refresh_at <= ?`, libraryID, now).Scan(&c.Refreshing)
	return c, err
}

// ErrNotFound means the item or row doesn't exist.
var ErrNotFound = errors.New("not found")

// placeholders returns "?, ?, ?" for n values.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
