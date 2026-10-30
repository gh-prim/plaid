package catalog

import (
	"context"
	"math/rand/v2"
	"strings"

	"github.com/gh-prim/plaid/server/internal/access"
)

// Sorts of a library grid (F008-R26, R12).
const (
	SortTitle    = "title"
	SortAdded    = "added"
	SortReleased = "released"
	SortRating   = "rating"
	SortPlayed   = "played"
	SortRandom   = "random"
)

// Watch filters.
const (
	WatchUnwatched  = "unwatched"
	WatchInProgress = "in_progress"
	WatchWatched    = "watched"
)

// Resolution buckets, by the largest file under an item.
const (
	Res2160 = "2160p"
	Res1080 = "1080p"
	Res720  = "720p"
	ResSD   = "sd"
)

// resolutionBucket maps a file's width and height (w, h: SQL expressions)
// to a bucket. Width decides first, so scope films (1920×800) are 1080p.
func resolutionBucket(w, h string) string {
	return `CASE WHEN ` + w + ` >= 3000 OR ` + h + ` >= 2000 THEN '2160p'
		WHEN ` + w + ` >= 1800 OR ` + h + ` >= 1000 THEN '1080p'
		WHEN ` + w + ` >= 1200 OR ` + h + ` >= 700 THEN '720p'
		WHEN ` + w + ` > 0 THEN 'sd' END`
}

// Query selects a page of a library's top-level items.
type Query struct {
	LibraryID string
	// Kind is the library's top level: movie or show.
	Kind string
	// ProfileID is the active profile, for watch filters and played order.
	ProfileID string
	Access    access.Limit

	Sort string // "" is title
	Desc bool
	Seed int64 // random order

	Watch       string
	Genres      []int // TMDB ids, any of
	YearFrom    int   // 0: none
	YearTo      int
	Resolutions []string // any of
	HDR         *bool

	Offset, Limit int
	WithTotal     bool
}

// Page is a page of item ids, in order.
type Page struct {
	IDs []string
	// Total counts every item matching the query, -1 when not asked.
	Total int
}

// List returns a page of the visible top-level items of a library that
// match q, after the age predicate (F008-R26).
func (c *Catalog) List(ctx context.Context, q Query) (Page, error) {
	from, args := c.filtered(q)
	page := Page{Total: -1}
	if q.WithTotal {
		if err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+from+`)`, args...).Scan(&page.Total); err != nil { //nolint:gosec // from is built from constants
			return page, err
		}
	}
	if q.Sort == SortRandom {
		ids, err := c.ids(ctx, `SELECT id FROM (`+from+`) ORDER BY id`, args) //nolint:gosec // as above
		if err != nil {
			return page, err
		}
		r := rand.New(rand.NewPCG(uint64(q.Seed), uint64(q.Seed)^0x9e3779b97f4a7c15)) //nolint:gosec // an order, not a secret
		r.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		lo := min(q.Offset, len(ids))
		page.IDs = ids[lo:min(lo+q.Limit, len(ids))]
		return page, nil
	}
	query := `SELECT id FROM (` + from + `) ORDER BY ` + orderBy(q.Sort, q.Desc) + ` LIMIT ? OFFSET ?` //nolint:gosec // as above
	ids, err := c.ids(ctx, query, append(args, q.Limit, q.Offset))
	page.IDs = ids
	return page, err
}

// orderBy sorts by the chosen key, missing values last whatever the order,
// then by title and id so pages never overlap.
func orderBy(sort string, desc bool) string {
	dir := " ASC"
	if desc {
		dir = " DESC"
	}
	switch sort {
	case SortAdded:
		return "added" + dir + ", sort_key, id"
	case SortReleased:
		return "released IS NULL, released" + dir + ", sort_key, id"
	case SortRating:
		return "rating IS NULL, rating" + dir + ", rating_count DESC, sort_key, id"
	case SortPlayed:
		return "played IS NULL, played" + dir + ", sort_key, id"
	default:
		return "sort_key" + dir + ", id" + dir
	}
}

