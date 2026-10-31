package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gh-prim/plaid/server/internal/metadata/certs"
	"github.com/gh-prim/plaid/server/internal/metadata/match"
	"github.com/gh-prim/plaid/server/internal/metadata/nfo"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
)

// Match sources, as item_metadata.match_source stores them.
const (
	sourceNFO    = "nfo"
	sourceHint   = "hint"
	sourceAuto   = "auto"
	sourceManual = "manual"
	sourceParent = "parent"
)

// resolved is how an item was identified.
type resolved struct {
	tmdbID     int
	source     string
	confidence float64
	// decision is set when matching ran a search.
	decision *match.Decision
}

// resolve applies R3 then R4/R5, in order: a locked manual match, an .nfo
// id, the current match (refreshes), a file name hint, then a search.
func (s *Service) resolve(ctx context.Context, prov provider.Provider, kind string, p parsed, cur row, doc *nfo.NFO) (resolved, error) {
	if cur.MatchLocked && cur.TMDBID > 0 {
		return resolved{tmdbID: cur.TMDBID, source: sourceManual, confidence: 1}, nil
	}
	ids := []struct {
		source, tmdb, imdb string
	}{}
	if doc != nil {
		ids = append(ids, struct{ source, tmdb, imdb string }{sourceNFO, itoa(doc.TMDBID), doc.IMDbID})
	}
	for _, c := range ids {
		if n, _ := strconv.Atoi(c.tmdb); n > 0 {
			return resolved{tmdbID: n, source: c.source, confidence: 1}, nil
		}
		if c.imdb != "" {
			n, err := prov.FindIMDb(ctx, kind, c.imdb)
			if err != nil {
				return resolved{}, err
			}
			if n > 0 {
				return resolved{tmdbID: n, source: c.source, confidence: 1}, nil
			}
		}
	}
	// A refresh keeps the current match: it never searches again, so a
	// change in popularity can't send a matched item back to review.
	if cur.State == match.Matched && cur.TMDBID > 0 {
		return resolved{tmdbID: cur.TMDBID, source: cur.MatchSource, confidence: cur.Confidence}, nil
	}
	ids = []struct{ source, tmdb, imdb string }{{sourceHint, p.Hints["tmdb"], p.Hints["imdb"]}}
	for _, c := range ids {
		if n, _ := strconv.Atoi(c.tmdb); n > 0 {
			return resolved{tmdbID: n, source: c.source, confidence: 1}, nil
		}
		if c.imdb != "" {
			n, err := prov.FindIMDb(ctx, kind, c.imdb)
			if err != nil {
				return resolved{}, err
			}
			if n > 0 {
				return resolved{tmdbID: n, source: c.source, confidence: 1}, nil
			}
		}
	}
	title, year := p.Title, p.Year
	if doc != nil && doc.Title != "" {
		title, year = doc.Title, doc.Year
	}
	if title == "" {
		d := match.Decision{State: match.Unmatched}
		return resolved{decision: &d}, nil
	}
	d, err := match.Run(ctx, prov, kind, title, year)
	if err != nil {
		return resolved{}, err
	}
	r := resolved{decision: &d}
	if d.Best != nil {
		r.tmdbID, r.source, r.confidence = d.Best.TMDBID, sourceAuto, d.Best.Confidence
	}
	return r, nil
}

func (s *Service) logNFO(path string, err error) {
	if err != nil {
		s.log.Warn("unreadable .nfo skipped", "path", path, "err", err)
	}
}

