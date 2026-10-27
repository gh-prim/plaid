// Package problems records what needs the admin's attention (F007 R29): a
// GPU that fell back to software, a folder that went away, a refused
// provider key. Each problem is one row; it is reported where the server
// already logs it, and resolved when the condition goes away.
package problems

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Problem codes, as in the API's ProblemCode.
const (
	HWFallback         = "hw_fallback"
	FFmpegRestarted    = "ffmpeg_restarted"
	FolderUnavailable  = "folder_unavailable"
	FolderUnreadable   = "folder_unreadable"
	WatchLimit         = "watch_limit"
	ProviderKeyRefused = "provider_key_refused"
)

// Keep is how long a resolved problem stays listed (R29).
const Keep = 7 * 24 * time.Hour

// Reporter is what the services call. A nil Reporter is valid: use Of.
type Reporter interface {
	// Report records that a problem happens now; an open or resolved row
	// with the same code and params is bumped and reopened.
	Report(code string, params map[string]string)
	// Resolve marks the open problem with this code and params as fixed.
	Resolve(code string, params map[string]string)
}

// Of returns r, or a Reporter that does nothing when r is nil, so services
// built without one (tests, tools) need no checks.
func Of(r Reporter) Reporter {
	if r == nil {
		return nop{}
	}
	return r
}

type nop struct{}

func (nop) Report(string, map[string]string)  {}
func (nop) Resolve(string, map[string]string) {}

// Problem is one row.
type Problem struct {
	ID         string
	Code       string
	Params     map[string]string
	FirstSeen  time.Time
	LastSeen   time.Time
	Count      int
	ResolvedAt *time.Time
}

// Recorder stores problems in the database.
type Recorder struct {
	db  *sql.DB
	log *slog.Logger
	// Now is the clock; tests replace it.
	Now func() time.Time
}

// New returns a recorder over an open database.
func New(db *sql.DB, log *slog.Logger) *Recorder {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Recorder{db: db, log: log, Now: time.Now}
}

// writeTimeout bounds a report: callers are scans and transcodes, which
// must not wait on a busy database.
const writeTimeout = 5 * time.Second

// Report implements Reporter. Errors are logged, never returned: a problem
// that can't be recorded must not break what reported it.
func (r *Recorder) Report(code string, params map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	p, key := canonical(code, params)
	now := r.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `INSERT INTO problems (id, code, params, dedupe_key, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (dedupe_key) DO UPDATE SET last_seen = excluded.last_seen, count = count + 1, resolved_at = NULL`,
		uuid.Must(uuid.NewV7()).String(), code, p, key, now, now)
	if err != nil {
		r.log.Error("problem not recorded", "code", code, "err", err)
	}
}

// Resolve implements Reporter.
func (r *Recorder) Resolve(code string, params map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	_, key := canonical(code, params)
	_, err := r.db.ExecContext(ctx, `UPDATE problems SET resolved_at = ? WHERE dedupe_key = ? AND resolved_at IS NULL`,
		r.Now().UTC().Format(time.RFC3339), key)
	if err != nil {
		r.log.Error("problem not resolved", "code", code, "err", err)
	}
}

// List returns open problems and those resolved within Keep, newest first.
func (r *Recorder) List(ctx context.Context) ([]Problem, error) {
	since := r.Now().Add(-Keep).UTC().Format(time.RFC3339)
	rows, err := r.db.QueryContext(ctx, `SELECT id, code, params, first_seen, last_seen, count, resolved_at
		FROM problems WHERE resolved_at IS NULL OR resolved_at >= ? ORDER BY last_seen DESC, id DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Problem{}
	for rows.Next() {
		var (
			p                 Problem
			params, first, ls string
			resolved          sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.Code, &params, &first, &ls, &p.Count, &resolved); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(params), &p.Params); err != nil {
			return nil, err
		}
		p.FirstSeen, _ = time.Parse(time.RFC3339, first)
		p.LastSeen, _ = time.Parse(time.RFC3339, ls)
		if resolved.Valid {
			t, _ := time.Parse(time.RFC3339, resolved.String)
			p.ResolvedAt = &t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Purge deletes problems resolved more than Keep ago.
func (r *Recorder) Purge(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM problems WHERE resolved_at IS NOT NULL AND resolved_at < ?`,
		r.Now().Add(-Keep).UTC().Format(time.RFC3339))
	return err
}

// RunJanitor purges every hour until ctx is done.
func (r *Recorder) RunJanitor(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := r.Purge(ctx); err != nil && ctx.Err() == nil {
			r.log.Error("old problems not purged", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// secretNames are param names that must never be stored: a guard against
// a call site passing a token or a key by mistake.
var secretNames = []string{"token", "key", "secret", "password", "url"}

// canonical returns params as JSON (map keys are sorted by encoding/json)
// without secret-looking names, and the dedupe key.
func canonical(code string, params map[string]string) (string, string) {
	clean := make(map[string]string, len(params))
	for k, v := range params {
		lk := strings.ToLower(k)
		secret := false
		for _, s := range secretNames {
			if strings.Contains(lk, s) {
				secret = true
				break
			}
		}
		if !secret {
			clean[k] = v
		}
	}
	b, _ := json.Marshal(clean)
	return string(b), code + " " + string(b)
}
