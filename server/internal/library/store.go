// Package library turns folders on disk into a structured media library:
// walking, name parsing, ffprobe analysis and reconciliation with SQLite (F004).
package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Library types.
const (
	TypeMovies = "movies"
	TypeShows  = "shows"
)

// Folder statuses.
const (
	FolderOK          = "ok"
	FolderUnavailable = "unavailable"
	FolderUnreadable  = "unreadable"
)

// Item kinds.
const (
	KindMovie   = "movie"
	KindShow    = "show"
	KindSeason  = "season"
	KindEpisode = "episode"
)

// File parse and probe statuses.
const (
	ParseOK           = "ok"
	ParseUnrecognized = "unrecognized"

	ProbePending = "pending"
	ProbeOK      = "ok"
	ProbeFailed  = "failed"
	ProbeSkipped = "skipped"
)

// batchLimit is the maximum number of rows written per transaction (R24).
const batchLimit = 500

// batchHold is how long a batch keeps its transaction open across units.
// The write lock is held from the first write (_txlock=immediate) while
// the scanner reads the next files, which can take seconds on a network
// share: past this, the batch commits so other writers (metadata, artwork,
// sessions) don't wait out busy_timeout and fail with SQLITE_BUSY.
const batchHold = 250 * time.Millisecond

// ErrNotFound is returned when a row doesn't exist.
var ErrNotFound = errors.New("not found")

// Library is a row of the libraries table with its folders.
type Library struct {
	ID            string
	Name          string
	Type          string
	ScanIntervalS *int64
	LastScanAt    *time.Time
	// MetadataLanguage (ISO 639-1) and MetadataRegion (ISO 3166-1 alpha-2)
	// choose the language F005 fetches metadata in.
	MetadataLanguage string
	MetadataRegion   string
	Folders          []Folder
}

// Folder is a row of library_folders.
type Folder struct {
	ID      string
	Path    string
	FSType  string
	Watched bool
	Status  string
}

// Store reads and writes the library tables.
type Store struct {
	db *sql.DB
	// Now is the clock used for every timestamp; tests replace it.
	Now func() time.Time
	// batchSize and batchHold override batchLimit and the const batchHold
	// in tests.
	batchSize int
	batchHold time.Duration
	// afterCommit, when set, runs after each batch commit; tests use it to
	// simulate a crash between batches.
	afterCommit func() error
}

// NewStore wraps an open database (see internal/db).
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, Now: time.Now}
}