func (s *Service) processMovie(ctx context.Context, p parsed) error {
	cur, err := s.store.loadRow(ctx, s.store.db, p.ID)
	if err != nil {
		return err
	}
	dirs, bases, err := s.store.itemDirs(ctx, p.ID)
	if err != nil {
		return err
	}
	names := []string{"movie.nfo"}
	for _, b := range bases {
		names = append(names, b+".nfo")
	}
	doc, path, nerr := readNFO(dirs, names)
	s.logNFO(path, nerr)

	localArt := func() { s.artworkAfter(ctx, p, func() error { return s.movieArtwork(ctx, p, nil) }) }
	prov := s.provider()
	if prov == nil {
		if err := s.store.writeLocal(ctx, p, cur, doc); err != nil {
			return err
		}
		localArt()
		return ErrPaused
	}
	res, err := s.resolve(ctx, prov, provider.Movie, p, cur, doc)
	if err != nil {
		return err
	}
	if res.tmdbID == 0 {
		if err := s.store.writeUnresolved(ctx, p, cur, doc, res.decision); err != nil {
			return err
		}
		localArt()
		return nil
	}
	d, err := prov.Movie(ctx, res.tmdbID, p.Locale)
	if err != nil {
		return err
	}
	collectionID := ""
	if d.Collection != nil {
		// A collection is a nice-to-have (R23): if it can't be fetched,
		// the movie is stored without it and the next refresh tries again.
		if collectionID, err = s.ensureCollection(ctx, prov, p, d.Collection); err != nil {
			s.log.Warn("collection not fetched", "collection", d.Collection.TMDBID, "err", err)
			collectionID = ""
		}
	}
	if err := s.store.inTx(ctx, func(tx *sql.Tx) error {
		r := build(p, cur, d, doc, res, certs.Movie)
		r.CollectionID = collectionID
		if err := s.store.writeRow(ctx, tx, p.ID, r, refreshAt(s.o.Now(), d)); err != nil {
			return err
		}
		if err := s.store.writeCredits(ctx, tx, p.ID, d.Cast, d.Crew); err != nil {
			return err
		}
		return s.store.markDuplicates(ctx, tx, p, r.TMDBID)
	}); err != nil {
		return err
	}
	// Artwork comes after the metadata is stored: an image that can't be
	// fetched never undoes a match.
	s.artworkAfter(ctx, p, func() error { return s.movieArtwork(ctx, p, d) }, d.Cast, d.Crew)
	return nil
}

// artworkRetry is how soon an item whose artwork or portraits couldn't be
// written (a busy database, a full disk) is refreshed again, instead of
// waiting weeks for its scheduled refresh with no stills or portraits.
const artworkRetry = 10 * time.Minute

// retryArtwork brings the item's refresh forward to artworkRetry from now.
func (s *Service) retryArtwork(ctx context.Context, id string) {
	if err := s.store.refreshBy(ctx, id, s.o.Now().Add(artworkRetry)); err != nil {
		s.log.Error("artwork retry not scheduled", "item", id, "err", err)
	}
}

// artworkAfter runs an artwork sync and records portraits, logging errors
// and trying again soon after one.
func (s *Service) artworkAfter(ctx context.Context, p parsed, sync func() error, people ...[]provider.Person) {
	if s.art == nil {
		return
	}
	if err := sync(); err != nil {
		s.log.Error("artwork not updated", "item", p.ID, "err", err)
		s.retryArtwork(ctx, p.ID)
	}
	for _, list := range people {
		if err := s.store.portraits(ctx, list); err != nil {
			s.log.Error("portraits not recorded", "item", p.ID, "err", err)
			s.retryArtwork(ctx, p.ID)
		}
	}
}