// filtered returns the SELECT of the matching items with their sort
// columns, and its arguments. Only constant fragments are concatenated.
func (c *Catalog) filtered(q Query) (string, []any) {
	var (
		with  []string
		wargs []any
		where = []string{"i.library_id = ?", "i.hidden = 0", "i.kind = ?"}
		args  = []any{q.LibraryID, q.Kind}
	)
	if cond, a := q.Access.SQL("m"); cond != "" {
		where = append(where, cond)
		args = append(args, a...)
	}

	// Watch state: a movie's own row; a show's counts over its visible episodes.
	watched, progress, played := "0", "0", "NULL"
	if q.ProfileID != "" {
		if q.Kind == "show" {
			with = append(with, `sw AS (
				SELECT s.parent_id AS show_id, COUNT(*) AS total, COALESCE(SUM(w.watched), 0) AS wat,
					COALESCE(SUM(w.watched = 0 AND w.position_ms > 0), 0) AS prog, MAX(w.last_played_at) AS last
				FROM items s JOIN items e ON e.parent_id = s.id AND e.kind = 'episode' AND e.hidden = 0
				LEFT JOIN watch_state w ON w.item_id = e.id AND w.profile_id = ?
				WHERE s.library_id = ? AND s.kind = 'season' GROUP BY s.parent_id)`)
			wargs = append(wargs, q.ProfileID, q.LibraryID)
			watched = "(sw.total > 0 AND sw.wat = sw.total)"
			progress = "(NOT " + watched + " AND (sw.wat > 0 OR sw.prog > 0))"
			played = "sw.last"
		} else {
			watched = "COALESCE(w.watched, 0) = 1"
			progress = "(COALESCE(w.watched, 0) = 0 AND COALESCE(w.position_ms, 0) > 0)"
			played = "w.last_played_at"
		}
	}
	switch q.Watch {
	case WatchWatched:
		where = append(where, watched)
	case WatchInProgress:
		where = append(where, progress)
	case WatchUnwatched:
		where = append(where, "NOT "+watched, "NOT "+progress)
	}

	year := "COALESCE(CAST(substr(m.release_date, 1, 4) AS INTEGER), i.year)"
	if q.YearFrom > 0 {
		where = append(where, year+" >= ?")
		args = append(args, q.YearFrom)
	}
	if q.YearTo > 0 {
		where = append(where, year+" <= ?")
		args = append(args, q.YearTo)
	}
	if len(q.Genres) > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM item_genres g WHERE g.item_id = i.id AND g.genre_id IN ("+placeholders(len(q.Genres))+"))")
		for _, g := range q.Genres {
			args = append(args, g)
		}
	}

	// Files present under each item: a movie's own, a show's episodes'.
	joinTech := ""
	if len(q.Resolutions) > 0 || q.HDR != nil {
		with = append(with, `t AS (
			SELECT root, MAX(f.width) AS w, MAX(f.height) AS h, MAX(COALESCE(f.hdr_format, '') <> '') AS hdr FROM (
				SELECT x.item_id AS root, x.file_id FROM item_files x JOIN items r ON r.id = x.item_id
				WHERE r.library_id = ? AND r.kind = ?
				UNION ALL
				SELECT s.parent_id, x.file_id FROM items s JOIN items e ON e.parent_id = s.id JOIN item_files x ON x.item_id = e.id
				WHERE s.library_id = ? AND s.kind = 'season'
			) fo JOIN media_files f ON f.id = fo.file_id AND f.missing_since IS NULL GROUP BY root)`)
		wargs = append(wargs, q.LibraryID, q.Kind, q.LibraryID)
		joinTech = " LEFT JOIN t ON t.root = i.id"
		if len(q.Resolutions) > 0 {
			where = append(where, "("+resolutionBucket("COALESCE(t.w, 0)", "COALESCE(t.h, 0)")+") IN ("+placeholders(len(q.Resolutions))+")")
			for _, r := range q.Resolutions {
				args = append(args, r)
			}
		}
		if q.HDR != nil {
			where = append(where, "COALESCE(t.hdr, 0) = ?")
			args = append(args, *q.HDR)
		}
	}

	join := " LEFT JOIN item_metadata m ON m.item_id = i.id"
	if q.ProfileID != "" {
		if q.Kind == "show" {
			join += " LEFT JOIN sw ON sw.show_id = i.id"
		} else {
			join += " LEFT JOIN watch_state w ON w.item_id = i.id AND w.profile_id = ?"
			args = append([]any{q.ProfileID}, args...)
		}
	}
	sel := `SELECT i.id AS id, COALESCE(m.sort_title, i.sort_title, '') AS sort_key,
		COALESCE(i.newest_child_added_at, i.created_at) AS added,
		COALESCE(m.release_date, CASE WHEN i.year > 0 THEN printf('%04d', i.year) END) AS released,
		m.rating AS rating, COALESCE(m.rating_count, 0) AS rating_count, ` + played + ` AS played
		FROM items i` + join + joinTech + ` WHERE ` + strings.Join(where, " AND ")
	if len(with) == 0 {
		return sel, args
	}
	return `WITH ` + strings.Join(with, ", ") + ` ` + sel, append(wargs, args...)
}

func (c *Catalog) ids(ctx context.Context, query string, args []any) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
