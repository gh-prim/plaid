package catalog

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gh-prim/plaid/server/internal/access"
)

// ErrNotFound is an item that doesn't exist, is hidden, or that the
// profile may not see.
var ErrNotFound = errors.New("not found")

type episode struct {
	id       string
	pos      int64 // episodeOrder
	watched  bool
	position int64
	played   string
}

// UpNext returns the episode to play next for the profile (F008-R32), ""
// when there is none:
//   - on a show, the episode in progress (the latest played), else the
//     first unwatched one after the latest watched, else the first one;
//     specials are skipped;
//   - on a season, the same within that season;
//   - on an episode, the next one by season and episode number, specials
//     skipped (from a special, the next special).
//
// Episodes hidden or above the age limit don't count.
func (c *Catalog) UpNext(ctx context.Context, itemID, profileID string, lim access.Limit) (string, error) {
	var (
		kind, parent string
		age          sql.NullInt64
		hidden       bool
	)
	err := c.db.QueryRowContext(ctx, `SELECT i.kind, COALESCE(i.parent_id, ''), m.age_rating, i.hidden
		FROM items i LEFT JOIN item_metadata m ON m.item_id = i.id WHERE i.id = ?`, itemID).Scan(&kind, &parent, &age, &hidden)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	var rating *int
	if age.Valid {
		n := int(age.Int64)
		rating = &n
	}
	if hidden || !lim.Allows(rating) {
		return "", ErrNotFound
	}

	switch kind {
	case "show":
		eps, err := c.episodes(ctx, `s.parent_id = ? AND COALESCE(s.season_number, 0) <> 0`, itemID, profileID, lim)
		if err != nil {
			return "", err
		}
		return resume(eps), nil
	case "season":
		eps, err := c.episodes(ctx, `s.id = ?`, itemID, profileID, lim)
		if err != nil {
			return "", err
		}
		return resume(eps), nil
	case "episode":
		// The scanner numbers the season, not its episodes.
		var (
			show   string
			season int
		)
		if err := c.db.QueryRowContext(ctx, `SELECT parent_id, COALESCE(season_number, 0) FROM items WHERE id = ?`, parent).Scan(&show, &season); err != nil {
			return "", err
		}
		special := season == 0
		where := `s.parent_id = ? AND COALESCE(s.season_number, 0) <> 0`
		if special {
			where = `s.parent_id = ? AND COALESCE(s.season_number, 0) = 0`
		}
		eps, err := c.episodes(ctx, where, show, profileID, lim)
		if err != nil {
			return "", err
		}
		pos, err := c.position(ctx, itemID)
		if err != nil {
			return "", err
		}
		for _, e := range eps {
			if e.pos > pos {
				return e.id, nil
			}
		}
		return "", nil
	}
	return "", nil
}

// resume picks the episode to resume or start in an ordered list.
func resume(eps []episode) string {
	if len(eps) == 0 {
		return ""
	}
	latest := func(ok func(episode) bool) int {
		best := -1
		for i, e := range eps {
			if ok(e) && e.played != "" && (best < 0 || e.played > eps[best].played) {
				best = i
			}
		}
		return best
	}
	if i := latest(func(e episode) bool { return !e.watched && e.position > 0 }); i >= 0 {
		return eps[i].id
	}
	i := latest(func(e episode) bool { return e.watched })
	if i < 0 {
		for _, e := range eps {
			if !e.watched {
				return e.id
			}
		}
		return ""
	}
	for _, e := range eps[i+1:] {
		if !e.watched {
			return e.id
		}
	}
	return ""
}

// episodeOrder sorts episodes by season then number, specials (season 0)
// last, like continue watching.
const episodeOrder = `(CASE WHEN COALESCE(s.season_number, 0) = 0 THEN 1000000 ELSE s.season_number END) * 100000 + COALESCE(e.episode_number, 0)`

// episodes lists the visible episodes of the seasons matching where (one
// argument, arg), in order, with the profile's watch state.
func (c *Catalog) episodes(ctx context.Context, where, arg, profileID string, lim access.Limit) ([]episode, error) {
	q := `SELECT e.id, ` + episodeOrder + `, COALESCE(w.watched, 0), COALESCE(w.position_ms, 0), COALESCE(w.last_played_at, '')
		FROM items s JOIN items e ON e.parent_id = s.id AND e.kind = 'episode' AND e.hidden = 0
		LEFT JOIN item_metadata m ON m.item_id = e.id
		LEFT JOIN watch_state w ON w.item_id = e.id AND w.profile_id = ?
		WHERE s.kind = 'season' AND ` + where
	args := []any{profileID, arg}
	if cond, a := lim.SQL("m"); cond != "" {
		q += " AND " + cond
		args = append(args, a...)
	}
	var out []episode
	err := c.scan(ctx, q+` ORDER BY `+episodeOrder+`, e.id`, args, func(r *sql.Rows) error { //nolint:gosec // fixed fragments
		var e episode
		if err := r.Scan(&e.id, &e.pos, &e.watched, &e.position, &e.played); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	return out, err
}

func (c *Catalog) position(ctx context.Context, episodeID string) (int64, error) {
	var pos int64
	err := c.db.QueryRowContext(ctx, `SELECT `+episodeOrder+` FROM items e JOIN items s ON s.id = e.parent_id WHERE e.id = ?`, episodeID).Scan(&pos)
	return pos, err
}