func (s *Service) processShow(ctx context.Context, p parsed) error {
	cur, err := s.store.loadRow(ctx, s.store.db, p.ID)
	if err != nil {
		return err
	}
	dir, err := s.store.showDir(ctx, p.ID)
	if err != nil {
		return err
	}
	var doc *nfo.NFO
	if dir != "" {
		var path string
		var nerr error
		doc, path, nerr = readNFO([]string{dir}, []string{"tvshow.nfo"})
		s.logNFO(path, nerr)
	}

	localArt := func() { s.artworkAfter(ctx, p, func() error { return s.showArtwork(ctx, p, nil, nil, nil) }) }
	prov := s.provider()
	if prov == nil {
		if err := s.store.writeLocal(ctx, p, cur, doc); err != nil {
			return err
		}
		localArt()
		return ErrPaused
	}
	res, err := s.resolve(ctx, prov, provider.Show, p, cur, doc)
	if err != nil {
		return err
	}
	if res.tmdbID == 0 {
		if err := s.store.writeUnresolved(ctx, p, cur, doc, res.decision); err != nil {
			return err
		}
		localArt()
		return nil
	}
	d, err := prov.Show(ctx, res.tmdbID, p.Locale)
	if err != nil {
		return err
	}
	children, err := s.store.children(ctx, p.ID)
	if err != nil {
		return err
	}
	seasons, err := s.fetchSeasons(ctx, prov, d, children, p.Locale)
	if err != nil {
		return err
	}
	// Everything fetched: write the seasons and their episodes one season
	// per transaction (a long show in one would hold the write lock for
	// seconds, and every other writer would time out), then the show. Its
	// row goes last, so a failure half-way leaves it due: the next attempt
	// writes everything again (R33).
	r := build(p, cur, d, doc, res, certs.TV)
	for _, group := range bySeasonGroup(children) {
		if err := s.store.inTx(ctx, func(tx *sql.Tx) error {
			return s.store.writeChildren(ctx, tx, r, group, seasons)
		}); err != nil {
			return err
		}
	}
	if err := s.store.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.store.writeRow(ctx, tx, p.ID, r, refreshAt(s.o.Now(), d)); err != nil {
			return err
		}
		if err := s.store.writeCredits(ctx, tx, p.ID, d.Cast, d.Crew); err != nil {
			return err
		}
		return s.store.markDuplicates(ctx, tx, p, r.TMDBID)
	}); err != nil {
		return err
	}
	s.artworkAfter(ctx, p, func() error { return s.showArtwork(ctx, p, d, children, seasons) }, d.Cast, d.Crew)
	return nil
}

// bySeasonGroup splits a show's children into a season and its episodes
// each, in order; episodes whose season isn't listed go in a group of
// their own.
func bySeasonGroup(children []child) [][]child {
	idx := map[string]int{}
	var groups [][]child
	for _, c := range children {
		if c.Kind == "season" {
			idx[c.ID] = len(groups)
			groups = append(groups, []child{c})
		}
	}
	var orphans []child
	for _, c := range children {
		if c.Kind == "season" {
			continue
		}
		if i, ok := idx[c.ParentID]; ok {
			groups[i] = append(groups[i], c)
		} else {
			orphans = append(orphans, c)
		}
	}
	if len(orphans) > 0 {
		groups = append(groups, orphans)
	}
	return groups
}

// child is a season or an episode of a show.
type child struct {
	ID       string
	Kind     string
	ParentID string
	Season   int // -1 when unknown
	Episode  int // -1 when unknown
	AirDate  string
	Locked   []string
}

