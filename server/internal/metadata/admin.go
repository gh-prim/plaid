package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/metadata/certs"
	"github.com/gh-prim/plaid/server/internal/metadata/fanart"
	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/match"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdb"
	"github.com/gh-prim/plaid/server/internal/settings"
)

// Errors the admin routes map to API error codes (F005).
var (
	ErrDisabled          = errors.New("metadata disabled: no TMDB key")
	ErrMatchNotFound     = errors.New("no such TMDB title")
	ErrKindMismatch      = errors.New("that kind of item can't take this match")
	ErrAlreadyIdentified = errors.New("file already identified")
	ErrFieldNotEditable  = errors.New("field not editable")
	ErrInvalidAPIKey     = errors.New("the provider rejected the key")
	ErrBadImagePath      = errors.New("not a TMDB image file name")
)

// --- Search (R27) ----------------------------------------------------------

var idQuery = regexp.MustCompile(`^(tmdb|imdb):\s*(\S+)$`)

// Search looks TMDB up by free text or by id, scoring candidates against
// an item's parsed title and year when itemID is given.
func (s *Service) Search(ctx context.Context, kind, query string, year int, itemID string) ([]match.Scored, error) {
	prov := s.provider()
	if prov == nil {
		return nil, ErrDisabled
	}
	pkind := provider.Movie
	if kind == "show" {
		pkind = provider.Show
	}
	title, ty := query, year
	if itemID != "" {
		p, err := s.store.loadParsed(ctx, itemID)
		if err != nil {
			return nil, err
		}
		title, ty = p.Title, p.Year
	}
	var cands []provider.Candidate
	if m := idQuery.FindStringSubmatch(strings.TrimSpace(query)); m != nil {
		id, err := s.resolveID(ctx, prov, pkind, m[1], m[2])
		if err != nil {
			return nil, err
		}
		if id > 0 {
			c, err := s.candidateByID(ctx, prov, pkind, id)
			if err != nil {
				return nil, err
			}
			cands = append(cands, c)
		}
	} else {
		var err error
		if cands, err = prov.Search(ctx, pkind, query, year, provider.Locale{Language: "en", Region: "US"}); err != nil {
			return nil, err
		}
	}
	out := make([]match.Scored, 0, len(cands))
	for _, c := range cands {
		sc := match.Scored{Candidate: c}
		if title != "" {
			sc.Confidence = match.Score(title, ty, c)
		}
		out = append(out, sc)
	}
	if title != "" {
		slices.SortStableFunc(out, func(a, b match.Scored) int {
			switch {
			case a.Confidence > b.Confidence:
				return -1
			case a.Confidence < b.Confidence:
				return 1
			}
			return 0
		})
	}
	return out, nil
}

func (s *Service) resolveID(ctx context.Context, prov provider.Provider, kind, scheme, value string) (int, error) {
	if scheme == "tmdb" {
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return 0, nil
		}
		return n, nil
	}
	return prov.FindIMDb(ctx, kind, value)
}

func (s *Service) candidateByID(ctx context.Context, prov provider.Provider, kind string, id int) (provider.Candidate, error) {
	loc := provider.Locale{Language: "en", Region: "US"}
	var (
		d   *provider.Details
		err error
	)
	if kind == provider.Show {
		d, err = prov.Show(ctx, id, loc)
	} else {
		d, err = prov.Movie(ctx, id, loc)
	}
	if errors.Is(err, httpx.ErrNotFound) {
		return provider.Candidate{}, ErrMatchNotFound
	}
	if err != nil {
		return provider.Candidate{}, err
	}
	c := provider.Candidate{TMDBID: d.TMDBID, Title: d.Title, OriginalTitle: d.OriginalTitle, Overview: d.Overview}
	if len(d.ReleaseDate) >= 4 {
		c.Year, _ = strconv.Atoi(d.ReleaseDate[:4])
	}
	if len(d.Images.Posters) > 0 {
		c.PosterPath = d.Images.Posters[0].Path
	}
	return c, nil
}

// candidateImage is a cached candidate poster.
type candidateImage struct {
	body    []byte
	expires time.Time
}

var (
	imageName       = regexp.MustCompile(`^[A-Za-z0-9]+\.(jpg|png)$`)
	candidateMu     sync.Mutex
	candidateImages = map[string]candidateImage{}
)

const candidateImageTTL = time.Hour

