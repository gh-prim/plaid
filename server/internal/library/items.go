package library

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
)

// MaxPageSize bounds list pages (R22).
const MaxPageSize = 200

// ErrBadCursor rejects a cursor this server didn't produce.
var ErrBadCursor = errors.New("invalid cursor")

// itemSortKey orders items within a list: by sort title for movies and
// shows, by number for seasons, by number then air date for episodes.
// Movies and shows use their matched sort title (F005) when they have one,
// joined as m.
const itemSortKey = `CASE items.kind
	WHEN 'season' THEN printf('%06d', COALESCE(items.season_number, 0))
	WHEN 'episode' THEN printf('%06d|%s', COALESCE(items.episode_number, 999999), COALESCE(items.air_date, ''))
	ELSE COALESCE(m.sort_title, items.sort_title, '') END`

const itemColumns = `SELECT items.id, items.kind, items.library_id, COALESCE(items.parent_id, ''), COALESCE(items.title, ''),
	COALESCE(items.year, 0), COALESCE(items.season_number, (SELECT p.season_number FROM items p
		WHERE p.id = items.parent_id AND items.kind = 'episode')), items.episode_number, COALESCE(items.air_date, ''),
	COALESCE(items.hints, ''), m.age_rating, items.created_at, COALESCE(items.newest_child_added_at, '')
	FROM items LEFT JOIN item_metadata m ON m.item_id = items.id WHERE items.id IN (`

// cursor is the keyset position after the last row of a page.
type cursor struct {
	Key string `json:"k"`
	ID  string `json:"i"`
}

func encodeCursor(key, id string) string {
	raw, _ := json.Marshal(cursor{key, id})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string) (cursor, error) {
	var c cursor
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || json.Unmarshal(raw, &c) != nil || c.ID == "" {
		return c, ErrBadCursor
	}
	return c, nil
}

func pageSize(n int) int {
	if n <= 0 {
		return 50
	}
	return min(n, MaxPageSize)
}

// Item is an item as the read API shows it (R22).
type Item struct {
	ID        string
	Kind      string
	LibraryID string
	ParentID  string
	Title     string
	Year      int // 0 when unknown
	// Season is an episode's season number too (the scanner stores it on
	// the season), so a list of episodes can say S1 E4.
	Season  *int
	Episode *int
	AirDate string
	Hints   map[string]string
	// AgeRating is the minimum age from F005's certification, nil when
	// unrated.
	AgeRating *int
	// CreatedAt is when the scanner added the item; NewestChildAddedAt,
	// for a show, when its newest episode was added ("" when none).
	CreatedAt          time.Time
	NewestChildAddedAt time.Time
	// FileCount counts files present on disk, ChildCount visible children.
	FileCount  int
	ChildCount int
	Technical  Technical
}

// Technical sums up the files present under an item.
type Technical struct {
	Width, Height  int // largest resolution, 0 when unknown
	HDRFormats     []string
	AudioLanguages []string
}

// ItemQuery selects a page of a library's items.
type ItemQuery struct {
	LibraryID string
	// Kind filters by kind; with no ParentID it defaults to the library's
	// top level (movie or show).
	Kind     string
	ParentID string
	Cursor   string
	Limit    int
	// Access filters out what the active profile may not see (F005 R25).
	Access access.Limit
}