func (s *Store) children(ctx context.Context, showID string) ([]child, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.id, i.kind, i.parent_id, COALESCE(i.season_number, -1),
		COALESCE(i.episode_number, -1), COALESCE(i.air_date, ''), COALESCE(m.locked_fields, '')
		FROM items i LEFT JOIN items s ON s.id = i.parent_id LEFT JOIN item_metadata m ON m.item_id = i.id
		WHERE i.parent_id = ? OR s.parent_id = ? ORDER BY i.kind DESC, i.season_number, i.episode_number`, showID, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []child
	for rows.Next() {
		var (
			c      child
			locked string
		)
		if err := rows.Scan(&c.ID, &c.Kind, &c.ParentID, &c.Season, &c.Episode, &c.AirDate, &locked); err != nil {
			return nil, err
		}
		if locked != "" {
			_ = json.Unmarshal([]byte(locked), &c.Locked)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// fetchSeasons gets the seasons the show has on disk (R6), plus the ones a
// date-based episode needs: the seasons that aired in its year first, then
// the others until it's found.
func (s *Service) fetchSeasons(ctx context.Context, prov provider.Provider, d *provider.Details, children []child, loc provider.Locale) (map[int]*provider.Season, error) {
	known := map[int]bool{}
	for _, sn := range d.Seasons {
		known[sn.Number] = true
	}
	out := map[int]*provider.Season{}
	get := func(n int) (*provider.Season, error) {
		if sn, ok := out[n]; ok {
			return sn, nil
		}
		sn, err := prov.Season(ctx, d.TMDBID, n, loc)
		if err != nil {
			return nil, err
		}
		out[n] = sn
		return sn, nil
	}
	seasonOf := map[string]int{}
	for _, c := range children {
		if c.Kind == "season" {
			seasonOf[c.ID] = c.Season
			if known[c.Season] {
				if _, err := get(c.Season); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, c := range children {
		if c.Kind != "episode" || c.Episode >= 0 || c.AirDate == "" {
			continue
		}
		if sn, ok := out[seasonOf[c.ParentID]]; ok && episodeByDate(sn, c.AirDate) != nil {
			continue
		}
		year := c.AirDate[:4]
		order := slices.Clone(d.Seasons)
		slices.SortStableFunc(order, func(a, b provider.SeasonRef) int {
			ra, rb := !strings.HasPrefix(a.AirDate, year), !strings.HasPrefix(b.AirDate, year)
			switch {
			case ra == rb:
				return 0
			case !ra:
				return -1
			}
			return 1
		})
		for _, ref := range order {
			sn, err := get(ref.Number)
			if err != nil {
				return nil, err
			}
			if episodeByDate(sn, c.AirDate) != nil {
				break
			}
		}
	}
	return out, nil
}

func episodeByDate(sn *provider.Season, date string) *provider.Episode {
	for i := range sn.Episodes {
		if sn.Episodes[i].AirDate == date {
			return &sn.Episodes[i]
		}
	}
	return nil
}

func (s *Service) ensureCollection(ctx context.Context, prov provider.Provider, p parsed, ref *provider.CollectionRef) (string, error) {
	var id string
	err := s.store.db.QueryRowContext(ctx, `SELECT id FROM collections WHERE library_id = ? AND tmdb_id = ?`, p.LibraryID, ref.TMDBID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	col, err := prov.Collection(ctx, ref.TMDBID, p.Locale)
	if err != nil {
		return "", err
	}
	id = newID()
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO collections (id, library_id, tmdb_id, name, overview, updated_at)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (library_id, tmdb_id) DO NOTHING`,
		id, p.LibraryID, ref.TMDBID, col.Name, col.Overview, ts(s.o.Now()))
	if err != nil {
		return "", err
	}
	// Another worker may have inserted it first.
	if err := s.store.db.QueryRowContext(ctx, `SELECT id FROM collections WHERE library_id = ? AND tmdb_id = ?`, p.LibraryID, ref.TMDBID).Scan(&id); err != nil {
		return "", err
	}
	// Its poster and backdrop are fetched on first request (R23).
	o := owner{"collection", id}
	for kind, path := range map[string]string{"poster": col.PosterPath, "backdrop": col.BackdropPath} {
		if path == "" {
			continue
		}
		if err := s.store.syncArtwork(ctx, o, kind, []candidate{{Source: "tmdb", URL: path}}); err != nil {
			return "", err
		}
	}
	return id, nil
}

