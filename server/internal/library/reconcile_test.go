package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeProber returns a small result for any file and counts its calls.
// Files whose name contains "broken" fail.
type fakeProber struct {
	mu    sync.Mutex
	calls []string
}

func (p *fakeProber) Probe(_ context.Context, path string) (*ProbeResult, error) {
	p.mu.Lock()
	p.calls = append(p.calls, path)
	p.mu.Unlock()
	if strings.Contains(filepath.Base(path), "broken") {
		return nil, errors.New("invalid data")
	}
	return &ProbeResult{
		Container: "matroska,webm", DurationMs: 1000, Bitrate: 1000, Raw: []byte(`{}`),
		Streams: []Stream{
			{Index: 0, Type: "video", Codec: "hevc", Width: 3840, Height: 2160, HDR: HDR10, Language: "und"},
			{Index: 1, Type: "audio", Codec: "eac3", Language: "eng", Channels: 6, Default: true},
			{Index: 2, Type: "audio", Codec: "aac", Language: "fre", Channels: 2},
		},
	}, nil
}

func (p *fakeProber) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.calls)
	p.calls = nil
	return n
}

type scanEnv struct {
	t      *testing.T
	db     *sql.DB
	store  *Store
	scan   *Scanner
	prober *fakeProber
	lib    Library
	root   string // first folder
	now    time.Time
}

