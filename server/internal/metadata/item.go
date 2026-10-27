package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gh-prim/plaid/server/internal/metadata/nfo"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
)

// parsed is an item as F004 parsed it, with its library's locale.
type parsed struct {
	ID        string
	LibraryID string
	Kind      string // movie, show, season, episode
	ParentID  string
	Title     string
	Year      int
	Season    int // -1 when none
	Episode   int // -1 when none
	AirDate   string
	Hints     map[string]string
	CreatedAt string
	Locale    provider.Locale
}

// row is an item_metadata row: what F005 stores about an item.
type row struct {
	State          string
	MatchSource    string
	Confidence     float64
	MatchLocked    bool
	TMDBID         int
	IMDbID         string
	TVDBID         int
	Title          string
	OriginalTitle  string
	SortTitle      string
	Overview       string
	Tagline        string
	ReleaseDate    string
	EndDate        string
	ShowStatus     string
	Runtime        int
	Rating         float64
	RatingCount    int
	Certification  string
	CertRegion     string
	AgeRating      *int
	Genres         []provider.Genre
	Studios        []string
	Trailers       []provider.Video
	CollectionID   string
	DuplicateOf    string
	FieldLanguages map[string]string
	LockedFields   []string
	LocalFields    []string
}

func (s *Store) loadParsed(ctx context.Context, itemID string) (parsed, error) {
	var (
		p               parsed
		parent, title   sql.NullString
		airDate, hints  sql.NullString
		year            sql.NullInt64
		season, episode sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx, `SELECT i.id, i.library_id, i.kind, i.parent_id, i.title, i.year, i.season_number,
		i.episode_number, i.air_date, i.hints, i.created_at, l.metadata_language, l.metadata_region
		FROM items i JOIN libraries l ON l.id = i.library_id WHERE i.id = ?`, itemID).
		Scan(&p.ID, &p.LibraryID, &p.Kind, &parent, &title, &year, &season, &episode, &airDate, &hints, &p.CreatedAt,
			&p.Locale.Language, &p.Locale.Region)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.ParentID, p.Title, p.AirDate = parent.String, title.String, airDate.String
	p.Year = int(year.Int64)
	p.Season, p.Episode = -1, -1
	if season.Valid {
		p.Season = int(season.Int64)
	}
	if episode.Valid {
		p.Episode = int(episode.Int64)
	}
	if hints.String != "" {
		_ = json.Unmarshal([]byte(hints.String), &p.Hints)
	}
	return p, nil
}

const rowColumns = `state, COALESCE(match_source, ''), COALESCE(confidence, 0), match_locked, COALESCE(tmdb_id, 0),
	COALESCE(imdb_id, ''), COALESCE(tvdb_id, 0), COALESCE(title, ''), COALESCE(original_title, ''), COALESCE(sort_title, ''),
	COALESCE(overview, ''), COALESCE(tagline, ''), COALESCE(release_date, ''), COALESCE(end_date, ''), COALESCE(show_status, ''),
	COALESCE(runtime_minutes, 0), COALESCE(rating, 0), COALESCE(rating_count, 0), COALESCE(certification, ''),
	COALESCE(certification_region, ''), age_rating, COALESCE(genres, ''), COALESCE(studios, ''), COALESCE(trailers, ''),
	COALESCE(collection_id, ''), COALESCE(duplicate_of, ''), COALESCE(field_languages, ''), COALESCE(locked_fields, ''),
	COALESCE(local_fields, '')`

type scanner interface{ Scan(...any) error }

func scanRow(sc scanner) (row, error) {
	var (
		r                                    row
		age                                  sql.NullInt64
		genres, studios, trailers, langs, lk string
		local                                string
	)
	err := sc.Scan(&r.State, &r.MatchSource, &r.Confidence, &r.MatchLocked, &r.TMDBID, &r.IMDbID, &r.TVDBID, &r.Title,
		&r.OriginalTitle, &r.SortTitle, &r.Overview, &r.Tagline, &r.ReleaseDate, &r.EndDate, &r.ShowStatus, &r.Runtime,
		&r.Rating, &r.RatingCount, &r.Certification, &r.CertRegion, &age, &genres, &studios, &trailers, &r.CollectionID,
		&r.DuplicateOf, &langs, &lk, &local)
	if err != nil {
		return r, err
	}
	if age.Valid {
		n := int(age.Int64)
		r.AgeRating = &n
	}
	for _, j := range []struct {
		raw string
		dst any
	}{{genres, &r.Genres}, {studios, &r.Studios}, {trailers, &r.Trailers}, {langs, &r.FieldLanguages}, {lk, &r.LockedFields}, {local, &r.LocalFields}} {
		if j.raw != "" {
			_ = json.Unmarshal([]byte(j.raw), j.dst)
		}
	}
	return r, nil
}

func (s *Store) loadRow(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, itemID string) (row, error) {
	r, err := scanRow(q.QueryRowContext(ctx, `SELECT `+rowColumns+` FROM item_metadata WHERE item_id = ?`, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return row{State: "pending"}, nil
	}
	return r, err
}

// itemDirs returns the folders holding an item's files present on disk,
// with the base names of those files (for <video>.nfo).
func (s *Store) itemDirs(ctx context.Context, itemID string) (dirs, bases []string, err error) {
	rows, err := s.db.QueryContext(ctx, `SELECT f.path FROM item_files x JOIN media_files f ON f.id = x.file_id
		WHERE x.item_id = ? AND f.missing_since IS NULL ORDER BY f.path`, itemID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, nil, err
		}
		if d := filepath.Dir(p); !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
		bases = append(bases, strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)))
	}
	return dirs, bases, rows.Err()
}

// showDir returns a show's folder: the top folder under the library folder
// that holds its episodes (F004 R9).
func (s *Store) showDir(ctx context.Context, showID string) (string, error) {
	var folder, path string
	err := s.db.QueryRowContext(ctx, `WITH RECURSIVE d(id) AS (SELECT ? UNION ALL SELECT i.id FROM items i JOIN d ON i.parent_id = d.id)
		SELECT lf.path, f.path FROM d JOIN item_files x ON x.item_id = d.id JOIN media_files f ON f.id = x.file_id
		JOIN library_folders lf ON lf.id = f.folder_id WHERE f.missing_since IS NULL LIMIT 1`, showID).Scan(&folder, &path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(folder, path)
	if err != nil {
		return "", err
	}
	top, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	return filepath.Join(folder, top), nil
}

// readNFO reads the first .nfo of names found in dirs. A missing file is
// not an error; a broken one is logged by the caller and skipped (R12).
func readNFO(dirs, names []string) (*nfo.NFO, string, error) {
	for _, d := range dirs {
		for _, n := range names {
			p := filepath.Join(d, n)
			f, err := os.Open(p) //nolint:gosec // inside a library folder, read-only
			if err != nil {
				continue
			}
			doc, err := nfo.Parse(f)
			_ = f.Close()
			return doc, p, err
		}
	}
	return nil, "", nil
}