// CandidateImage returns a TMDB w185 poster for the review screens, so
// clients never call TMDB themselves (R27). Only TMDB file names pass.
func (s *Service) CandidateImage(ctx context.Context, name string) ([]byte, error) {
	if !imageName.MatchString(name) {
		return nil, ErrBadImagePath
	}
	if s.art == nil {
		return nil, ErrDisabled
	}
	now := s.o.Now()
	candidateMu.Lock()
	if c, ok := candidateImages[name]; ok && now.Before(c.expires) {
		candidateMu.Unlock()
		return c.body, nil
	}
	candidateMu.Unlock()
	body, err := s.art.Fetch(ctx, tmdbImages+"w185/"+name)
	if err != nil {
		return nil, err
	}
	candidateMu.Lock()
	defer candidateMu.Unlock()
	for k, c := range candidateImages { // drop expired ones as we go
		if now.After(c.expires) {
			delete(candidateImages, k)
		}
	}
	candidateImages[name] = candidateImage{body: body, expires: now.Add(candidateImageTTL)}
	return body, nil
}

// --- Fix match, unmatch, refresh (R26, R31, R39) ---------------------------

// Match applies a TMDB id to a movie or show and locks it, or moves an
// episode's files to another episode of its show. It runs the match now
// and returns the resulting item's id once it's stored.
func (s *Service) Match(ctx context.Context, itemID string, tmdbID int, season, episode *int) (string, error) {
	p, err := s.store.loadParsed(ctx, itemID)
	if err != nil {
		return "", err
	}
	target, err := s.match(ctx, p, tmdbID, season, episode)
	if target == "" {
		target = itemID
	}
	return target, err
}

func (s *Service) match(ctx context.Context, p parsed, tmdbID int, season, episode *int) (string, error) {
	itemID := p.ID
	switch p.Kind {
	case "movie", "show":
		if tmdbID <= 0 {
			return "", ErrKindMismatch
		}
		prov := s.provider()
		if prov == nil {
			return "", ErrDisabled
		}
		kind := provider.Movie
		if p.Kind == "show" {
			kind = provider.Show
		}
		if _, err := s.candidateByID(ctx, prov, kind, tmdbID); err != nil {
			return "", err
		}
		if err := s.store.lockMatch(ctx, itemID, tmdbID); err != nil {
			return "", err
		}
		if p.Kind == "movie" {
			return itemID, s.processMovie(ctx, p)
		}
		return itemID, s.processShow(ctx, p)
	case "episode":
		if season == nil || episode == nil || tmdbID != 0 {
			return "", ErrKindMismatch
		}
		showID, target, err := s.store.relinkEpisode(ctx, p, *season, *episode)
		if err != nil {
			return "", err
		}
		show, err := s.store.loadParsed(ctx, showID)
		if err != nil || s.provider() == nil {
			return target, err
		}
		return target, s.processShow(ctx, show)
	}
	return "", ErrKindMismatch
}