// build merges what's known about an item into the row to store, field by
// field: a locked value, else the .nfo, else the provider (R12, R29).
func build(p parsed, cur row, d *provider.Details, doc *nfo.NFO, res resolved, system string) row {
	r := row{
		State: match.Matched, MatchSource: res.source, Confidence: res.confidence, MatchLocked: cur.MatchLocked,
		TMDBID: d.TMDBID, IMDbID: d.IMDbID, TVDBID: d.TVDBID,
		Title: d.Title, OriginalTitle: d.OriginalTitle, Overview: d.Overview, Tagline: d.Tagline,
		ReleaseDate: d.ReleaseDate, EndDate: d.EndDate, ShowStatus: d.Status, Runtime: d.Runtime,
		Rating: d.Rating, RatingCount: d.RatingCount, Genres: d.Genres, Studios: d.Studios, Trailers: d.Trailers,
		FieldLanguages: map[string]string{}, LockedFields: cur.LockedFields,
	}
	for k, v := range d.FieldLanguages {
		r.FieldLanguages[k] = v
	}
	region := p.Locale.Region
	if c := d.Certifications[region]; c != "" {
		r.Certification, r.CertRegion = c, region
	} else if c := d.Certifications["US"]; c != "" {
		r.Certification, r.CertRegion = c, "US"
	}
	applyNFO(&r, doc, region)
	r.SortTitle = sortKey(r.Title)
	if doc != nil && doc.SortTitle != "" {
		r.SortTitle = sortKey(doc.SortTitle)
	}
	keepLocked(&r, cur)
	r.AgeRating = age(r.CertRegion, system, r.Certification)
	return r
}

// applyNFO lets .nfo fields win over the provider's, and records them.
func applyNFO(r *row, doc *nfo.NFO, region string) {
	if doc == nil {
		return
	}
	set := func(field string, dst *string, v string) {
		if v != "" {
			*dst = v
			r.LocalFields = append(r.LocalFields, field)
			delete(r.FieldLanguages, field)
		}
	}
	set("title", &r.Title, doc.Title)
	set("originalTitle", &r.OriginalTitle, doc.OriginalTitle)
	set("overview", &r.Overview, doc.Plot)
	set("tagline", &r.Tagline, doc.Tagline)
	set("releaseDate", &r.ReleaseDate, doc.Premiered)
	if doc.Runtime > 0 {
		r.Runtime = doc.Runtime
		r.LocalFields = append(r.LocalFields, "runtime")
	}
	if len(doc.Genres) > 0 {
		r.Genres = r.Genres[:0:0]
		for _, g := range doc.Genres {
			r.Genres = append(r.Genres, provider.Genre{Name: g})
		}
		r.LocalFields = append(r.LocalFields, "genres")
	}
	if len(doc.Studios) > 0 {
		r.Studios = doc.Studios
		r.LocalFields = append(r.LocalFields, "studios")
	}
	if doc.MPAA != "" {
		country, cert := certs.ParseNFO(doc.MPAA, region)
		r.Certification, r.CertRegion = cert, country
		r.LocalFields = append(r.LocalFields, "certification")
	}
}

// keepLocked restores the locked fields from the current row (R29).
func keepLocked(r *row, cur row) {
	for _, f := range cur.LockedFields {
		switch f {
		case "title":
			r.Title = cur.Title
		case "sortTitle":
			r.SortTitle = cur.SortTitle
		case "originalTitle":
			r.OriginalTitle = cur.OriginalTitle
		case "overview":
			r.Overview = cur.Overview
		case "tagline":
			r.Tagline = cur.Tagline
		case "releaseDate":
			r.ReleaseDate = cur.ReleaseDate
		case "genres":
			r.Genres = cur.Genres
		case "certification":
			r.Certification, r.CertRegion = cur.Certification, cur.CertRegion
		}
		delete(r.FieldLanguages, f)
	}
}

func age(region, system, cert string) *int {
	if cert == "" {
		return nil
	}
	if a, ok := certs.Age(region, system, cert); ok {
		return &a
	}
	return nil
}

// sortKey is the sort title: normalized, without a leading article (F004
// lists sort on it too).
func sortKey(title string) string { return match.Normalize(title) }