func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// ts formats a timestamp as RFC 3339 UTC with second precision, which keeps
// stored values lexically comparable.
func ts(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// mtimeString keeps nanoseconds: mtimes are only compared for equality.
func mtimeString(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func (s *Store) now() string { return ts(s.Now()) }

// CreateOptions are the optional settings of a new library.
type CreateOptions struct {
	ScanIntervalS *int64 // nil = defaults (R19)
	// Empty means the database defaults, en/US (F005 R10).
	MetadataLanguage string
	MetadataRegion   string
}

// CreateLibrary inserts a library and its folders. Paths must already be
// validated and cleaned by the caller. opts is optional.
func (s *Store) CreateLibrary(ctx context.Context, name, typ string, folders []string, opts ...CreateOptions) (Library, error) {
	lib := Library{ID: newID(), Name: name, Type: typ, MetadataLanguage: "en", MetadataRegion: "US"}
	if len(opts) > 0 {
		lib.ScanIntervalS = opts[0].ScanIntervalS
		if opts[0].MetadataLanguage != "" {
			lib.MetadataLanguage = opts[0].MetadataLanguage
		}
		if opts[0].MetadataRegion != "" {
			lib.MetadataRegion = opts[0].MetadataRegion
		}
	}
	now := s.now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return lib, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO libraries (id, name, type, scan_interval_s, metadata_language, metadata_region, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		lib.ID, name, typ, nullInt64Ptr(lib.ScanIntervalS), lib.MetadataLanguage, lib.MetadataRegion, now, now); err != nil {
		return lib, fmt.Errorf("insert library: %w", err)
	}
	for _, p := range folders {
		f := Folder{ID: newID(), Path: p, Status: FolderOK}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO library_folders (id, library_id, path, created_at) VALUES (?, ?, ?, ?)`,
			f.ID, lib.ID, p, now); err != nil {
			return lib, fmt.Errorf("insert folder %s: %w", p, err)
		}
		lib.Folders = append(lib.Folders, f)
	}
	return lib, tx.Commit()
}

// GetLibrary loads a library and its folders.
func (s *Store) GetLibrary(ctx context.Context, id string) (Library, error) {
	var (
		lib      Library
		interval sql.NullInt64
		lastScan sql.NullString
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, type, scan_interval_s, last_scan_at, metadata_language, metadata_region FROM libraries WHERE id = ?`, id).
		Scan(&lib.ID, &lib.Name, &lib.Type, &interval, &lastScan, &lib.MetadataLanguage, &lib.MetadataRegion)
	if errors.Is(err, sql.ErrNoRows) {
		return lib, ErrNotFound
	}
	if err != nil {
		return lib, err
	}
	if interval.Valid {
		lib.ScanIntervalS = &interval.Int64
	}
	if lastScan.Valid {
		if t, err := time.Parse(time.RFC3339, lastScan.String); err == nil {
			lib.LastScanAt = &t
		}
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, path, COALESCE(fs_type, ''), watched, status FROM library_folders
		 WHERE library_id = ? ORDER BY path`, id)
	if err != nil {
		return lib, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Path, &f.FSType, &f.Watched, &f.Status); err != nil {
			return lib, err
		}
		lib.Folders = append(lib.Folders, f)
	}
	return lib, rows.Err()
}

// DeleteLibrary removes a library; folders, files, items, links and streams
// go with it through ON DELETE CASCADE. Nothing on disk is touched (R23).
func (s *Store) DeleteLibrary(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM libraries WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) setFolderStatus(ctx context.Context, folderID, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE library_folders SET status = ? WHERE id = ?`, status, folderID)
	return err
}

func (s *Store) setLastScan(ctx context.Context, libraryID string) error {
	now := s.now()
	_, err := s.db.ExecContext(ctx,
		`UPDATE libraries SET last_scan_at = ?, updated_at = ? WHERE id = ?`, now, now, libraryID)
	return err
}

// fileRow is the part of media_files the reconciler needs.
type fileRow struct {
	ID           string
	FolderID     string
	Path         string
	Size         int64
	MTime        string
	Fingerprint  []byte
	ParseStatus  string
	ProbeStatus  string
	MissingSince string // "" when present
}

func (s *Store) libraryFiles(ctx context.Context, libraryID string) ([]fileRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, folder_id, path, size, mtime, fingerprint, parse_status, probe_status,
		        COALESCE(missing_since, '')
		 FROM media_files WHERE library_id = ?`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []fileRow
	for rows.Next() {
		var f fileRow
		if err := rows.Scan(&f.ID, &f.FolderID, &f.Path, &f.Size, &f.MTime, &f.Fingerprint,
			&f.ParseStatus, &f.ProbeStatus, &f.MissingSince); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// externalStreams returns the external subtitle paths stored per file id.
func (s *Store) externalStreams(ctx context.Context, libraryID string) (map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.file_id, s.external_path FROM media_streams s
		 JOIN media_files f ON f.id = s.file_id
		 WHERE f.library_id = ? AND s.external_path IS NOT NULL ORDER BY s.file_id, s.idx`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string][]string{}
	for rows.Next() {
		var id, p string
		if err := rows.Scan(&id, &p); err != nil {
			return nil, err
		}
		out[id] = append(out[id], p)
	}
	return out, rows.Err()
}

// itemRow is the part of items the reconciler needs to find existing items.
type itemRow struct {
	ID       string
	Kind     string
	ParentID string
	Title    string
	Year     int
	Season   int
	Episode  int
	AirDate  string
}

func (s *Store) libraryItems(ctx context.Context, libraryID string) ([]itemRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kind, COALESCE(parent_id, ''), COALESCE(title, ''), COALESCE(year, 0),
		        COALESCE(season_number, -1), COALESCE(episode_number, -1), COALESCE(air_date, '')
		 FROM items WHERE library_id = ?`, libraryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []itemRow
	for rows.Next() {
		var it itemRow
		if err := rows.Scan(&it.ID, &it.Kind, &it.ParentID, &it.Title, &it.Year,
			&it.Season, &it.Episode, &it.AirDate); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Batch groups writes into transactions of at most batchLimit rows. Callers
// write one logical unit (a file and its items, links and streams) and then
// call EndUnit, so a commit never splits a unit: a crash leaves either the
// whole unit or none of it.
type Batch struct {
	ctx   context.Context
	s     *Store
	tx    *sql.Tx
	rows  int
	limit int
	hold  time.Duration
	since time.Time
}

// NewBatch starts a batch; nothing is opened until the first write.
func (s *Store) NewBatch(ctx context.Context) *Batch {
	limit := batchLimit
	if s.batchSize > 0 {
		limit = s.batchSize
	}
	hold := batchHold
	if s.batchHold > 0 {
		hold = s.batchHold
	}
	return &Batch{ctx: ctx, s: s, limit: limit, hold: hold}
}

// Exec runs one write inside the current transaction.
func (b *Batch) Exec(query string, args ...any) error {
	_, err := b.execCount(query, args...)
	return err
}

// execCount runs one write and returns the number of rows it affected.
func (b *Batch) execCount(query string, args ...any) (int64, error) {
	if b.tx == nil {
		tx, err := b.s.db.BeginTx(b.ctx, nil)
		if err != nil {
			return 0, err
		}
		b.tx, b.since = tx, time.Now()
	}
	res, err := b.tx.ExecContext(b.ctx, query, args...)
	if err != nil {
		return 0, err
	}
	b.rows++
	return res.RowsAffected()
}

// EndUnit commits when the current transaction reached the row limit, or
// has held the write lock for batchHold.
func (b *Batch) EndUnit() error {
	if b.rows < b.limit && (b.tx == nil || time.Since(b.since) < b.hold) {
		return nil
	}
	return b.Commit()
}

// Commit commits whatever is pending.
func (b *Batch) Commit() error {
	if b.tx == nil {
		return nil
	}
	err := b.tx.Commit()
	if held := time.Since(b.since); held > time.Second {
		// Other writers wait this long: worth knowing (and fixing).
		slog.Warn("scan held the database write lock", "held_ms", held.Milliseconds(), "rows", b.rows)
	}
	b.tx, b.rows = nil, 0
	if err != nil {
		return err
	}
	if b.s.afterCommit != nil {
		return b.s.afterCommit()
	}
	return nil
}

// Rollback drops the pending transaction, if any.
func (b *Batch) Rollback() {
	if b.tx != nil {
		b.tx.Rollback()
		b.tx, b.rows = nil, 0
	}
}

// ListLibraries loads every library with its folders, ordered by name.
func (s *Store) ListLibraries(ctx context.Context) ([]Library, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM libraries ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Library, 0, len(ids))
	for _, id := range ids {
		lib, err := s.GetLibrary(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue // deleted in between
		}
		if err != nil {
			return nil, err
		}
		out = append(out, lib)
	}
	return out, nil
}

// Patch changes a library; nil fields are left as they are.
type Patch struct {
	Name *string
	// SetInterval applies ScanIntervalS, where nil means back to defaults.
	SetInterval   bool
	ScanIntervalS *int64
	// Folders, when set, is the new full list of cleaned paths.
	Folders []string
	// MetadataLanguage and MetadataRegion, when set, change the language
	// F005 fetches metadata in (R10).
	MetadataLanguage *string
	MetadataRegion   *string
}

// UpdateLibrary applies p and returns the ids of folders it added. Removed
// folders take their files with them (ON DELETE CASCADE); items left
// without files are cleaned up by the next scan.
func (s *Store) UpdateLibrary(ctx context.Context, id string, p Patch) (added []string, err error) {
	now := s.now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE libraries SET updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	if p.Name != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE libraries SET name = ? WHERE id = ?`, *p.Name, id); err != nil {
			return nil, err
		}
	}
	if p.SetInterval {
		if _, err := tx.ExecContext(ctx, `UPDATE libraries SET scan_interval_s = ? WHERE id = ?`,
			nullInt64Ptr(p.ScanIntervalS), id); err != nil {
			return nil, err
		}
	}
	if p.MetadataLanguage != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE libraries SET metadata_language = ? WHERE id = ?`, *p.MetadataLanguage, id); err != nil {
			return nil, err
		}
	}
	if p.MetadataRegion != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE libraries SET metadata_region = ? WHERE id = ?`, *p.MetadataRegion, id); err != nil {
			return nil, err
		}
	}
	if p.Folders != nil {
		keep := map[string]bool{}
		for _, f := range p.Folders {
			keep[f] = true
		}
		rows, err := tx.QueryContext(ctx, `SELECT id, path FROM library_folders WHERE library_id = ?`, id)
		if err != nil {
			return nil, err
		}
		existing := map[string]string{}
		for rows.Next() {
			var fid, path string
			if err := rows.Scan(&fid, &path); err != nil {
				_ = rows.Close()
				return nil, err
			}
			existing[path] = fid
		}
		_ = rows.Close()
		for path, fid := range existing {
			if !keep[path] {
				if _, err := tx.ExecContext(ctx, `DELETE FROM library_folders WHERE id = ?`, fid); err != nil {
					return nil, err
				}
			}
		}
		for _, path := range p.Folders {
			if _, ok := existing[path]; ok {
				continue
			}
			fid := newID()
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO library_folders (id, library_id, path, created_at) VALUES (?, ?, ?, ?)`,
				fid, id, path, now); err != nil {
				return nil, fmt.Errorf("insert folder %s: %w", path, err)
			}
			added = append(added, fid)
		}
	}
	return added, tx.Commit()
}

// SetFolderWatch records a folder's file system type and whether it is
// watched (R18).
func (s *Store) SetFolderWatch(ctx context.Context, folderID, fsType string, watched bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE library_folders SET fs_type = ?, watched = ? WHERE id = ?`,
		nullString(fsType), watched, folderID)
	return err
}

