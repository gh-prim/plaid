package playback

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
)

// Watch thresholds (R33).
const (
	minResume      = 60 * time.Second
	watchedShare   = 0.9
	watchedLeft    = 2 * time.Minute
	progressWrites = 10 * time.Second
)

// UserState is a profile's watch state for an item (R36).
type UserState struct {
	PositionMs   int64
	DurationMs   int64 // of the file last played, 0 when unknown
	Watched      bool
	PlayCount    int
	LastPlayedAt *time.Time
	// Shows and seasons only.
	WatchedEpisodes, TotalEpisodes int
	Aggregate                      bool
}

// reachedEnd reports whether a position counts as having watched the item:
// 90% of it, or less than 2 minutes left once past the middle (so a
// 3-minute short isn't watched at second 0).
func reachedEnd(positionMs, durationMs int64) bool {
	if durationMs <= 0 || positionMs <= 0 {
		return false
	}
	left := time.Duration(durationMs-positionMs) * time.Millisecond
	return float64(positionMs) >= watchedShare*float64(durationMs) || (left < watchedLeft && positionMs*2 >= durationMs)
}

// WatchStore reads and writes watch_state.
type WatchStore struct {
	DB  *sql.DB
	Now func() time.Time
}

func (w *WatchStore) now() string { return w.Now().UTC().Format(time.RFC3339) }

// Save records a position (R33). watched marks the item; countPlay adds
// one play, once per session.
func (w *WatchStore) Save(ctx context.Context, profileID, itemID, fileID string, positionMs, durationMs int64, watched, countPlay bool) error {
	if watched || time.Duration(positionMs)*time.Millisecond < minResume {
		positionMs = 0
	}
	inc := 0
	if countPlay {
		inc = 1
	}
	now := w.now()
	_, err := w.DB.ExecContext(ctx, `
		INSERT INTO watch_state (profile_id, item_id, position_ms, duration_ms, watched, play_count, last_played_at, last_file_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (profile_id, item_id) DO UPDATE SET
			position_ms = excluded.position_ms, duration_ms = excluded.duration_ms,
			watched = MAX(watched, excluded.watched), play_count = play_count + ?,
			last_played_at = excluded.last_played_at, last_file_id = excluded.last_file_id,
			updated_at = excluded.updated_at`,
		profileID, itemID, positionMs, nullInt(durationMs), boolInt(watched), inc, now, nullString(fileID), now, inc)
	return err
}

// Mark marks an item watched or unwatched for a profile; a show or season
// marks every episode below it, in one transaction (R35).
func (w *WatchStore) Mark(ctx context.Context, profileID, itemID string, watched bool) error {
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM items WHERE id = ? AND kind IN ('movie', 'episode')
		UNION SELECT id FROM items WHERE parent_id = ? AND kind = 'episode'
		UNION SELECT e.id FROM items e JOIN items s ON e.parent_id = s.id WHERE s.parent_id = ? AND e.kind = 'episode'`,
		itemID, itemID, itemID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	now := w.now()
	for _, id := range ids {
		if watched {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO watch_state (profile_id, item_id, position_ms, watched, play_count, last_played_at, updated_at)
				VALUES (?, ?, 0, 1, 1, ?, ?)
				ON CONFLICT (profile_id, item_id) DO UPDATE SET
					position_ms = 0, watched = 1, play_count = MAX(play_count, 1),
					last_played_at = excluded.last_played_at, updated_at = excluded.updated_at`,
				profileID, id, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `
				UPDATE watch_state SET position_ms = 0, watched = 0, play_count = 0, updated_at = ?
				WHERE profile_id = ? AND item_id = ?`, now, profileID, id)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Queries of States, around a list of placeholders.
const (
	statesQuery = `SELECT item_id, position_ms, COALESCE(duration_ms, 0), watched, play_count, last_played_at FROM watch_state
		WHERE profile_id = ? AND item_id IN (`
	countsQuery = `
		WITH parents AS (SELECT id, kind FROM items WHERE id IN (`
	countsQueryEnd = `) AND kind IN ('show', 'season')),
		eps AS (
			SELECT p.id AS parent, e.id AS ep FROM parents p JOIN items e ON e.parent_id = p.id
			WHERE p.kind = 'season' AND e.kind = 'episode' AND e.hidden = 0
			UNION ALL
			SELECT p.id, e.id FROM parents p JOIN items s ON s.parent_id = p.id JOIN items e ON e.parent_id = s.id
			WHERE p.kind = 'show' AND e.kind = 'episode' AND e.hidden = 0
		)
		SELECT eps.parent, COUNT(*), COALESCE(SUM(w.watched), 0), MAX(w.last_played_at)
		FROM eps LEFT JOIN watch_state w ON w.item_id = eps.ep AND w.profile_id = ?
		GROUP BY eps.parent`
)