// refreshAt schedules the next refresh (R31).
func refreshAt(now time.Time, d *provider.Details) time.Time {
	if d.Kind == provider.Show {
		if d.Status == "returning" || d.Status == "in_production" {
			return now.Add(3 * 24 * time.Hour)
		}
		return now.Add(90 * 24 * time.Hour)
	}
	if released, err := time.Parse("2006-01-02", d.ReleaseDate); err == nil && now.Sub(released) < 90*24*time.Hour {
		return now.Add(7 * 24 * time.Hour)
	}
	return now.Add(90 * 24 * time.Hour)
}

// Retry schedule for items that didn't match (not in the spec: a title
// TMDB doesn't know today may be added later).
const unmatchedRetry = 7 * 24 * time.Hour

// writeUnresolved stores a needs_review or unmatched decision, keeping the
// .nfo fields so the item still shows what's known locally (R5).
func (s *Store) writeUnresolved(ctx context.Context, p parsed, cur row, doc *nfo.NFO, d *match.Decision) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		r := cur
		r.State = d.State
		r.MatchSource, r.Confidence, r.TMDBID = "", 0, 0
		r.LocalFields = nil
		applyNFO(&r, doc, p.Locale.Region)
		if doc != nil && doc.SortTitle != "" {
			r.SortTitle = sortKey(doc.SortTitle)
		} else if r.Title != "" {
			r.SortTitle = sortKey(r.Title)
		}
		keepLocked(&r, cur)
		system := certs.Movie
		if p.Kind == "show" {
			system = certs.TV
		}
		r.AgeRating = age(r.CertRegion, system, r.Certification)
		var refresh time.Time
		if d.State == match.Unmatched {
			refresh = s.now().Add(unmatchedRetry)
		}
		if err := s.writeRow(ctx, tx, p.ID, r, refresh); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM match_candidates WHERE item_id = ?`, p.ID); err != nil {
			return err
		}
		for i, c := range d.Candidates {
			if _, err := tx.ExecContext(ctx, `INSERT INTO match_candidates (item_id, rank, tmdb_id, title, original_title,
				year, overview, poster_path, confidence) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				p.ID, i, c.TMDBID, c.Title, nullString(c.OriginalTitle), nullInt(c.Year), nullString(c.Overview),
				nullString(c.PosterPath), c.Confidence); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeLocal stores .nfo fields while no provider is available; the item
// stays pending (R34).
func (s *Store) writeLocal(ctx context.Context, p parsed, cur row, doc *nfo.NFO) error {
	if doc == nil {
		return nil
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		r := cur
		r.State = "pending"
		r.LocalFields = nil
		applyNFO(&r, doc, p.Locale.Region)
		if r.Title != "" {
			r.SortTitle = sortKey(r.Title)
		}
		keepLocked(&r, cur)
		system := certs.Movie
		if p.Kind == "show" {
			system = certs.TV
		}
		r.AgeRating = age(r.CertRegion, system, r.Certification)
		if p.Kind == "movie" && doc.Kind != nfo.URLOnly {
			r.MatchSource = sourceNFO
		}
		return s.writeRow(ctx, tx, p.ID, r, time.Time{})
	})
}