// Counts are the visible contents of a library.
type Counts struct {
	Movies, Shows, Episodes, Files, Unrecognized int
}

// LibraryCounts returns the counts of every library, by library id.
func (s *Store) LibraryCounts(ctx context.Context) (map[string]Counts, error) {
	out := map[string]Counts{}
	rows, err := s.db.QueryContext(ctx,
		`SELECT library_id, kind, COUNT(*) FROM items WHERE hidden = 0 AND kind IN ('movie','show','episode')
		 GROUP BY library_id, kind`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			id, kind string
			n        int
		)
		if err := rows.Scan(&id, &kind, &n); err != nil {
			_ = rows.Close()
			return nil, err
		}
		c := out[id]
		switch kind {
		case KindMovie:
			c.Movies = n
		case KindShow:
			c.Shows = n
		case KindEpisode:
			c.Episodes = n
		}
		out[id] = c
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx,
		`SELECT library_id, COUNT(*), COALESCE(SUM(parse_status = 'unrecognized'), 0)
		 FROM media_files WHERE missing_since IS NULL GROUP BY library_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			id      string
			files   int
			unknown int
		)
		if err := rows.Scan(&id, &files, &unknown); err != nil {
			return nil, err
		}
		c := out[id]
		c.Files, c.Unrecognized = files, unknown
		out[id] = c
	}
	return out, rows.Err()
}

// AllFolders returns every library folder with its library id, for the
// overlap check (R2).
func (s *Store) AllFolders(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, library_id FROM library_folders`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var path, lib string
		if err := rows.Scan(&path, &lib); err != nil {
			return nil, err
		}
		out[path] = lib
	}
	return out, rows.Err()
}

func nullInt64Ptr(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
