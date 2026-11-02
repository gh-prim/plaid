package library

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// ReportItem is a movie or a show in the library report (admin): what it
// weighs on disk, its best picture, its files and whether anyone watched.
type ReportItem struct {
	ID, LibraryID, Kind string
	Title               string
	Bytes               int64
	Files               int
	// Height is the best picture's lines (2160, 1080…), 0 when unknown.
	Height int
	// Duplicate is true when a movie, or one of a show's episodes, has
	// more than one file.
	Duplicate bool
	// Plays counts profiles' plays (watch state); LastPlayed is the latest.
	Plays      int
	LastPlayed *time.Time
	AddedAt    time.Time
	GenreIDs   []int
}

// Genre is a TMDB genre and its name in the library's language.
type Genre struct {
	ID   int
	Name string
}

// Report lists every movie and show not hidden, in one library or all
// (libraryID empty), with the genres they use.
func (s *Store) Report(ctx context.Context, libraryID string) ([]ReportItem, []Genre, error) {
	// Each file counts toward its movie, or its episode's show.
	rows, err := s.db.QueryContext(ctx, `
		WITH owner AS (
			SELECT i.id AS item_id, i.id AS top_id FROM items i WHERE i.kind = 'movie' AND i.hidden = 0
			UNION ALL
			SELECT e.id, se.parent_id FROM items e JOIN items se ON se.id = e.parent_id
			WHERE e.kind = 'episode' AND e.hidden = 0
		),
		per_item AS (
			SELECT o.top_id, o.item_id, COUNT(f.id) AS n, COALESCE(SUM(f.size), 0) AS bytes, COALESCE(MAX(f.height), 0) AS height
			FROM owner o JOIN item_files x ON x.item_id = o.item_id JOIN media_files f ON f.id = x.file_id
			WHERE f.missing_since IS NULL GROUP BY o.top_id, o.item_id
		),
		files AS (
			SELECT top_id, SUM(n) AS n, SUM(bytes) AS bytes, MAX(height) AS height, MAX(n > 1) AS dup
			FROM per_item GROUP BY top_id
		),
		plays AS (
			SELECT o.top_id, SUM(CASE WHEN w.play_count > 0 THEN w.play_count WHEN w.watched = 1 OR w.position_ms > 0 THEN 1 ELSE 0 END) AS n,
				MAX(w.last_played_at) AS last
			FROM owner o JOIN watch_state w ON w.item_id = o.item_id GROUP BY o.top_id
		)
		SELECT t.id, t.library_id, t.kind, COALESCE(NULLIF(m.title, ''), t.title, ''), COALESCE(f.bytes, 0), COALESCE(f.n, 0),
			COALESCE(f.height, 0), COALESCE(f.dup, 0), COALESCE(p.n, 0), p.last, t.created_at,
			(SELECT GROUP_CONCAT(g.genre_id) FROM item_genres g WHERE g.item_id = t.id)
		FROM items t LEFT JOIN item_metadata m ON m.item_id = t.id
		LEFT JOIN files f ON f.top_id = t.id LEFT JOIN plays p ON p.top_id = t.id
		WHERE t.kind IN ('movie', 'show') AND t.hidden = 0 AND (? = '' OR t.library_id = ?)`, libraryID, libraryID)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ReportItem
	for rows.Next() {
		var (
			it     ReportItem
			last   sql.NullString
			added  string
			genres sql.NullString
			dup    int
		)
		if err := rows.Scan(&it.ID, &it.LibraryID, &it.Kind, &it.Title, &it.Bytes, &it.Files, &it.Height, &dup, &it.Plays,
			&last, &added, &genres); err != nil {
			return nil, nil, err
		}
		it.Duplicate = dup > 0
		it.AddedAt, _ = time.Parse(time.RFC3339Nano, added)
		if last.Valid {
			if at, err := time.Parse(time.RFC3339Nano, last.String); err == nil {
				it.LastPlayed = &at
			}
		}
		for _, g := range strings.Split(genres.String, ",") {
			if id, err := strconv.Atoi(g); err == nil {
				it.GenreIDs = append(it.GenreIDs, id)
			}
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	g, err := s.db.QueryContext(ctx, `SELECT g.genre_id, MIN(g.name) FROM item_genres g JOIN items i ON i.id = g.item_id
		WHERE i.hidden = 0 AND (? = '' OR i.library_id = ?) GROUP BY g.genre_id ORDER BY MIN(g.name)`, libraryID, libraryID)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = g.Close() }()
	var list []Genre
	for g.Next() {
		var x Genre
		if err := g.Scan(&x.ID, &x.Name); err != nil {
			return nil, nil, err
		}
		list = append(list, x)
	}
	return out, list, g.Err()
}