func (s *Store) lockMatch(ctx context.Context, itemID string, tmdbID int) error {
	now := ts(s.now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO item_metadata (item_id, state, match_source, confidence, match_locked, tmdb_id, updated_at)
		VALUES (?, 'matched', 'manual', 1, 1, ?, ?)
		ON CONFLICT (item_id) DO UPDATE SET state = 'matched', match_source = 'manual', confidence = 1, match_locked = 1,
		tmdb_id = excluded.tmdb_id, updated_at = excluded.updated_at`, itemID, tmdbID, now)
	return err
}

// relinkEpisode moves an episode's files to episode `number` of season
// `season` of the same show, creating the season and episode if needed
// (R39). The old episode is removed if no file is left on it.
func (s *Store) relinkEpisode(ctx context.Context, p parsed, season, number int) (showID, target string, err error) {
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `SELECT s.parent_id FROM items s WHERE s.id = ?`, p.ParentID).Scan(&showID); err != nil {
			return err
		}
		seasonID, err := ensureChild(ctx, tx, p.LibraryID, showID, "season", season, -1, s.now())
		if err != nil {
			return err
		}
		target, err = ensureChild(ctx, tx, p.LibraryID, seasonID, "episode", -1, number, s.now())
		if err != nil {
			return err
		}
		if target == p.ID {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_files (item_id, file_id) SELECT ?, file_id FROM item_files WHERE item_id = ?`,
			target, p.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, p.ID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM items WHERE id = ? AND NOT EXISTS (SELECT 1 FROM items c WHERE c.parent_id = items.id)`, p.ParentID)
		return err
	})
	return showID, target, err
}

// ensureChild finds or creates a season (number in season) or an episode
// (number in episode) under parent.
func ensureChild(ctx context.Context, tx *sql.Tx, libraryID, parentID, kind string, season, episode int, now time.Time) (string, error) {
	var id string
	q := `SELECT id FROM items WHERE parent_id = ? AND kind = ? AND season_number = ?`
	n := season
	if kind == "episode" {
		q, n = `SELECT id FROM items WHERE parent_id = ? AND kind = ? AND episode_number = ?`, episode
	}
	err := tx.QueryRowContext(ctx, q, parentID, kind, n).Scan(&id)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}
	id = newID()
	var sn, en any
	if season >= 0 {
		sn = season
	}
	if episode >= 0 {
		en = episode
	}
	t := ts(now)
	_, err = tx.ExecContext(ctx, `INSERT INTO items (id, library_id, kind, parent_id, season_number, episode_number, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, libraryID, kind, parentID, sn, en, t, t)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO item_metadata (item_id, state, next_attempt_at, updated_at) VALUES (?, 'pending', ?, ?)`, id, t, t)
	return id, err
}

// Unmatch forgets an item's match and lock; it's matched again
// automatically (R26).
func (s *Service) Unmatch(ctx context.Context, itemID string) error {
	if _, err := s.store.loadParsed(ctx, itemID); err != nil {
		return err
	}
	now := ts(s.o.Now())
	if _, err := s.store.db.ExecContext(ctx, `UPDATE item_metadata SET state = 'pending', match_source = NULL, confidence = NULL,
		match_locked = 0, tmdb_id = NULL, duplicate_of = NULL, attempts = 0, next_attempt_at = ?, refresh_at = NULL, updated_at = ?
		WHERE item_id = ?`, now, now, itemID); err != nil {
		return err
	}
	s.queue.Wake()
	return nil
}

// Refresh refetches an item (its movie or show) now; with images, its
// unlocked artwork is chosen and downloaded again (R31).
func (s *Service) Refresh(ctx context.Context, itemID string, images bool) error {
	top, err := s.store.topItem(ctx, itemID)
	if err != nil {
		return err
	}
	if err := s.store.refreshNow(ctx, []string{top}, images); err != nil {
		return err
	}
	s.queue.Wake()
	return nil
}

// RefreshLibrary refetches every movie and show of a library now.
func (s *Service) RefreshLibrary(ctx context.Context, libraryID string, images bool) error {
	rows, err := s.store.db.QueryContext(ctx, `SELECT id FROM items WHERE library_id = ? AND kind IN ('movie', 'show')`, libraryID)
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
	if err := s.store.refreshNow(ctx, ids, images); err != nil {
		return err
	}
	s.queue.Wake()
	return nil
}

func (s *Store) topItem(ctx context.Context, itemID string) (string, error) {
	var top string
	err := s.db.QueryRowContext(ctx, `WITH RECURSIVE up(id, parent, kind) AS (
			SELECT id, parent_id, kind FROM items WHERE id = ?
			UNION SELECT i.id, i.parent_id, i.kind FROM items i JOIN up ON i.id = up.parent)
		SELECT id FROM up WHERE kind IN ('movie', 'show')`, itemID).Scan(&top)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return top, err
}

func (s *Store) refreshNow(ctx context.Context, ids []string, images bool) error {
	now := ts(s.now())
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE item_metadata SET refresh_at = ?, next_attempt_at = CASE WHEN state = 'pending' THEN ? ELSE next_attempt_at END
				WHERE item_id = ?`, now, now, id); err != nil {
				return err
			}
			if images {
				// Forget the unlocked choices so they're picked and fetched anew.
				if _, err := tx.ExecContext(ctx, `DELETE FROM artwork WHERE owner_type = 'item' AND locked = 0 AND owner_id IN (
						WITH RECURSIVE d(id) AS (SELECT ? UNION ALL SELECT i.id FROM items i JOIN d ON i.parent_id = d.id) SELECT id FROM d)
					AND kind NOT IN (SELECT kind FROM artwork a WHERE a.owner_id = artwork.owner_id AND a.locked = 1)`, id); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// --- Identify (R28) ---------------------------------------------------------

// Identify says what an unrecognized file is: a movie, or episodes of a
// show. The identity is stored with the file, so rescans and moves keep it.
// It returns the movie or (first) episode item.
func (s *Service) Identify(ctx context.Context, fileID, userID string, tmdbID int, season *int, episodes []int) (string, error) {
	prov := s.provider()
	if prov == nil {
		return "", ErrDisabled
	}
	var (
		libraryID, libType, status string
		identified                 int
	)
	err := s.store.db.QueryRowContext(ctx, `SELECT f.library_id, l.type, f.parse_status,
		(SELECT COUNT(*) FROM file_identifications i WHERE i.file_id = f.id)
		FROM media_files f JOIN libraries l ON l.id = f.library_id WHERE f.id = ?`, fileID).Scan(&libraryID, &libType, &status, &identified)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if identified > 0 || status != "unrecognized" {
		return "", ErrAlreadyIdentified
	}
	kind := provider.Movie
	if libType == "shows" {
		kind = provider.Show
		if season == nil || len(episodes) == 0 {
			return "", ErrKindMismatch
		}
	} else if season != nil || len(episodes) > 0 {
		return "", ErrKindMismatch
	}
	c, err := s.candidateByID(ctx, prov, kind, tmdbID)
	if err != nil {
		return "", err
	}
	var top, result string
	now := s.o.Now()
	err = s.store.inTx(ctx, func(tx *sql.Tx) error {
		// The movie or show: an item already matched to this id, or a new one.
		k := "movie"
		if kind == provider.Show {
			k = "show"
		}
		err := tx.QueryRowContext(ctx, `SELECT i.id FROM items i JOIN item_metadata m ON m.item_id = i.id
			WHERE i.library_id = ? AND i.kind = ? AND m.tmdb_id = ? ORDER BY i.created_at LIMIT 1`, libraryID, k, tmdbID).Scan(&top)
		if errors.Is(err, sql.ErrNoRows) {
			top = newID()
			t := ts(now)
			if _, err := tx.ExecContext(ctx, `INSERT INTO items (id, library_id, kind, title, sort_title, year, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, top, libraryID, k, c.Title, sortKey(c.Title), nullInt(c.Year), t, t); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		targets := []string{top}
		if kind == provider.Show {
			seasonID, err := ensureChild(ctx, tx, libraryID, top, "season", *season, -1, now)
			if err != nil {
				return err
			}
			targets = targets[:0]
			for _, n := range episodes {
				ep, err := ensureChild(ctx, tx, libraryID, seasonID, "episode", -1, n, now)
				if err != nil {
					return err
				}
				targets = append(targets, ep)
			}
		}
		result = targets[0]
		for _, t := range targets {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_files (item_id, file_id) VALUES (?, ?)`, t, fileID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE media_files SET parse_status = 'ok', updated_at = ? WHERE id = ?`, ts(now), fileID); err != nil {
			return err
		}
		eps, _ := json.Marshal(episodes)
		var sn, en any
		if season != nil {
			sn, en = *season, string(eps)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO file_identifications (file_id, kind, tmdb_id, season_number, episode_numbers, created_by, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, fileID, map[string]string{provider.Movie: "movie", provider.Show: "episode"}[kind],
			tmdbID, sn, en, nullString(userID), ts(now))
		return err
	})
	if err != nil {
		return "", err
	}
	if err := s.store.lockMatch(ctx, top, tmdbID); err != nil {
		return "", err
	}
	p, err := s.store.loadParsed(ctx, top)
	if err != nil {
		return "", err
	}
	if kind == provider.Show {
		err = s.processShow(ctx, p)
	} else {
		err = s.processMovie(ctx, p)
	}
	return result, err
}

// --- Field edits (R29) ------------------------------------------------------

// Patch is an edit of an item's fields; nil fields are left alone.
type Patch struct {
	Title, SortTitle, OriginalTitle, Overview, Tagline, ReleaseDate, Certification *string
	Genres                                                                         *[]string
	Unlock                                                                         []string
}

var lockable = []string{"title", "sortTitle", "originalTitle", "overview", "tagline", "releaseDate", "genres", "certification"}

// PatchMetadata sets fields and locks them; Unlock gives fields back to the
// provider at the next refresh (R29).
func (s *Service) PatchMetadata(ctx context.Context, itemID string, p Patch) error {
	for _, f := range p.Unlock {
		if !slices.Contains(lockable, f) {
			return ErrFieldNotEditable
		}
	}
	parsedItem, err := s.store.loadParsed(ctx, itemID)
	if err != nil {
		return err
	}
	err = s.store.inTx(ctx, func(tx *sql.Tx) error {
		cur, err := s.store.loadRow(ctx, tx, itemID)
		if err != nil {
			return err
		}
		r := cur
		lock := func(field string) {
			if !slices.Contains(r.LockedFields, field) {
				r.LockedFields = append(r.LockedFields, field)
			}
			delete(r.FieldLanguages, field)
		}
		set := func(field string, dst *string, v *string) {
			if v != nil {
				*dst = strings.TrimSpace(*v)
				lock(field)
			}
		}
		set("title", &r.Title, p.Title)
		set("originalTitle", &r.OriginalTitle, p.OriginalTitle)
		set("overview", &r.Overview, p.Overview)
		set("tagline", &r.Tagline, p.Tagline)
		set("releaseDate", &r.ReleaseDate, p.ReleaseDate)
		if p.SortTitle != nil {
			r.SortTitle = sortKey(*p.SortTitle)
			lock("sortTitle")
		} else if p.Title != nil && !slices.Contains(r.LockedFields, "sortTitle") {
			r.SortTitle = sortKey(r.Title)
		}
		if p.Genres != nil {
			r.Genres = nil
			for _, g := range *p.Genres {
				if g = strings.TrimSpace(g); g != "" {
					r.Genres = append(r.Genres, provider.Genre{Name: g})
				}
			}
			lock("genres")
		}
		if p.Certification != nil {
			r.Certification, r.CertRegion = strings.TrimSpace(*p.Certification), parsedItem.Locale.Region
			system := certs.Movie
			if parsedItem.Kind != "movie" {
				system = certs.TV
			}
			r.AgeRating = age(r.CertRegion, system, r.Certification)
			lock("certification")
		}
		r.LockedFields = slices.DeleteFunc(r.LockedFields, func(f string) bool { return slices.Contains(p.Unlock, f) })
		var refresh time.Time
		if len(p.Unlock) > 0 {
			refresh = s.o.Now() // give the fields back at once
		}
		return s.store.writeRow(ctx, tx, itemID, r, refresh)
	})
	if err == nil && len(p.Unlock) > 0 {
		s.queue.Wake()
	}
	return err
}

// --- Artwork choice (R30) ---------------------------------------------------

// ArtworkCandidate is one image an admin can pick.
type ArtworkCandidate struct {
	ID, Kind, Source, Language string
	Width, Height              int
	Votes                      float64
	Selected, Locked           bool
}

// ArtworkCandidates lists an item's images of a kind, selected first.
func (s *Service) ArtworkCandidates(ctx context.Context, itemID, kind string) ([]ArtworkCandidate, error) {
	if _, err := s.store.loadParsed(ctx, itemID); err != nil {
		return nil, err
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT id, kind, source, COALESCE(language, ''), COALESCE(width, 0), COALESCE(height, 0),
		COALESCE(votes, 0), selected, locked FROM artwork WHERE owner_type = 'item' AND owner_id = ? AND kind = ?
		ORDER BY selected DESC, rowid`, itemID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ArtworkCandidate{}
	for rows.Next() {
		var c ArtworkCandidate
		if err := rows.Scan(&c.ID, &c.Kind, &c.Source, &c.Language, &c.Width, &c.Height, &c.Votes, &c.Selected, &c.Locked); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SelectArtwork picks one image of a kind and locks the kind (R30).
func (s *Service) SelectArtwork(ctx context.Context, itemID, kind, candidateID string) error {
	res, err := s.store.db.ExecContext(ctx, `UPDATE artwork SET selected = (id = ?), locked = 1
		WHERE owner_type = 'item' AND owner_id = ? AND kind = ? AND EXISTS (
			SELECT 1 FROM artwork a WHERE a.id = ? AND a.owner_id = ? AND a.kind = ?)`,
		candidateID, itemID, kind, candidateID, itemID, kind)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrImageNotFound
	}
	if s.art != nil {
		if _, err := s.storeImage(ctx, candidateID); err != nil {
			s.log.Warn("picked artwork not fetched yet", "artwork", candidateID, "err", err)
		}
	}
	return nil
}

// UnlockArtwork gives a kind back to the automatic choice and refreshes the
// item (R30).
func (s *Service) UnlockArtwork(ctx context.Context, itemID, kind string) error {
	if _, err := s.store.db.ExecContext(ctx, `UPDATE artwork SET locked = 0 WHERE owner_type = 'item' AND owner_id = ? AND kind = ?`,
		itemID, kind); err != nil {
		return err
	}
	return s.Refresh(ctx, itemID, false)
}

// --- Settings, status, attribution (R34, R35, R37) --------------------------

// KeySettings say which key each provider uses, never the keys.
type KeySettings struct {
	TMDBSource, FanartSource string
	FanartPersonalKeySet     bool
}

// Settings returns where each provider's key comes from.
func (s *Service) Settings(ctx context.Context) (KeySettings, error) {
	var out KeySettings
	var err error
	if _, out.TMDBSource, err = resolveKey(ctx, s.o.Settings, settings.TMDBAPIKey, s.o.TMDBKey, builtinTMDBKey); err != nil {
		return out, err
	}
	if _, out.FanartSource, err = resolveKey(ctx, s.o.Settings, settings.FanartAPIKey, s.o.FanartKey, builtinFanartKey); err != nil {
		return out, err
	}
	if s.o.Settings != nil {
		_, out.FanartPersonalKeySet, err = s.o.Settings.Get(ctx, settings.FanartPersonalKey)
	}
	return out, err
}

// KeyChange sets (non-empty), clears (empty) or leaves (nil) a key.
type KeyChange struct {
	TMDB, Fanart, FanartPersonal *string
}

// UpdateKeys checks new keys against their provider, saves them and
// reloads the clients (R34). A rejected key stores nothing.
func (s *Service) UpdateKeys(ctx context.Context, ch KeyChange) error {
	if s.o.Settings == nil {
		return ErrDisabled
	}
	check := s.o.CheckKey
	if check == nil {
		check = checkKey
	}
	if ch.TMDB != nil && *ch.TMDB != "" {
		if err := check(ctx, "tmdb", *ch.TMDB); err != nil {
			return err
		}
	}
	if ch.Fanart != nil && *ch.Fanart != "" {
		if err := check(ctx, "fanart", *ch.Fanart); err != nil {
			return err
		}
	}
	for _, c := range []struct {
		name string
		v    *string
	}{{settings.TMDBAPIKey, ch.TMDB}, {settings.FanartAPIKey, ch.Fanart}, {settings.FanartPersonalKey, ch.FanartPersonal}} {
		var err error
		switch {
		case c.v == nil:
			continue
		case *c.v == "":
			err = s.o.Settings.Delete(ctx, c.name)
		default:
			err = s.o.Settings.Set(ctx, c.name, *c.v)
		}
		if err != nil {
			return err
		}
	}
	if err := s.ReloadKeys(ctx); err != nil {
		return err
	}
	s.queue.Wake()
	return nil
}

// checkKey asks the provider whether a key works: TMDB's /configuration,
// fanart.tv's images of The Matrix.
func checkKey(ctx context.Context, name, key string) error {
	h := httpx.New(httpx.Options{Name: name + "-check", Hosts: append(slices.Clone(tmdb.Hosts), fanart.Hosts...)})
	return checkKeyWith(ctx, h, tmdb.BaseURL, fanart.BaseURL, name, key)
}

func checkKeyWith(ctx context.Context, h *httpx.Client, tmdbBase, fanartBase, name, key string) error {
	u := tmdbBase + "/configuration?api_key=" + url.QueryEscape(key)
	if name == "fanart" {
		u = fanartBase + "/movies/603?api_key=" + url.QueryEscape(key)
	}
	_, err := h.Get(ctx, u, nil)
	switch {
	case errors.Is(err, httpx.ErrInvalidKey):
		return ErrInvalidAPIKey
	case errors.Is(err, httpx.ErrNotFound):
		return nil // the key works; fanart.tv just has no image
	}
	return err
}

// ProviderState is one provider's health (R35).
type ProviderState struct {
	Provider, State, KeySource, LastError string
	LastErrorAt                           time.Time
}

// Status returns the providers' health.
func (s *Service) Status(ctx context.Context) ([]ProviderState, error) {
	keys, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	tmdbHTTP, fan, fanHTTP := s.http, s.fan, s.fanHTTP
	s.mu.Unlock()
	state := func(name, source string, h *httpx.Client, present bool) ProviderState {
		ps := ProviderState{Provider: name, KeySource: source, State: "no_api_key"}
		if source == KeyNone {
			ps.KeySource = "none"
		}
		if s.o.Provider != nil && name == "tmdb" {
			ps.State = httpx.StateOK
		}
		if h != nil && present {
			st := h.Status()
			ps.State, ps.LastError, ps.LastErrorAt = st.State, st.LastError, st.LastErrorAt
		}
		return ps
	}
	return []ProviderState{
		state("tmdb", keys.TMDBSource, tmdbHTTP, s.provider() != nil),
		state("fanart", keys.FanartSource, fanHTTP, fan != nil),
	}, nil
}

// Attribution is a notice a provider's terms ask for (R37).
type Attribution struct {
	Provider, Name, URL, Notice string
}

// Attributions lists the notices of the providers in use.
func (s *Service) Attributions() []Attribution {
	var out []Attribution
	if s.provider() != nil {
		out = append(out, Attribution{Provider: "tmdb", Name: "TMDB", URL: "https://www.themoviedb.org",
			Notice: "This product uses the TMDB API but is not endorsed or certified by TMDB."})
	}
	if s.fanart() != nil {
		out = append(out, Attribution{Provider: "fanart", Name: "fanart.tv", URL: "https://fanart.tv",
			Notice: "Logos, clearart and some backdrops are provided by fanart.tv."})
	}
	return out
}

// --- Review, collections, people (R5, R8, R23, R38) -------------------------

// ReviewEntry is an item an admin should look at.
type ReviewEntry struct {
	ItemID, Reason, DuplicateOf string
	Candidates                  []match.Scored
}

// Review lists a library's movies and shows that need review, weren't
// found, or duplicate another item, by item id after the cursor.
func (s *Store) Review(ctx context.Context, libraryID, after string, limit int) ([]ReviewEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.id, CASE WHEN m.duplicate_of IS NOT NULL THEN 'duplicate' ELSE m.state END,
		COALESCE(m.duplicate_of, '') FROM items i JOIN item_metadata m ON m.item_id = i.id
		WHERE i.library_id = ? AND i.kind IN ('movie', 'show') AND i.hidden = 0 AND i.id > ?
		AND (m.state IN ('needs_review', 'unmatched') OR m.duplicate_of IS NOT NULL)
		ORDER BY i.id LIMIT ?`, libraryID, after, limit)
	if err != nil {
		return nil, err
	}
	var out []ReviewEntry
	for rows.Next() {
		var e ReviewEntry
		if err := rows.Scan(&e.ItemID, &e.Reason, &e.DuplicateOf); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		cr, err := s.db.QueryContext(ctx, `SELECT tmdb_id, title, COALESCE(original_title, ''), COALESCE(year, 0), COALESCE(overview, ''),
			COALESCE(poster_path, ''), confidence FROM match_candidates WHERE item_id = ? ORDER BY rank`, out[i].ItemID)
		if err != nil {
			return nil, err
		}
		out[i].Candidates = []match.Scored{}
		for cr.Next() {
			var c match.Scored
			if err := cr.Scan(&c.TMDBID, &c.Title, &c.OriginalTitle, &c.Year, &c.Overview, &c.PosterPath, &c.Confidence); err != nil {
				_ = cr.Close()
				return nil, err
			}
			out[i].Candidates = append(out[i].Candidates, c)
		}
		_ = cr.Close()
	}
	return out, nil
}

// Collection is a collection with the movies the profile may see.
type Collection struct {
	ID, Name, Overview string
	Poster, Backdrop   *Art
	ItemIDs            []string // by release date
}

// Collections lists a library's collections that have at least one movie
// the profile may see.
func (s *Store) Collections(ctx context.Context, libraryID string, limit access.Limit) ([]Collection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM collections WHERE library_id = ? ORDER BY name COLLATE NOCASE`, libraryID)
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
	out := []Collection{}
	for _, id := range ids {
		c, err := s.Collection(ctx, id, limit)
		if err != nil {
			return nil, err
		}
		if len(c.ItemIDs) > 0 {
			out = append(out, c)
		}
	}
	return out, nil
}

// Collection returns one collection; ErrNotFound when it doesn't exist.
func (s *Store) Collection(ctx context.Context, id string, limit access.Limit) (Collection, error) {
	c := Collection{ID: id}
	err := s.db.QueryRowContext(ctx, `SELECT name, COALESCE(overview, '') FROM collections WHERE id = ?`, id).Scan(&c.Name, &c.Overview)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, err
	}
	q := `SELECT i.id FROM item_metadata m JOIN items i ON i.id = m.item_id WHERE m.collection_id = ? AND i.hidden = 0`
	args := []any{id}
	if cond, a := limit.SQL("m"); cond != "" {
		q += ` AND ` + cond
		args = append(args, a...)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY COALESCE(m.release_date, '9999'), i.id`, args...) //nolint:gosec // fixed fragments
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			return c, err
		}
		c.ItemIDs = append(c.ItemIDs, item)
	}
	art, err := s.selectedArt(ctx, "collection", []string{id})
	if err != nil {
		return c, err
	}
	c.Poster, c.Backdrop = first(art[id]["poster"]), first(art[id]["backdrop"])
	return c, rows.Err()
}

// PersonCredit is one of a person's credits.
type PersonCredit struct {
	ItemID, Role, Character string
}

// Person is a person and their credits the profile may see.
type Person struct {
	ID, Name string
	Portrait *Art
	Credits  []PersonCredit
}

// Person returns a person; ErrNotFound when they don't exist.
func (s *Store) Person(ctx context.Context, id string, limit access.Limit) (Person, error) {
	p := Person{ID: id, Credits: []PersonCredit{}}
	err := s.db.QueryRowContext(ctx, `SELECT name FROM people WHERE id = ?`, id).Scan(&p.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	q := `SELECT c.item_id, c.role, COALESCE(c.character, '') FROM item_credits c JOIN items i ON i.id = c.item_id
		LEFT JOIN item_metadata m ON m.item_id = i.id WHERE c.person_id = ? AND i.hidden = 0`
	args := []any{id}
	if cond, a := limit.SQL("m"); cond != "" {
		q += ` AND ` + cond
		args = append(args, a...)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY COALESCE(m.release_date, '') DESC, c.ord`, args...) //nolint:gosec // fixed fragments
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var c PersonCredit
		if err := rows.Scan(&c.ItemID, &c.Role, &c.Character); err != nil {
			return p, err
		}
		p.Credits = append(p.Credits, c)
	}
	art, err := s.selectedArt(ctx, "person", []string{id})
	if err != nil {
		return p, err
	}
	p.Portrait = first(art[id]["portrait"])
	return p, rows.Err()
}

// Portraits returns the selected portrait of each person, by id; people
// without one are absent.
func (s *Store) Portraits(ctx context.Context, ids []string) (map[string]Art, error) {
	art, err := s.selectedArt(ctx, "person", ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Art, len(art))
	for id, kinds := range art {
		if a := first(kinds["portrait"]); a != nil {
			out[id] = *a
		}
	}
	return out, nil
}

// --- Live counters (R35) ----------------------------------------------------

// countsEvery throttles metadata events per library.
const countsEvery = 2 * time.Second

type counterHub struct {
	mu   sync.Mutex
	subs map[string]map[chan Counters]struct{}
	last map[string]time.Time
	due  map[string]bool
}

// Subscribe returns a library's counters as they change, at most every
// 2 s; cancel stops it.
func (s *Service) Subscribe(libraryID string) (<-chan Counters, func()) {
	h := &s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs, h.last, h.due = map[string]map[chan Counters]struct{}{}, map[string]time.Time{}, map[string]bool{}
	}
	ch := make(chan Counters, 1)
	if h.subs[libraryID] == nil {
		h.subs[libraryID] = map[chan Counters]struct{}{}
	}
	h.subs[libraryID][ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[libraryID], ch)
	}
}

// changed publishes a library's counters, throttled; a change inside the
// window is sent at its end.
func (s *Service) changed(libraryID string) {
	h := &s.hub
	h.mu.Lock()
	if len(h.subs[libraryID]) == 0 || h.due[libraryID] {
		h.mu.Unlock()
		return
	}
	wait := countsEvery - s.o.Now().Sub(h.last[libraryID])
	h.due[libraryID] = true
	h.mu.Unlock()
	send := func() {
		counts, err := s.store.Counts(context.Background(), libraryID)
		h.mu.Lock()
		defer h.mu.Unlock()
		h.due[libraryID] = false
		h.last[libraryID] = s.o.Now()
		if err != nil {
			return
		}
		for ch := range h.subs[libraryID] {
			select {
			case <-ch: // drop a stale value
			default:
			}
			ch <- counts
		}
	}
	if wait <= 0 {
		send()
		return
	}
	time.AfterFunc(wait, send)
}