// ListItems returns a page of visible items and the cursor of the next
// page ("" on the last one).
func (s *Store) ListItems(ctx context.Context, q ItemQuery) ([]Item, string, error) {
	lib, err := s.GetLibrary(ctx, q.LibraryID)
	if err != nil {
		return nil, "", err
	}
	where := []string{"items.library_id = ?", "items.hidden = 0"}
	args := []any{q.LibraryID}
	switch {
	case q.ParentID != "":
		where = append(where, "items.parent_id = ?")
		args = append(args, q.ParentID)
	case q.Kind == "":
		q.Kind = KindMovie
		if lib.Type == TypeShows {
			q.Kind = KindShow
		}
	}
	if q.Kind != "" {
		where = append(where, "items.kind = ?")
		args = append(args, q.Kind)
	}
	if cond, condArgs := q.Access.SQL("m"); cond != "" {
		where = append(where, cond)
		args = append(args, condArgs...)
	}
	// Only constant fragments are concatenated; values go through args.
	const from = ` AS k FROM items LEFT JOIN item_metadata m ON m.item_id = items.id WHERE `
	query := `SELECT id, k FROM (SELECT items.id AS id, ` + itemSortKey + from + strings.Join(where, " AND ") + `)` //nolint:gosec // see above
	if q.Cursor != "" {
		c, err := decodeCursor(q.Cursor)
		if err != nil {
			return nil, "", err
		}
		query += ` WHERE k > ? OR (k = ? AND id > ?)`
		args = append(args, c.Key, c.Key, c.ID)
	}
	limit := pageSize(q.Limit)
	query += ` ORDER BY k, id LIMIT ?`
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	var ids, keys []string
	for rows.Next() {
		var id, k string
		if err := rows.Scan(&id, &k); err != nil {
			_ = rows.Close()
			return nil, "", err
		}
		ids, keys = append(ids, id), append(keys, k)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(ids) > limit {
		ids, keys = ids[:limit], keys[:limit]
		next = encodeCursor(keys[limit-1], ids[limit-1])
	}
	items, err := s.items(ctx, ids)
	return items, next, err
}

// GetItem returns one item, hidden or not.
func (s *Store) GetItem(ctx context.Context, id string) (Item, error) {
	items, err := s.items(ctx, []string{id})
	if err != nil {
		return Item{}, err
	}
	if len(items) == 0 {
		return Item{}, ErrNotFound
	}
	return items[0], nil
}

// Shows maps seasons and episodes, by id, to their show's id; other
// items are left out.
func (s *Store) Shows(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	q := `SELECT i.id, CASE i.kind WHEN 'season' THEN i.parent_id ELSE p.parent_id END
		FROM items i LEFT JOIN items p ON p.id = i.parent_id
		WHERE i.kind IN ('season', 'episode') AND i.id IN (` + placeholders(len(ids)) + `)` //nolint:gosec // only placeholders
	err := s.scanRows(ctx, q, args, func(r *sql.Rows) error {
		var id string
		var show sql.NullString
		if err := r.Scan(&id, &show); err != nil {
			return err
		}
		if show.Valid {
			out[id] = show.String
		}
		return nil
	})
	return out, err
}

// Items loads items by id, hidden or not, in the given order; unknown ids
// are left out.
func (s *Store) Items(ctx context.Context, ids []string) ([]Item, error) {
	return s.items(ctx, ids)
}

// items loads items by id, in the given order, with counts and technical
// summaries.
func (s *Store) items(ctx context.Context, ids []string) ([]Item, error) {
	if len(ids) == 0 {
		return []Item{}, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	in := placeholders(len(ids))
	byID := map[string]*Item{}

	byIDQuery := itemColumns + in + `)` //nolint:gosec // in is only placeholders
	rows, err := s.db.QueryContext(ctx, byIDQuery, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			it                     Item
			season, episode, age   sql.NullInt64
			hints, created, newest string
		)
		if err := rows.Scan(&it.ID, &it.Kind, &it.LibraryID, &it.ParentID, &it.Title, &it.Year, &season, &episode,
			&it.AirDate, &hints, &age, &created, &newest); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if season.Valid {
			n := int(season.Int64)
			it.Season = &n
		}
		if episode.Valid {
			n := int(episode.Int64)
			it.Episode = &n
		}
		if hints != "" {
			_ = json.Unmarshal([]byte(hints), &it.Hints)
		}
		if age.Valid {
			n := int(age.Int64)
			it.AgeRating = &n
		}
		it.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		it.NewestChildAddedAt, _ = time.Parse(time.RFC3339Nano, newest)
		it.Technical = Technical{HDRFormats: []string{}, AudioLanguages: []string{}}
		byID[it.ID] = &it
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Files present under each item: its own, its children's, its
	// grandchildren's (show → season → episode).
	descendants := `WITH RECURSIVE d(root, id) AS (
		SELECT id, id FROM items WHERE id IN (` + in + `)
		UNION ALL SELECT d.root, i.id FROM items i JOIN d ON i.parent_id = d.id)`
	files := descendants + `
		SELECT DISTINCT d.root, f.id, COALESCE(f.width, 0), COALESCE(f.height, 0), COALESCE(f.hdr_format, '')
		FROM d JOIN item_files x ON x.item_id = d.id JOIN media_files f ON f.id = x.file_id
		WHERE f.missing_since IS NULL`
	if err := s.scanRows(ctx, files, args, func(r *sql.Rows) error {
		var (
			root, fileID, hdr string
			w, h              int
		)
		if err := r.Scan(&root, &fileID, &w, &h, &hdr); err != nil {
			return err
		}
		it := byID[root]
		it.FileCount++
		if w*h > it.Technical.Width*it.Technical.Height {
			it.Technical.Width, it.Technical.Height = w, h
		}
		if hdr != "" && !slices.Contains(it.Technical.HDRFormats, hdr) {
			it.Technical.HDRFormats = append(it.Technical.HDRFormats, hdr)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	langs := descendants + `
		SELECT DISTINCT d.root, s.language
		FROM d JOIN item_files x ON x.item_id = d.id JOIN media_files f ON f.id = x.file_id
		JOIN media_streams s ON s.file_id = f.id
		WHERE f.missing_since IS NULL AND s.type = 'audio' AND s.language IS NOT NULL
		ORDER BY d.root, s.language`
	if err := s.scanRows(ctx, langs, args, func(r *sql.Rows) error {
		var root, lang string
		if err := r.Scan(&root, &lang); err != nil {
			return err
		}
		byID[root].Technical.AudioLanguages = append(byID[root].Technical.AudioLanguages, lang)
		return nil
	}); err != nil {
		return nil, err
	}
	children := `SELECT parent_id, COUNT(*) FROM items WHERE hidden = 0 AND parent_id IN (` + in + `) GROUP BY parent_id`
	if err := s.scanRows(ctx, children, args, func(r *sql.Rows) error {
		var (
			id string
			n  int
		)
		if err := r.Scan(&id, &n); err != nil {
			return err
		}
		byID[id].ChildCount = n
		return nil
	}); err != nil {
		return nil, err
	}

	out := make([]Item, 0, len(ids))
	for _, id := range ids {
		if it, ok := byID[id]; ok {
			slices.Sort(it.Technical.HDRFormats)
			out = append(out, *it)
		}
	}
	return out, nil
}

func (s *Store) scanRows(ctx context.Context, query string, args []any, each func(*sql.Rows) error) error {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// MediaFile is a file linked to an item, with its streams.
type MediaFile struct {
	ID           string
	Path         string
	Size         int64
	ModTime      time.Time
	Edition      string
	ParseStatus  string
	ProbeStatus  string
	Container    string
	DurationMs   int64
	Bitrate      int64
	Width        int
	Height       int
	VideoCodec   string
	HDRFormat    string
	DVProfile    int // with DVCompat, set for Dolby Vision files
	DVCompat     int
	MissingSince *time.Time
	Streams      []MediaStream

	probeJSON string // kept for PlayableFiles
}

// MediaStream is a stored stream; External is set for subtitle files.
type MediaStream struct {
	Index    int
	Type     string
	Codec    string
	Language string
	Title    string
	Channels int
	Default  bool
	Forced   bool
	SDH      bool
	Image    bool
	External bool
	// ExternalPath is the subtitle file's path; never sent to clients.
	ExternalPath string
}

// ItemFiles returns the files linked to an item, present ones first.
func (s *Store) ItemFiles(ctx context.Context, itemID string) ([]MediaFile, error) {
	var files []MediaFile
	err := s.scanRows(ctx, `
		SELECT f.id, f.path, f.size, f.mtime, COALESCE(f.edition, ''), f.parse_status, f.probe_status,
		       COALESCE(f.container, ''), COALESCE(f.duration_ms, 0), COALESCE(f.bitrate, 0),
		       COALESCE(f.width, 0), COALESCE(f.height, 0), COALESCE(f.video_codec, ''),
		       COALESCE(f.hdr_format, ''), COALESCE(f.probe_json, ''), COALESCE(f.missing_since, '')
		FROM item_files x JOIN media_files f ON f.id = x.file_id
		WHERE x.item_id = ? ORDER BY f.missing_since IS NOT NULL, f.path`, []any{itemID},
		func(r *sql.Rows) error {
			var (
				f              MediaFile
				mtime, missing string
				probeJSON      string
			)
			if err := r.Scan(&f.ID, &f.Path, &f.Size, &mtime, &f.Edition, &f.ParseStatus, &f.ProbeStatus,
				&f.Container, &f.DurationMs, &f.Bitrate, &f.Width, &f.Height, &f.VideoCodec, &f.HDRFormat,
				&probeJSON, &missing); err != nil {
				return err
			}
			f.ModTime, _ = time.Parse(time.RFC3339Nano, mtime)
			if missing != "" {
				if t, err := time.Parse(time.RFC3339, missing); err == nil {
					f.MissingSince = &t
				}
			}
			f.probeJSON = probeJSON
			if f.HDRFormat == DolbyVision && probeJSON != "" {
				if res, err := ParseProbe([]byte(probeJSON)); err == nil {
					if v := res.Video(); v != nil {
						f.DVProfile, f.DVCompat = v.DVProfile, v.DVCompat
					}
				}
			}
			files = append(files, f)
			return nil
		})
	if err != nil {
		return nil, err
	}
	for i := range files {
		f := &files[i]
		f.Streams = []MediaStream{}
		if err := s.scanRows(ctx, `
			SELECT idx, type, COALESCE(codec, ''), COALESCE(language, 'und'), COALESCE(title, ''),
			       COALESCE(channels, 0), is_default, is_forced, is_sdh, is_image, COALESCE(external_path, '')
			FROM media_streams WHERE file_id = ? ORDER BY idx`, []any{f.ID},
			func(r *sql.Rows) error {
				var st MediaStream
				if err := r.Scan(&st.Index, &st.Type, &st.Codec, &st.Language, &st.Title, &st.Channels,
					&st.Default, &st.Forced, &st.SDH, &st.Image, &st.ExternalPath); err != nil {
					return err
				}
				st.External = st.ExternalPath != ""
				f.Streams = append(f.Streams, st)
				return nil
			}); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// PlayableFile is a file with what ffprobe said about it, for playback
// (F006 R3). Probe is nil when the file isn't analyzed or its analysis
// can't be read.
type PlayableFile struct {
	MediaFile
	Probe *ProbeResult
}

// PlayableFiles returns the files linked to an item with their analysis,
// present ones first. Missing and unanalyzed files are included: F006
// tells them apart for its errors.
func (s *Store) PlayableFiles(ctx context.Context, itemID string) ([]PlayableFile, error) {
	files, err := s.ItemFiles(ctx, itemID)
	if err != nil {
		return nil, err
	}
	out := make([]PlayableFile, len(files))
	for i, f := range files {
		out[i].MediaFile = f
		if f.ProbeStatus == "ok" && f.probeJSON != "" {
			out[i].Probe, _ = ParseProbe([]byte(f.probeJSON))
		}
	}
	return out, nil
}

// UnrecognizedFile is a file present on disk whose name couldn't be parsed
// (R11).
type UnrecognizedFile struct {
	ID          string
	Path        string
	Size        int64
	ProbeStatus string
}

// ListUnrecognized returns a page of a library's unrecognized files, by path.
func (s *Store) ListUnrecognized(ctx context.Context, libraryID, cur string, limit int) ([]UnrecognizedFile, string, error) {
	if _, err := s.GetLibrary(ctx, libraryID); err != nil {
		return nil, "", err
	}
	query := `SELECT id, path, size, probe_status FROM media_files
		WHERE library_id = ? AND parse_status = 'unrecognized' AND missing_since IS NULL`
	args := []any{libraryID}
	if cur != "" {
		c, err := decodeCursor(cur)
		if err != nil {
			return nil, "", err
		}
		query += ` AND (path > ? OR (path = ? AND id > ?))`
		args = append(args, c.Key, c.Key, c.ID)
	}
	n := pageSize(limit)
	query += ` ORDER BY path, id LIMIT ?`
	args = append(args, n+1)
	out := []UnrecognizedFile{}
	err := s.scanRows(ctx, query, args, func(r *sql.Rows) error {
		var f UnrecognizedFile
		if err := r.Scan(&f.ID, &f.Path, &f.Size, &f.ProbeStatus); err != nil {
			return err
		}
		out = append(out, f)
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > n {
		out = out[:n]
		next = encodeCursor(out[n-1].Path, out[n-1].ID)
	}
	return out, next, nil
}
