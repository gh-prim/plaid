package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// missingRetention is how long a missing file is kept before its row (and
// items left without files) are deleted (R17).
const missingRetention = 30 * 24 * time.Hour

// Scanner reconciles a library's folders with the database (R8, R10, R11,
// R14–R17). It only reads library folders, never writes to them (R23).
type Scanner struct {
	Store *Store
	// Prober is nil when ffprobe is missing: files are then marked skipped.
	Prober Prober
	// Workers bounds concurrent probes; 0 means min(4, NumCPU) (R24).
	Workers int
	Log     *slog.Logger
}

// ScanOptions tune one scan.
type ScanOptions struct {
	// Full re-probes every file, changed or not (R20 "Full rescan").
	Full bool
}

// Report counts what a scan did (R21).
type Report struct {
	Found        int // videos found in available folders
	New          int
	Changed      int
	Moved        int
	Returned     int // missing files found again at the same path
	Missing      int // files newly marked missing
	Purged       int // rows deleted after 30 days missing
	Analyzed     int // probed successfully
	Failed       int // probe failed
	Unrecognized int // found files whose name couldn't be parsed
	// Unavailable lists the ids of folders skipped by the R17 guard.
	Unavailable []string
}

// folderScan is the walk outcome of one available folder.
type folderScan struct {
	folder Folder
	walk   *WalkResult
}

// newFile is a walked file with no row at its path.
type newFile struct {
	folder      Folder
	file        WalkedFile
	fingerprint []byte
}

// analysisJob is a file to fingerprint and/or probe after reconciliation.
type analysisJob struct {
	id, path    string
	fingerprint bool
	probe       bool
}