func newScanEnv(t *testing.T, typ string, extraFolders ...string) *scanEnv {
	t.Helper()
	d := openTestDB(t)
	e := &scanEnv{t: t, db: d, store: NewStore(d), prober: &fakeProber{}, now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	e.store.Now = func() time.Time { return e.now }
	e.scan = &Scanner{Store: e.store, Prober: e.prober, Workers: 2}
	e.root = t.TempDir()
	lib, err := e.store.CreateLibrary(context.Background(), "Lib", typ, append([]string{e.root}, extraFolders...))
	if err != nil {
		t.Fatal(err)
	}
	e.lib = lib
	return e
}

func (e *scanEnv) run(opts ScanOptions) Report {
	e.t.Helper()
	rep, err := e.scan.ScanLibrary(context.Background(), e.lib.ID, opts)
	if err != nil {
		e.t.Fatalf("scan: %v", err)
	}
	return rep
}

func (e *scanEnv) write(rel string, content string) string {
	e.t.Helper()
	p := filepath.Join(e.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *scanEnv) path(rel string) string { return filepath.Join(e.root, filepath.FromSlash(rel)) }

// file returns a column of the file at path.
func (e *scanEnv) fileCol(path, col string) sql.NullString {
	e.t.Helper()
	var v sql.NullString
	err := e.db.QueryRow(`SELECT `+col+` FROM media_files WHERE path = ?`, path).Scan(&v)
	if err != nil {
		e.t.Fatalf("file %s: %v", path, err)
	}
	return v
}

// itemsOf returns the item ids linked to the file at path.
func (e *scanEnv) itemsOf(path string) []string {
	e.t.Helper()
	rows, err := e.db.Query(`SELECT f.item_id FROM item_files f JOIN media_files m ON m.id = f.file_id
		WHERE m.path = ? ORDER BY f.item_id`, path)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			e.t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func (e *scanEnv) hidden(itemID string) bool {
	e.t.Helper()
	var h bool
	if err := e.db.QueryRow(`SELECT hidden FROM items WHERE id = ?`, itemID).Scan(&h); err != nil {
		e.t.Fatalf("item %s: %v", itemID, err)
	}
	return h
}

func (e *scanEnv) count(q string, args ...any) int { e.t.Helper(); return count(e.t, e.db, q, args...) }

func TestScanNewLibrary(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "matrix")
	e.write("The Matrix (1999)/The Matrix (1999).fr.forced.srt", "sub")
	e.write("Alien (1979) {tmdb-348}.mkv", "alien")
	e.write("broken (2001).mkv", "broken")

	rep := e.run(ScanOptions{})
	if rep.Found != 3 || rep.New != 3 || rep.Analyzed != 2 || rep.Failed != 1 || rep.Unrecognized != 0 {
		t.Errorf("report = %+v", rep)
	}
	if n := e.count(`SELECT COUNT(*) FROM items WHERE kind = 'movie'`); n != 3 {
		t.Errorf("movies = %d", n)
	}
	matrix := e.path("The Matrix (1999)/The Matrix (1999).mkv")
	if s := e.fileCol(matrix, "probe_status").String; s != ProbeOK {
		t.Errorf("probe_status = %q", s)
	}
	if s := e.fileCol(matrix, "hdr_format").String; s != HDR10 {
		t.Errorf("hdr_format = %q", s)
	}
	if e.fileCol(matrix, "fingerprint").String == "" {
		t.Error("no fingerprint after analysis")
	}
	if s := e.fileCol(e.path("broken (2001).mkv"), "probe_status").String; s != ProbeFailed {
		t.Errorf("broken probe_status = %q", s)
	}
	var hints string
	if err := e.db.QueryRow(`SELECT hints FROM items WHERE title = 'Alien'`).Scan(&hints); err != nil || hints != `{"tmdb":"348"}` {
		t.Errorf("hints = %q, %v", hints, err)
	}
	// Acceptance: English and French audio plus a forced French text subtitle.
	rows, err := e.db.Query(`SELECT s.type, s.language, s.is_forced, s.is_image FROM media_streams s
		JOIN media_files f ON f.id = s.file_id WHERE f.path = ? AND s.type <> 'video' ORDER BY s.idx`, matrix)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var typ, lang string
		var forced, image bool
		if err := rows.Scan(&typ, &lang, &forced, &image); err != nil {
			t.Fatal(err)
		}
		s := typ + ":" + lang
		if forced {
			s += ":forced"
		}
		if image {
			s += ":image"
		}
		got = append(got, s)
	}
	if want := []string{"audio:eng", "audio:fre", "subtitle:fre:forced"}; !slices.Equal(got, want) {
		t.Errorf("streams = %v, want %v", got, want)
	}
}

func TestRescanWithoutChangesProbesNothing(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	e.write("A (2001).mkv", "a")
	e.write("B (2002).mkv", "b")
	e.run(ScanOptions{})
	if n := e.prober.count(); n != 2 {
		t.Fatalf("first scan probed %d files", n)
	}
	rep := e.run(ScanOptions{})
	if rep.New != 0 || rep.Changed != 0 || rep.Missing != 0 || rep.Analyzed != 0 {
		t.Errorf("report = %+v", rep)
	}
	if n := e.prober.count(); n != 0 {
		t.Errorf("unchanged rescan probed %d files", n)
	}
	// Full rescan re-probes everything.
	e.run(ScanOptions{Full: true})
	if n := e.prober.count(); n != 2 {
		t.Errorf("full rescan probed %d files, want 2", n)
	}
}

func TestChangedFileIsReprobed(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	p := e.write("A (2001).mkv", "a")
	e.run(ScanOptions{})
	e.prober.count()
	itemBefore := e.itemsOf(p)
	e.write("A (2001).mkv", "a longer content")
	rep := e.run(ScanOptions{})
	if rep.Changed != 1 || e.prober.count() != 1 {
		t.Errorf("report = %+v", rep)
	}
	if !slices.Equal(e.itemsOf(p), itemBefore) {
		t.Error("changed file lost its item")
	}
}

func TestFailedProbeIsRetried(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	e.write("broken (2001).mkv", "x")
	e.run(ScanOptions{})
	e.prober.count()
	e.run(ScanOptions{})
	if n := e.prober.count(); n != 1 {
		t.Errorf("failed file probed %d times on rescan, want 1", n)
	}
}

func TestMoveInsideLibraryKeepsItem(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	old := e.write("Inbox/Heat (1995)/Heat (1995).mkv", strings.Repeat("heat", 1000))
	e.write("Inbox/Heat (1995)/Heat (1995).en.srt", "sub")
	e.run(ScanOptions{})
	items := e.itemsOf(old)
	id := e.fileCol(old, "id").String

	if err := os.MkdirAll(e.path("Movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(e.path("Inbox/Heat (1995)"), e.path("Movies/Heat (1995)")); err != nil {
		t.Fatal(err)
	}
	e.prober.count()
	rep := e.run(ScanOptions{})
	moved := e.path("Movies/Heat (1995)/Heat (1995).mkv")
	if rep.Moved != 1 || rep.New != 0 || rep.Missing != 0 {
		t.Errorf("report = %+v", rep)
	}
	if got := e.fileCol(moved, "id").String; got != id {
		t.Errorf("file id changed: %s → %s", id, got)
	}
	if !slices.Equal(e.itemsOf(moved), items) {
		t.Errorf("items changed: %v → %v", items, e.itemsOf(moved))
	}
	if n := e.prober.count(); n != 0 {
		t.Errorf("a moved file was probed again (%d)", n)
	}
	var ext string
	if err := e.db.QueryRow(`SELECT external_path FROM media_streams WHERE file_id = ? AND idx >= 1000`, id).Scan(&ext); err != nil || !strings.HasPrefix(ext, e.path("Movies")) {
		t.Errorf("external subtitle path = %q, %v", ext, err)
	}
}

func TestMoveAcrossFoldersOfALibrary(t *testing.T) {
	second := t.TempDir()
	e := newScanEnv(t, TypeMovies, second)
	old := e.write("Heat (1995).mkv", "heat content")
	e.run(ScanOptions{})
	items := e.itemsOf(old)
	// Keep the first folder non-empty so it isn't considered unmounted.
	e.write("Other (2000).mkv", "other")
	if err := os.Rename(old, filepath.Join(second, "Heat (1995).mkv")); err != nil {
		t.Fatal(err)
	}
	rep := e.run(ScanOptions{})
	if rep.Moved != 1 {
		t.Errorf("report = %+v", rep)
	}
	if !slices.Equal(e.itemsOf(filepath.Join(second, "Heat (1995).mkv")), items) {
		t.Error("move across folders lost the item")
	}
}

func TestAmbiguousMoveIsNotAMove(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	old := e.write("Heat (1995).mkv", "same bytes")
	e.write("Keep (2000).mkv", "keep")
	e.run(ScanOptions{})
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	e.write("Copy A (1995).mkv", "same bytes")
	e.write("Copy B (1995).mkv", "same bytes")
	rep := e.run(ScanOptions{})
	if rep.Moved != 0 || rep.New != 2 || rep.Missing != 1 {
		t.Errorf("report = %+v", rep)
	}
}

func TestEmptyMountPointIsUnavailable(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	e.write("A (2001)/A (2001).mkv", "a")
	e.write("B (2002)/B (2002).mkv", "b")
	e.run(ScanOptions{})

	// "Unmount": the mount point stays, empty.
	for _, d := range []string{"A (2001)", "B (2002)"} {
		if err := os.RemoveAll(e.path(d)); err != nil {
			t.Fatal(err)
		}
	}
	e.now = e.now.Add(40 * 24 * time.Hour)
	rep := e.run(ScanOptions{})
	if !slices.Equal(rep.Unavailable, []string{e.lib.Folders[0].ID}) || rep.Missing != 0 {
		t.Errorf("report = %+v", rep)
	}
	if n := e.count(`SELECT COUNT(*) FROM media_files WHERE missing_since IS NOT NULL`); n != 0 {
		t.Errorf("%d files got missing_since on an empty mount point", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM items WHERE hidden = 1`); n != 0 {
		t.Errorf("%d items hidden", n)
	}
	var status string
	if err := e.db.QueryRow(`SELECT status FROM library_folders WHERE id = ?`, e.lib.Folders[0].ID).Scan(&status); err != nil || status != FolderUnavailable {
		t.Errorf("folder status = %q, %v", status, err)
	}

	// "Re-plug": everything is back, folder ok.
	e.write("A (2001)/A (2001).mkv", "a")
	e.write("B (2002)/B (2002).mkv", "b")
	rep = e.run(ScanOptions{})
	if len(rep.Unavailable) != 0 || rep.New != 0 || rep.Missing != 0 {
		t.Errorf("after re-plug: %+v", rep)
	}
	if err := e.db.QueryRow(`SELECT status FROM library_folders WHERE id = ?`, e.lib.Folders[0].ID).Scan(&status); err != nil || status != FolderOK {
		t.Errorf("folder status = %q, %v", status, err)
	}
}

func TestMissingRootIsUnavailable(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	e.write("A (2001).mkv", "a")
	e.run(ScanOptions{})
	hidden := e.root + ".away"
	if err := os.Rename(e.root, hidden); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Rename(hidden, e.root) })
	e.now = e.now.Add(40 * 24 * time.Hour)
	rep := e.run(ScanOptions{})
	if len(rep.Unavailable) != 1 || rep.Missing != 0 || rep.Purged != 0 {
		t.Errorf("report = %+v", rep)
	}
	if n := e.count(`SELECT COUNT(*) FROM media_files WHERE missing_since IS NULL`); n != 1 {
		t.Errorf("file touched while its folder was gone")
	}
}

func TestUnreadableSubfolderKeepsItsFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	e := newScanEnv(t, TypeMovies)
	e.write("A (2001)/A (2001).mkv", "a")
	e.write("B (2002)/B (2002).mkv", "b")
	e.run(ScanOptions{})
	if err := os.Chmod(e.path("B (2002)"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(e.path("B (2002)"), 0o755) })
	rep := e.run(ScanOptions{})
	if rep.Missing != 0 {
		t.Errorf("report = %+v", rep)
	}
}

func TestDeletedFileHiddenThenPurgedAfter30Days(t *testing.T) {
	e := newScanEnv(t, TypeShows)
	ep := e.write("Show/Season 1/Show S01E01.mkv", "e1")
	e.write("Show/Season 1/Show S01E02.mkv", "e2")
	e.write("Other/Season 1/Other S01E01.mkv", "o1")
	e.run(ScanOptions{})
	episode := e.itemsOf(ep)[0]
	var season, show string
	if err := e.db.QueryRow(`SELECT s.id, s.parent_id FROM items s JOIN items ep ON ep.parent_id = s.id WHERE ep.id = ?`, episode).Scan(&season, &show); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(e.path("Show")); err != nil {
		t.Fatal(err)
	}
	rep := e.run(ScanOptions{})
	if rep.Missing != 2 {
		t.Errorf("report = %+v", rep)
	}
	for _, id := range []string{episode, season, show} {
		if !e.hidden(id) {
			t.Errorf("item %s not hidden", id)
		}
	}

	e.now = e.now.Add(29 * 24 * time.Hour)
	if rep := e.run(ScanOptions{}); rep.Purged != 0 {
		t.Errorf("purged before 30 days: %+v", rep)
	}
	e.now = e.now.Add(2 * 24 * time.Hour)
	if rep := e.run(ScanOptions{}); rep.Purged != 2 {
		t.Errorf("after 31 days: %+v", rep)
	}
	for _, id := range []string{episode, season, show} {
		if n := e.count(`SELECT COUNT(*) FROM items WHERE id = ?`, id); n != 0 {
			t.Errorf("item %s still there after the purge", id)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM items WHERE title = 'Other'`); n != 1 {
		t.Error("purge removed an unrelated show")
	}
}

func TestFileReturningOnDay29IsVisibleAgain(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	p := e.write("Heat (1995).mkv", "heat")
	e.write("Keep (2000).mkv", "keep")
	e.run(ScanOptions{})
	item := e.itemsOf(p)[0]
	backup, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	e.run(ScanOptions{})
	if !e.hidden(item) {
		t.Fatal("item not hidden")
	}

	e.now = e.now.Add(29 * 24 * time.Hour)
	if err := os.WriteFile(p, backup, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	rep := e.run(ScanOptions{})
	if rep.Returned != 1 || rep.New != 0 {
		t.Errorf("report = %+v", rep)
	}
	if e.hidden(item) || !slices.Equal(e.itemsOf(p), []string{item}) {
		t.Error("returning file isn't back on the same visible item")
	}
	if e.fileCol(p, "missing_since").Valid {
		t.Error("missing_since not cleared")
	}
	e.now = e.now.Add(5 * 24 * time.Hour)
	if rep := e.run(ScanOptions{}); rep.Purged != 0 {
		t.Errorf("returned file purged: %+v", rep)
	}
}

func TestPurgeSkipsUnavailableFolders(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	p := e.write("Gone (2001).mkv", "gone")
	e.write("Keep (2000).mkv", "keep")
	e.run(ScanOptions{})
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	e.run(ScanOptions{}) // Gone is missing from now on
	hidden := e.root + ".away"
	if err := os.Rename(e.root, hidden); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Rename(hidden, e.root) })
	e.now = e.now.Add(31 * 24 * time.Hour)
	if rep := e.run(ScanOptions{}); rep.Purged != 0 {
		t.Errorf("purged inside an unavailable folder: %+v", rep)
	}
}

func TestTwoVersionsOneMovie(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	a := e.write("Movie (2019)/Movie (2019) - 1080p.mkv", "1080")
	b := e.write("Movie (2019)/Movie (2019) - 2160p.mkv", "2160")
	c := e.write("Movie (2019)/Movie (2019) {edition-Director's Cut}.mkv", "dc")
	e.run(ScanOptions{})
	if n := e.count(`SELECT COUNT(*) FROM items WHERE kind = 'movie'`); n != 1 {
		t.Fatalf("movies = %d, want 1", n)
	}
	id := e.itemsOf(a)
	if !slices.Equal(e.itemsOf(b), id) || !slices.Equal(e.itemsOf(c), id) {
		t.Error("versions linked to different items")
	}
	if ed := e.fileCol(c, "edition").String; ed != "Director's Cut" {
		t.Errorf("edition = %q", ed)
	}
	if e.fileCol(a, "edition").Valid {
		t.Error("1080p file got an edition")
	}
}

func TestShowsStructureAndMultiEpisode(t *testing.T) {
	e := newScanEnv(t, TypeShows)
	multi := e.write("Lost/Staffel 1/Lost S01E01-E02.mkv", "m")
	e.write("Lost/Staffel 1/Lost S01E03.mkv", "3")
	e.write("Lost/Specials/Lost S00E01.mkv", "s")
	daily := e.write("The Daily Show/2024-10-27.mkv", "d")
	e.run(ScanOptions{})

	if n := e.count(`SELECT COUNT(*) FROM items WHERE kind = 'show'`); n != 2 {
		t.Errorf("shows = %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM items WHERE kind = 'season' AND parent_id = (SELECT id FROM items WHERE title = 'Lost')`); n != 2 {
		t.Errorf("Lost seasons = %d, want 2 (1 and specials)", n)
	}
	eps := e.itemsOf(multi)
	if len(eps) != 2 {
		t.Fatalf("multi-episode file linked to %d items", len(eps))
	}
	var numbers []int
	for _, id := range eps {
		var n int
		if err := e.db.QueryRow(`SELECT episode_number FROM items WHERE id = ?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		numbers = append(numbers, n)
	}
	slices.Sort(numbers)
	if !slices.Equal(numbers, []int{1, 2}) {
		t.Errorf("episodes = %v", numbers)
	}
	var air string
	if err := e.db.QueryRow(`SELECT air_date FROM items WHERE id = ?`, e.itemsOf(daily)[0]).Scan(&air); err != nil || air != "2024-10-27" {
		t.Errorf("air_date = %q, %v", air, err)
	}
	// Rescan reuses every item.
	before := e.count(`SELECT COUNT(*) FROM items`)
	e.run(ScanOptions{})
	if after := e.count(`SELECT COUNT(*) FROM items`); after != before {
		t.Errorf("items %d → %d on rescan", before, after)
	}
}

func TestUnrecognizedFilesAreStored(t *testing.T) {
	e := newScanEnv(t, TypeShows)
	p := e.write("Show/Season 1/Episode One.mkv", "x")
	rep := e.run(ScanOptions{})
	if rep.Unrecognized != 1 || rep.New != 1 {
		t.Errorf("report = %+v", rep)
	}
	if s := e.fileCol(p, "parse_status").String; s != ParseUnrecognized {
		t.Errorf("parse_status = %q", s)
	}
	if len(e.itemsOf(p)) != 0 {
		t.Error("unrecognized file linked to an item")
	}
	if s := e.fileCol(p, "probe_status").String; s != ProbeOK {
		t.Errorf("unrecognized files are still analyzed, probe_status = %q", s)
	}
}

func TestFFprobeMissingMarksSkipped(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	e.scan.Prober = nil
	p := e.write("A (2001).mkv", "a")
	e.run(ScanOptions{})
	if s := e.fileCol(p, "probe_status").String; s != ProbeSkipped {
		t.Errorf("probe_status = %q", s)
	}
	if e.fileCol(p, "fingerprint").String == "" {
		t.Error("no fingerprint without ffprobe")
	}
	// ffprobe installed later: skipped files get analyzed.
	e.scan.Prober = e.prober
	e.run(ScanOptions{})
	if s := e.fileCol(p, "probe_status").String; s != ProbeOK {
		t.Errorf("after install, probe_status = %q", s)
	}
}

// snapshot dumps the comparable state of a library (no ids, no timestamps).
func (e *scanEnv) snapshot() []string {
	e.t.Helper()
	rows, err := e.db.Query(`SELECT m.path, m.parse_status, m.probe_status, COALESCE(m.missing_since, ''),
		COALESCE(i.kind, ''), COALESCE(i.title, ''), COALESCE(i.season_number, -1), COALESCE(i.episode_number, -1), COALESCE(i.hidden, -1)
		FROM media_files m LEFT JOIN item_files f ON f.file_id = m.id LEFT JOIN items i ON i.id = f.item_id
		ORDER BY 1, 5, 7, 8`)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a, b, c, d, k, ti string
		var s, ep, h int
		if err := rows.Scan(&a, &b, &c, &d, &k, &ti, &s, &ep, &h); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, strings.Join([]string{strings.TrimPrefix(a, e.root), b, c, d, k, ti, itoa(s), itoa(ep), itoa(h)}, "|"))
	}
	return out
}

func TestCrashBetweenBatchesIsCompletedByNextScan(t *testing.T) {
	build := func(e *scanEnv) {
		for i := range 30 {
			e.write("Show/Season 1/Show S01E"+itoa(i+1)+".mkv", "ep"+itoa(i))
		}
		e.write("Show/Season 1/notes episode.mkv", "unrecognized")
	}
	clean := newScanEnv(t, TypeShows)
	build(clean)
	clean.run(ScanOptions{})
	want := clean.snapshot()

	e := newScanEnv(t, TypeShows)
	build(e)
	e.store.batchSize = 10
	commits := 0
	e.store.afterCommit = func() error {
		commits++
		if commits == 2 {
			return errors.New("simulated crash")
		}
		return nil
	}
	if _, err := e.scan.ScanLibrary(context.Background(), e.lib.ID, ScanOptions{}); err == nil {
		t.Fatal("crash not reported")
	}
	partial := e.count(`SELECT COUNT(*) FROM media_files`)
	if partial == 0 || partial == 31 {
		t.Fatalf("crash left %d files, want a partial state", partial)
	}
	if n := e.count(`SELECT COUNT(*) FROM media_files m WHERE parse_status = 'ok'
		AND NOT EXISTS (SELECT 1 FROM item_files WHERE file_id = m.id)`); n != 0 {
		t.Errorf("%d recognized files without items: a unit was split", n)
	}

	e.store.afterCommit = nil
	e.run(ScanOptions{})
	if got := e.snapshot(); !slices.Equal(got, want) {
		t.Errorf("state after recovery differs from a clean scan\ngot  %v\nwant %v", got, want)
	}
}

func TestScanCancelled(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	e.write("A (2001).mkv", "a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.scan.ScanLibrary(ctx, e.lib.ID, ScanOptions{}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestScanUnknownLibrary(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	if _, err := e.scan.ScanLibrary(context.Background(), "nope", ScanOptions{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