// writeChildren stores a matched show's seasons and episodes (R6). They
// take the show's age rating (R24) and keep their own locked fields.
func (s *Store) writeChildren(ctx context.Context, tx *sql.Tx, show row, children []child, seasons map[int]*provider.Season) error {
	seasonOf := map[string]int{}
	for _, c := range children {
		if c.Kind == "season" {
			seasonOf[c.ID] = c.Season
		}
	}
	for _, c := range children {
		cur, err := s.loadRow(ctx, tx, c.ID)
		if err != nil {
			return err
		}
		r := row{
			State: match.Unmatched, MatchSource: sourceParent, Confidence: 1, TMDBID: show.TMDBID,
			Certification: show.Certification, CertRegion: show.CertRegion, AgeRating: show.AgeRating,
			FieldLanguages: map[string]string{}, LockedFields: c.Locked,
		}
		switch c.Kind {
		case "season":
			if sn := seasons[c.Season]; sn != nil {
				r.State = match.Matched
				r.Title, r.Overview, r.ReleaseDate = sn.Name, sn.Overview, sn.AirDate
			}
		case "episode":
			var ep *provider.Episode
			if sn := seasons[seasonOf[c.ParentID]]; sn != nil && c.Episode >= 0 {
				for i := range sn.Episodes {
					if sn.Episodes[i].Number == c.Episode {
						ep = &sn.Episodes[i]
					}
				}
			}
			if ep == nil && c.AirDate != "" {
				for _, sn := range seasons {
					if ep = episodeByDate(sn, c.AirDate); ep != nil {
						break
					}
				}
			}
			if ep != nil {
				r.State = match.Matched
				r.Title, r.Overview, r.ReleaseDate = ep.Name, ep.Overview, ep.AirDate
				r.Runtime, r.Rating = ep.Runtime, ep.Rating
				for k, v := range ep.FieldLanguages {
					if k == "name" {
						k = "title"
					}
					r.FieldLanguages[k] = v
				}
			}
		}
		r.SortTitle = sortKey(r.Title)
		keepLocked(&r, cur)
		if err := s.writeRow(ctx, tx, c.ID, r, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

// writeRow upserts an item_metadata row. A zero refresh means none.
func (s *Store) writeRow(ctx context.Context, tx *sql.Tx, itemID string, r row, refresh time.Time) error {
	j := func(v any, empty bool) any {
		if empty {
			return nil
		}
		b, _ := json.Marshal(v)
		return string(b)
	}
	var refreshAt any
	if !refresh.IsZero() {
		refreshAt = ts(refresh)
	}
	var ageRating any
	if r.AgeRating != nil {
		ageRating = *r.AgeRating
	}
	now := ts(s.now())
	fetched := any(nil)
	if r.State == match.Matched {
		fetched = now
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO item_metadata (item_id, state, match_source, confidence, match_locked,
		tmdb_id, imdb_id, tvdb_id, title, original_title, sort_title, overview, tagline, release_date, end_date,
		show_status, runtime_minutes, rating, rating_count, certification, certification_region, age_rating, genres,
		studios, trailers, collection_id, duplicate_of, field_languages, locked_fields, local_fields, attempts,
		next_attempt_at, refresh_at, last_error, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NULL, ?, NULL, ?, ?)
		ON CONFLICT (item_id) DO UPDATE SET state = excluded.state, match_source = excluded.match_source,
		confidence = excluded.confidence, match_locked = excluded.match_locked, tmdb_id = excluded.tmdb_id,
		imdb_id = excluded.imdb_id, tvdb_id = excluded.tvdb_id, title = excluded.title,
		original_title = excluded.original_title, sort_title = excluded.sort_title, overview = excluded.overview,
		tagline = excluded.tagline, release_date = excluded.release_date, end_date = excluded.end_date,
		show_status = excluded.show_status, runtime_minutes = excluded.runtime_minutes, rating = excluded.rating,
		rating_count = excluded.rating_count, certification = excluded.certification,
		certification_region = excluded.certification_region, age_rating = excluded.age_rating, genres = excluded.genres,
		studios = excluded.studios, trailers = excluded.trailers, collection_id = excluded.collection_id,
		field_languages = excluded.field_languages, locked_fields = excluded.locked_fields,
		local_fields = excluded.local_fields, attempts = 0, next_attempt_at = NULL, refresh_at = excluded.refresh_at,
		last_error = NULL, fetched_at = COALESCE(excluded.fetched_at, item_metadata.fetched_at),
		updated_at = excluded.updated_at`,
		itemID, r.State, nullString(r.MatchSource), nullFloat(r.Confidence, r.MatchSource == ""), r.MatchLocked,
		nullInt(r.TMDBID), nullString(r.IMDbID), nullInt(r.TVDBID), nullString(r.Title), nullString(r.OriginalTitle),
		nullString(r.SortTitle), nullString(r.Overview), nullString(r.Tagline), nullString(r.ReleaseDate),
		nullString(r.EndDate), nullString(r.ShowStatus), nullInt(r.Runtime), nullFloat(r.Rating, r.RatingCount == 0 && r.Rating == 0),
		nullInt(r.RatingCount), nullString(r.Certification), nullString(r.CertRegion), ageRating,
		j(r.Genres, len(r.Genres) == 0), j(r.Studios, len(r.Studios) == 0), j(r.Trailers, len(r.Trailers) == 0),
		nullString(r.CollectionID), nullString(r.DuplicateOf), j(r.FieldLanguages, len(r.FieldLanguages) == 0),
		j(r.LockedFields, len(r.LockedFields) == 0), j(r.LocalFields, len(r.LocalFields) == 0),
		refreshAt, fetched, now)
	return err
}

// writeCredits replaces an item's cast and crew (R8). People are shared,
// keyed by their TMDB id.
func (s *Store) writeCredits(ctx context.Context, tx *sql.Tx, itemID string, cast, crew []provider.Person) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_credits WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	now := ts(s.now())
	for _, p := range append(slices.Clone(cast), crew...) {
		if p.TMDBID == 0 || p.Name == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO people (id, tmdb_id, name, updated_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (tmdb_id) DO UPDATE SET name = excluded.name, updated_at = excluded.updated_at`,
			newID(), p.TMDBID, p.Name, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO item_credits (item_id, person_id, role, character, ord)
			SELECT ?, id, ?, ?, ? FROM people WHERE tmdb_id = ? ON CONFLICT DO NOTHING`,
			itemID, p.Role, nullString(p.Character), p.Order, p.TMDBID); err != nil {
			return err
		}
	}
	return nil
}

// markDuplicates points every newer item of the library matched to the same
// TMDB id at the oldest one (R38).
func (s *Store) markDuplicates(ctx context.Context, tx *sql.Tx, p parsed, tmdbID int) error {
	if tmdbID == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.id FROM item_metadata m JOIN items i ON i.id = m.item_id
		WHERE i.library_id = ? AND i.kind = ? AND m.tmdb_id = ? AND m.state = 'matched'
		ORDER BY i.created_at, i.id`, p.LibraryID, p.Kind, tmdbID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for i, id := range ids {
		var dup any
		if i > 0 {
			dup = ids[0]
		}
		if _, err := tx.ExecContext(ctx, `UPDATE item_metadata SET duplicate_of = ? WHERE item_id = ?`, dup, id); err != nil {
			return err
		}
	}
	return nil
}

const refreshShowsQuery = `UPDATE item_metadata SET refresh_at = ? WHERE state <> 'pending' AND item_id IN (
	SELECT CASE i.kind WHEN 'season' THEN i.parent_id ELSE p.parent_id END
	FROM items i LEFT JOIN items p ON p.id = i.parent_id
	WHERE i.kind IN ('season', 'episode') AND i.id IN (`

// refreshShowsOf schedules an immediate refresh of the matched shows that
// new seasons or episodes belong to.
func (s *Store) refreshShowsOf(ctx context.Context, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return nil
	}
	args := []any{ts(s.now())}
	for _, id := range itemIDs {
		args = append(args, id)
	}
	q := refreshShowsQuery + placeholders(len(itemIDs)) + "))" //nolint:gosec // only placeholders
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}

func (s *Store) inTx(ctx context.Context, f func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	start := time.Now()
	defer func() {
		if held := time.Since(start); held > time.Second {
			slog.Warn("metadata held the database write lock", "held_ms", held.Milliseconds())
		}
	}()
	if err := f(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func newID() string { return uuid.Must(uuid.NewV7()).String() }

func itoa(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
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

func nullFloat(f float64, null bool) any {
	if null {
		return nil
	}
	return f
}