func (s *Scanner) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// ScanLibrary walks every folder of a library and brings the database in
// line with what's on disk, then analyzes new and changed files.
//
// Safety rules, in order:
//   - A folder that is missing, unreadable, failing, or empty while the
//     database has present files for it is marked unavailable/unreadable and
//     none of its files are touched. This is decided before any file write.
//   - Sub-folders the walk couldn't read keep their files as they are.
//   - Rows are deleted only after 30 days missing, and only in folders that
//     were available in this scan.
func (s *Scanner) ScanLibrary(ctx context.Context, libraryID string, opts ScanOptions) (Report, error) {
	var rep Report
	st := s.Store
	lib, err := st.GetLibrary(ctx, libraryID)
	if err != nil {
		return rep, err
	}
	rows, err := st.libraryFiles(ctx, libraryID)
	if err != nil {
		return rep, err
	}
	byPath := make(map[string]*fileRow, len(rows))
	present := map[string]int{} // folder id → files not missing
	for i := range rows {
		byPath[rows[i].Path] = &rows[i]
		if rows[i].MissingSince == "" {
			present[rows[i].FolderID]++
		}
	}

	// 1. Walk, and apply the unavailable guard before any file write.
	var scans []folderScan
	for _, f := range lib.Folders {
		res, err := Walk(ctx, f.Path)
		status := FolderOK
		var fe *FolderError
		switch {
		case errors.As(err, &fe):
			status = fe.Status
			s.log().Warn("library folder skipped", "folder", f.Path, "status", status, "err", fe.Err)
		case err != nil:
			return rep, err
		case len(res.Files) == 0 && present[f.ID] > 0:
			// The classic unmounted drive: an empty mount point.
			status = FolderUnavailable
			s.log().Warn("library folder is empty but had files, treating it as unavailable", "folder", f.Path)
		}
		if status != f.Status {
			if err := st.setFolderStatus(ctx, f.ID, status); err != nil {
				return rep, err
			}
		}
		if status != FolderOK {
			rep.Unavailable = append(rep.Unavailable, f.ID)
			continue
		}
		scans = append(scans, folderScan{folder: f, walk: res})
	}

	idx, err := s.loadItemIndex(ctx, libraryID)
	if err != nil {
		return rep, err
	}
	extStreams, err := st.externalStreams(ctx, libraryID)
	if err != nil {
		return rep, err
	}

	b := st.NewBatch(ctx)
	defer b.Rollback()
	now := st.now()
	seen := map[string]bool{}
	var (
		news []*newFile
		jobs []analysisJob
	)
	needsProbe := func(r *fileRow) bool {
		return opts.Full || r.ProbeStatus == ProbePending || r.ProbeStatus == ProbeFailed ||
			r.ProbeStatus == ProbeSkipped && s.Prober != nil
	}

	// 2. Files already known at the same path.
	for _, sc := range scans {
		for _, wf := range sc.walk.Files {
			rep.Found++
			r, ok := byPath[wf.Path]
			if !ok {
				news = append(news, &newFile{folder: sc.folder, file: wf})
				continue
			}
			seen[r.ID] = true
			mtime := mtimeString(wf.ModTime)
			unit := false
			if r.Size == wf.Size && r.MTime == mtime {
				if r.MissingSince != "" {
					if err := b.Exec(`UPDATE media_files SET missing_since = NULL, updated_at = ? WHERE id = ?`, now, r.ID); err != nil {
						return rep, err
					}
					rep.Returned++
					unit = true
				}
				if needsProbe(r) {
					jobs = append(jobs, analysisJob{id: r.ID, path: wf.Path, probe: true, fingerprint: r.Fingerprint == nil})
				}
			} else {
				if err := b.Exec(`UPDATE media_files SET size = ?, mtime = ?, fingerprint = NULL, probe_status = ?,
					missing_since = NULL, updated_at = ? WHERE id = ?`, wf.Size, mtime, ProbePending, now, r.ID); err != nil {
					return rep, err
				}
				rep.Changed++
				unit = true
				jobs = append(jobs, analysisJob{id: r.ID, path: wf.Path, probe: true, fingerprint: true})
			}
			// Parser improvements: retry names that weren't understood.
			if r.ParseStatus == ParseUnrecognized {
				if edition, ok, err := s.link(b, idx, lib, wf, r.ID); err != nil {
					return rep, err
				} else if ok {
					if err := b.Exec(`UPDATE media_files SET parse_status = ?, edition = ?, updated_at = ? WHERE id = ?`,
						ParseOK, nullString(edition), now, r.ID); err != nil {
						return rep, err
					}
					unit = true
				} else {
					rep.Unrecognized++
				}
			}
			if changed, err := syncExternalSubtitles(b, r.ID, wf.Subtitles, extStreams[r.ID]); err != nil {
				return rep, err
			} else if changed {
				unit = true
			}
			if unit {
				if err := b.EndUnit(); err != nil {
					return rep, err
				}
			}
		}
	}

	// 3. Rows that went missing in this scan.
	var gone []*fileRow
	available := map[string]*folderScan{}
	for i := range scans {
		available[scans[i].folder.ID] = &scans[i]
	}
	for i := range rows {
		r := &rows[i]
		sc := available[r.FolderID]
		if seen[r.ID] || r.MissingSince != "" || sc == nil || underSkipped(sc, r.Path) {
			continue
		}
		gone = append(gone, r)
	}

	// 4. Move detection (R16): a new file takes over the row of a file that
	// went missing in this scan when size and fingerprint match one, and
	// only one, on each side.
	moves := s.detectMoves(news, gone)
	for nf, r := range moves {
		edition := s.edition(lib, nf.file)
		if err := b.Exec(`UPDATE media_files SET path = ?, folder_id = ?, mtime = ?, edition = ?, updated_at = ? WHERE id = ?`,
			nf.file.Path, nf.folder.ID, mtimeString(nf.file.ModTime), nullString(edition), now, r.ID); err != nil {
			return rep, err
		}
		if _, err := syncExternalSubtitles(b, r.ID, nf.file.Subtitles, extStreams[r.ID]); err != nil {
			return rep, err
		}
		if err := b.EndUnit(); err != nil {
			return rep, err
		}
		rep.Moved++
		if needsProbe(r) {
			jobs = append(jobs, analysisJob{id: r.ID, path: nf.file.Path, probe: true})
		}
	}

	// 5. New files.
	for _, nf := range news {
		if _, moved := moves[nf]; moved {
			continue
		}
		id := newID()
		edition, parsed, err := s.parseOnly(lib, nf.file)
		if err != nil {
			return rep, err
		}
		status := ParseOK
		if !parsed {
			status = ParseUnrecognized
			rep.Unrecognized++
		}
		if err := b.Exec(`INSERT INTO media_files (id, library_id, folder_id, path, size, mtime, fingerprint, edition,
			parse_status, probe_status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, lib.ID, nf.folder.ID, nf.file.Path, nf.file.Size, mtimeString(nf.file.ModTime), nf.fingerprint,
			nullString(edition), status, ProbePending, now, now); err != nil {
			return rep, err
		}
		if parsed {
			if _, _, err := s.link(b, idx, lib, nf.file, id); err != nil {
				return rep, err
			}
		}
		if _, err := syncExternalSubtitles(b, id, nf.file.Subtitles, nil); err != nil {
			return rep, err
		}
		if err := b.EndUnit(); err != nil {
			return rep, err
		}
		rep.New++
		jobs = append(jobs, analysisJob{id: id, path: nf.file.Path, probe: true, fingerprint: nf.fingerprint == nil})
	}

	// 6. Mark what's left as missing; their items get hidden below.
	movedIDs := map[string]bool{}
	for _, r := range moves {
		movedIDs[r.ID] = true
	}
	for _, r := range gone {
		if movedIDs[r.ID] {
			continue
		}
		if err := b.Exec(`UPDATE media_files SET missing_since = ?, updated_at = ? WHERE id = ?`, now, now, r.ID); err != nil {
			return rep, err
		}
		if err := b.EndUnit(); err != nil {
			return rep, err
		}
		rep.Missing++
	}

	// 7. Purge rows missing for 30 days, in available folders only, then
	// items left without files, and recompute visibility.
	if len(scans) > 0 {
		cutoff := ts(st.Now().Add(-missingRetention))
		ids := make([]any, 0, len(scans)+2)
		ids = append(ids, lib.ID, cutoff)
		for _, sc := range scans {
			ids = append(ids, sc.folder.ID)
		}
		n, err := b.execCount(`DELETE FROM media_files WHERE library_id = ? AND missing_since IS NOT NULL
			AND missing_since <= ? AND folder_id IN (`+placeholders(len(scans))+`)`, ids...)
		if err != nil {
			return rep, err
		}
		rep.Purged = int(n)
	}
	for _, q := range []string{
		`DELETE FROM items WHERE library_id = ? AND kind IN ('movie', 'episode')
		 AND NOT EXISTS (SELECT 1 FROM item_files WHERE item_id = items.id)`,
		`DELETE FROM items WHERE library_id = ? AND kind = 'season'
		 AND NOT EXISTS (SELECT 1 FROM items c WHERE c.parent_id = items.id)`,
		`DELETE FROM items WHERE library_id = ? AND kind = 'show'
		 AND NOT EXISTS (SELECT 1 FROM items c WHERE c.parent_id = items.id)`,
	} {
		if err := b.Exec(q, lib.ID); err != nil {
			return rep, err
		}
	}
	if err := updateHidden(b, lib.ID, now); err != nil {
		return rep, err
	}
	if err := b.Commit(); err != nil {
		return rep, err
	}
	if err := st.setLastScan(ctx, lib.ID); err != nil {
		return rep, err
	}

	// 8. Analysis, in the background of the walk (R12, R24).
	if err := s.analyze(ctx, jobs, &rep); err != nil {
		return rep, err
	}
	s.log().Info("library scanned", "library", lib.Name, "found", rep.Found, "new", rep.New,
		"changed", rep.Changed, "moved", rep.Moved, "missing", rep.Missing, "purged", rep.Purged,
		"analyzed", rep.Analyzed, "failed", rep.Failed, "unrecognized", rep.Unrecognized,
		"unavailable", len(rep.Unavailable))
	return rep, nil
}

func underSkipped(sc *folderScan, path string) bool {
	for _, d := range sc.walk.SkippedDirs {
		prefix := strings.TrimSuffix(sc.folder.Path, "/") + "/" + d + "/"
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// detectMoves pairs new files with rows that went missing in this scan.
// Ambiguous matches (two candidates on either side) are left alone: they
// become "new + missing", which loses nothing.
func (s *Scanner) detectMoves(news []*newFile, gone []*fileRow) map[*newFile]*fileRow {
	bySize := map[int64][]*fileRow{}
	for _, r := range gone {
		if r.Fingerprint != nil {
			bySize[r.Size] = append(bySize[r.Size], r)
		}
	}
	matches := map[*newFile]*fileRow{}
	claims := map[*fileRow]int{}
	for _, nf := range news {
		cands := bySize[nf.file.Size]
		if len(cands) == 0 {
			continue
		}
		fp, err := Fingerprint(nf.file.Path)
		if err != nil {
			s.log().Debug("fingerprint failed", "path", nf.file.Path, "err", err)
			continue
		}
		nf.fingerprint = fp
		var hit []*fileRow
		for _, r := range cands {
			if bytes.Equal(r.Fingerprint, fp) {
				hit = append(hit, r)
			}
		}
		if len(hit) == 1 {
			matches[nf] = hit[0]
			claims[hit[0]]++
		}
	}
	for nf, r := range matches {
		if claims[r] > 1 {
			delete(matches, nf)
		}
	}
	return matches
}

// parseOnly parses a walked file for its edition and whether it's recognized.
func (s *Scanner) parseOnly(lib Library, wf WalkedFile) (edition string, ok bool, err error) {
	switch lib.Type {
	case TypeMovies:
		m, ok := ParseMovie(wf.Rel)
		return m.Edition, ok, nil
	case TypeShows:
		_, ok := ParseEpisode(wf.Rel)
		return "", ok, nil
	}
	return "", false, fmt.Errorf("unknown library type %q", lib.Type)
}

func (s *Scanner) edition(lib Library, wf WalkedFile) string {
	e, _, _ := s.parseOnly(lib, wf)
	return e
}

// itemIndex finds existing items without a query per file.
type itemIndex struct {
	movies   map[string]string // normTitle \x00 year → id
	shows    map[string]string // normTitle \x00 year → id
	seasons  map[string]string // show id / season → id
	episodes map[string]string // season id / episode, or season id / d / date → id
}

func movieKey(title string, year int) string { return normTitle(title) + "\x00" + strconv.Itoa(year) }
func seasonKey(showID string, n int) string  { return showID + "/" + strconv.Itoa(n) }
func episodeKey(seasonID string, n int) string {
	return seasonID + "/" + strconv.Itoa(n)
}
func dateKey(seasonID, date string) string { return seasonID + "/d/" + date }

func (s *Scanner) loadItemIndex(ctx context.Context, libraryID string) (*itemIndex, error) {
	items, err := s.Store.libraryItems(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	idx := &itemIndex{movies: map[string]string{}, shows: map[string]string{}, seasons: map[string]string{}, episodes: map[string]string{}}
	for _, it := range items {
		switch it.Kind {
		case KindMovie:
			idx.movies[movieKey(it.Title, it.Year)] = it.ID
		case KindShow:
			idx.shows[movieKey(it.Title, it.Year)] = it.ID
		case KindSeason:
			idx.seasons[seasonKey(it.ParentID, it.Season)] = it.ID
		case KindEpisode:
			if it.AirDate != "" && it.Episode < 0 {
				idx.episodes[dateKey(it.ParentID, it.AirDate)] = it.ID
			} else {
				idx.episodes[episodeKey(it.ParentID, it.Episode)] = it.ID
			}
		}
	}
	return idx, nil
}

// link parses a file and links it to its items, creating them on demand:
// movies are grouped by (normalized title, year) whatever their edition
// (R8); shows and seasons are created as needed and a multi-episode file is
// linked to each episode (R10). ok is false for an unrecognized name.
func (s *Scanner) link(b *Batch, idx *itemIndex, lib Library, wf WalkedFile, fileID string) (edition string, ok bool, err error) {
	now := s.Store.now()
	insert := func(kind, parentID, title string, year, season, episode int, airDate, hints string) (string, error) {
		id := newID()
		err := b.Exec(`INSERT INTO items (id, library_id, kind, parent_id, title, sort_title, year, season_number,
			episode_number, air_date, hints, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, lib.ID, kind, nullString(parentID), nullString(title), nullString(sortTitle(title)), nullInt(year),
			nullIntNeg(season), nullIntNeg(episode), nullString(airDate), nullString(hints), now, now)
		return id, err
	}
	linkTo := func(itemID string) error {
		return b.Exec(`INSERT OR IGNORE INTO item_files (item_id, file_id) VALUES (?, ?)`, itemID, fileID)
	}

	switch lib.Type {
	case TypeMovies:
		m, ok := ParseMovie(wf.Rel)
		if !ok {
			return "", false, nil
		}
		key := movieKey(m.Title, m.Year)
		id, found := idx.movies[key]
		if !found {
			hints := ""
			if len(m.Hints) > 0 {
				raw, _ := json.Marshal(m.Hints)
				hints = string(raw)
			}
			if id, err = insert(KindMovie, "", m.Title, m.Year, -1, -1, "", hints); err != nil {
				return "", false, err
			}
			idx.movies[key] = id
		}
		return m.Edition, true, linkTo(id)

	case TypeShows:
		e, ok := ParseEpisode(wf.Rel)
		if !ok {
			return "", false, nil
		}
		key := movieKey(e.Show, e.ShowYear)
		showID, found := idx.shows[key]
		if !found {
			if showID, err = insert(KindShow, "", e.Show, e.ShowYear, -1, -1, "", ""); err != nil {
				return "", false, err
			}
			idx.shows[key] = showID
		}
		sk := seasonKey(showID, e.Season)
		seasonID, found := idx.seasons[sk]
		if !found {
			if seasonID, err = insert(KindSeason, showID, "", 0, e.Season, -1, "", ""); err != nil {
				return "", false, err
			}
			idx.seasons[sk] = seasonID
		}
		if e.AirDate != "" && len(e.Episodes) == 0 {
			dk := dateKey(seasonID, e.AirDate)
			epID, found := idx.episodes[dk]
			if !found {
				if epID, err = insert(KindEpisode, seasonID, "", 0, -1, -1, e.AirDate, ""); err != nil {
					return "", false, err
				}
				idx.episodes[dk] = epID
			}
			return "", true, linkTo(epID)
		}
		for _, n := range e.Episodes {
			ek := episodeKey(seasonID, n)
			epID, found := idx.episodes[ek]
			if !found {
				if epID, err = insert(KindEpisode, seasonID, "", 0, -1, n, e.AirDate, ""); err != nil {
					return "", false, err
				}
				idx.episodes[ek] = epID
			}
			if err := linkTo(epID); err != nil {
				return "", false, err
			}
		}
		return "", true, nil
	}
	return "", false, fmt.Errorf("unknown library type %q", lib.Type)
}

// syncExternalSubtitles rewrites a file's external subtitle streams when the
// set of subtitle files changed.
func syncExternalSubtitles(b *Batch, fileID string, subs []ExternalSubtitle, stored []string) (bool, error) {
	same := len(subs) == len(stored)
	for i := 0; same && i < len(subs); i++ {
		same = subs[i].Path == stored[i]
	}
	if same {
		return false, nil
	}
	if len(stored) > 0 {
		if err := b.Exec(`DELETE FROM media_streams WHERE file_id = ? AND idx >= ?`, fileID, externalSubtitleBase); err != nil {
			return false, err
		}
	}
	for i, sub := range subs {
		if err := b.Exec(`INSERT INTO media_streams (file_id, idx, type, codec, language, is_forced, is_sdh, is_image,
			external_path) VALUES (?, ?, 'subtitle', ?, ?, ?, ?, ?, ?)`,
			fileID, externalSubtitleBase+i, sub.Codec, sub.Language, sub.Forced, sub.SDH, sub.Image, sub.Path); err != nil {
			return false, err
		}
	}
	return true, nil
}

// updateHidden hides items with no file left on disk and shows those that
// have one again; seasons and shows follow their children (R17).
func updateHidden(b *Batch, libraryID, now string) error {
	const present = `EXISTS (SELECT 1 FROM item_files f JOIN media_files m ON m.id = f.file_id
		WHERE f.item_id = items.id AND m.missing_since IS NULL)`
	const visibleChild = `EXISTS (SELECT 1 FROM items c WHERE c.parent_id = items.id AND c.hidden = 0)`
	for _, q := range []struct{ kinds, cond string }{
		{`'movie', 'episode'`, present},
		{`'season'`, visibleChild},
		{`'show'`, visibleChild},
	} {
		if err := b.Exec(`UPDATE items SET hidden = NOT `+q.cond+`, updated_at = ?
			WHERE library_id = ? AND kind IN (`+q.kinds+`) AND hidden = `+q.cond, now, libraryID); err != nil {
			return err
		}
	}
	return nil
}

// analyze fingerprints and probes files with a bounded worker pool; results
// are written by this goroutine in batches.
func (s *Scanner) analyze(ctx context.Context, jobs []analysisJob, rep *Report) error {
	if len(jobs) == 0 {
		return nil
	}
	workers := s.Workers
	if workers <= 0 {
		workers = min(4, runtime.NumCPU())
	}
	// Cancelled on return so workers never block on an early error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		job         analysisJob
		fingerprint []byte
		probe       *ProbeResult
		probeErr    error
	}
	in := make(chan analysisJob)
	out := make(chan result)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range in {
				r := result{job: j}
				if j.fingerprint {
					fp, err := Fingerprint(j.path)
					if err != nil {
						s.log().Debug("fingerprint failed", "path", j.path, "err", err)
					}
					r.fingerprint = fp
				}
				if j.probe && s.Prober != nil {
					r.probe, r.probeErr = s.Prober.Probe(ctx, j.path)
				}
				select {
				case out <- r:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(in)
		for _, j := range jobs {
			select {
			case in <- j:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(out) }()

	b := s.Store.NewBatch(ctx)
	defer b.Rollback()
	for r := range out {
		if ctx.Err() != nil {
			continue // drain
		}
		if err := s.writeAnalysis(b, r.job, r.fingerprint, r.probe, r.probeErr, rep); err != nil {
			return err
		}
		if err := b.EndUnit(); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return b.Commit()
}

func (s *Scanner) writeAnalysis(b *Batch, j analysisJob, fp []byte, res *ProbeResult, probeErr error, rep *Report) error {
	now := s.Store.now()
	if fp != nil {
		if err := b.Exec(`UPDATE media_files SET fingerprint = ? WHERE id = ?`, fp, j.id); err != nil {
			return err
		}
	}
	if !j.probe {
		return nil
	}
	switch {
	case s.Prober == nil:
		return b.Exec(`UPDATE media_files SET probe_status = ?, updated_at = ? WHERE id = ?`, ProbeSkipped, now, j.id)
	case probeErr != nil:
		rep.Failed++
		s.log().Warn("probe failed", "path", j.path, "err", probeErr)
		return b.Exec(`UPDATE media_files SET probe_status = ?, probed_at = ?, updated_at = ? WHERE id = ?`,
			ProbeFailed, now, now, j.id)
	}
	rep.Analyzed++
	var width, height int
	var codec, hdr string
	if v := res.Video(); v != nil {
		width, height, codec, hdr = v.Width, v.Height, v.Codec, v.HDR
	}
	if err := b.Exec(`UPDATE media_files SET probe_status = ?, container = ?, duration_ms = ?, bitrate = ?, width = ?,
		height = ?, video_codec = ?, hdr_format = ?, probe_json = ?, probed_at = ?, updated_at = ? WHERE id = ?`,
		ProbeOK, nullString(res.Container), nullInt64(res.DurationMs), nullInt64(res.Bitrate), nullInt(width),
		nullInt(height), nullString(codec), nullString(hdr), string(res.Raw), now, now, j.id); err != nil {
		return err
	}
	if err := b.Exec(`DELETE FROM media_streams WHERE file_id = ? AND idx < ?`, j.id, externalSubtitleBase); err != nil {
		return err
	}
	for _, st := range res.Streams {
		var channels int
		if st.Type == "audio" {
			channels = st.Channels
		}
		if err := b.Exec(`INSERT INTO media_streams (file_id, idx, type, codec, language, title, channels, is_default,
			is_forced, is_sdh, is_image) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			j.id, st.Index, st.Type, nullString(st.Codec), st.Language, nullString(st.Title), nullInt(channels),
			st.Default, st.Forced, st.SDH, st.Image); err != nil {
			return err
		}
	}
	return nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullInt64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

// nullIntNeg keeps 0 (season 0 = Specials) and maps negatives to NULL.
func nullIntNeg(n int) any {
	if n < 0 {
		return nil
	}
	return n
}