// States returns a profile's watch state for items, keyed by id (R36).
// Shows and seasons get episode counts. Items with no state are left out.
func (w *WatchStore) States(ctx context.Context, profileID string, ids []string) (map[string]UserState, error) {
	out := map[string]UserState{}
	if profileID == "" || len(ids) == 0 {
		return out, nil
	}
	in := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, profileID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := w.DB.QueryContext(ctx, statesQuery+in+")", args...) //nolint:gosec // in is only placeholders
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id      string
			st      UserState
			watched int
			last    sql.NullString
		)
		if err := rows.Scan(&id, &st.PositionMs, &st.DurationMs, &watched, &st.PlayCount, &last); err != nil {
			rows.Close()
			return nil, err
		}
		st.Watched, st.LastPlayedAt = watched == 1, parseTime(last)
		out[id] = st
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Shows and seasons: counts over the visible episodes below them.
	rows, err = w.DB.QueryContext(ctx, countsQuery+in+countsQueryEnd, append(slices.Clone(args[1:]), profileID)...) //nolint:gosec // in is only placeholders
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id         string
			total, wat int
			last       sql.NullString
		)
		if err := rows.Scan(&id, &total, &wat, &last); err != nil {
			return nil, err
		}
		out[id] = UserState{
			Aggregate: true, TotalEpisodes: total, WatchedEpisodes: wat,
			Watched: total > 0 && wat == total, LastPlayedAt: parseTime(last),
		}
	}
	return out, rows.Err()
}

// ContinueWatching lists a profile's in-progress movies and episodes, plus
// each show's next episode after the one last watched, most recent first
// (R37). Items the profile may not see are left out. The cursor is an
// offset into that list.
func (w *WatchStore) ContinueWatching(ctx context.Context, profileID string, lim access.Limit, cursor string, limit int) (ids []string, next string, err error) {
	offset := 0
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, "", ErrBadCursor
		}
		if offset, err = strconv.Atoi(string(b)); err != nil || offset < 0 {
			return nil, "", ErrBadCursor
		}
	}
	if limit <= 0 {
		limit = 50
	}
	cond, condArgs := lim.SQL("m")
	visible := ""
	if cond != "" {
		visible = " AND " + cond
	}

	type entry struct {
		id   string
		when string
	}
	var list []entry
	seen := map[string]bool{}
	add := func(id, when string) {
		if !seen[id] {
			seen[id] = true
			list = append(list, entry{id, when})
		}
	}

	rows, err := w.DB.QueryContext(ctx, `
		SELECT w.item_id, w.last_played_at FROM watch_state w
		JOIN items i ON i.id = w.item_id LEFT JOIN item_metadata m ON m.item_id = i.id
		WHERE w.profile_id = ? AND w.watched = 0 AND w.position_ms > 0 AND i.hidden = 0
		  AND i.kind IN ('movie', 'episode')`+visible, append([]any{profileID}, condArgs...)...)
	if err != nil {
		return nil, "", err
	}
	if err := scanEntries(rows, add); err != nil {
		return nil, "", err
	}

	// Next up: per show, the episode after the most recently watched one,
	// in season then episode order with specials (season 0) last.
	order := `(CASE WHEN COALESCE(s.season_number, 0) = 0 THEN 1000000 ELSE s.season_number END) * 100000 + COALESCE(e.episode_number, 0)`
	rows, err = w.DB.QueryContext(ctx, `
		WITH last AS (
			SELECT s.parent_id AS show, `+order+` AS pos, w.last_played_at AS at,
				ROW_NUMBER() OVER (PARTITION BY s.parent_id ORDER BY w.last_played_at DESC) AS rn
			FROM watch_state w JOIN items e ON e.id = w.item_id JOIN items s ON s.id = e.parent_id
			WHERE w.profile_id = ? AND w.watched = 1 AND e.kind = 'episode'
		),
		candidates AS (
			SELECT l.show, e.id, l.at, ROW_NUMBER() OVER (PARTITION BY l.show ORDER BY `+order+`) AS rn
			FROM last l JOIN items s ON s.parent_id = l.show JOIN items e ON e.parent_id = s.id
			LEFT JOIN watch_state w ON w.item_id = e.id AND w.profile_id = ?
			LEFT JOIN item_metadata m ON m.item_id = e.id
			WHERE l.rn = 1 AND e.kind = 'episode' AND e.hidden = 0 AND `+order+` > l.pos
			  AND COALESCE(w.watched, 0) = 0`+visible+`
		)
		SELECT id, at FROM candidates WHERE rn = 1`, append([]any{profileID, profileID}, condArgs...)...)
	if err != nil {
		return nil, "", err
	}
	if err := scanEntries(rows, add); err != nil {
		return nil, "", err
	}

	slices.SortStableFunc(list, func(a, b entry) int { return strings.Compare(b.when, a.when) })
	if offset >= len(list) {
		return []string{}, "", nil
	}
	end := min(offset+limit, len(list))
	for _, e := range list[offset:end] {
		ids = append(ids, e.id)
	}
	if end < len(list) {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	return ids, next, nil
}

// ErrBadCursor is returned for a cursor ContinueWatching didn't make.
var ErrBadCursor = errors.New("invalid cursor")

func scanEntries(rows *sql.Rows, add func(id, when string)) error {
	defer rows.Close()
	for rows.Next() {
		var id string
		var when sql.NullString
		if err := rows.Scan(&id, &when); err != nil {
			return err
		}
		add(id, when.String)
	}
	return rows.Err()
}

func parseTime(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s.String)
	if err != nil {
		return nil
	}
	return &t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullInt(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
