package metadata

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata/artwork"
	"github.com/gh-prim/plaid/server/internal/metadata/fanart"
	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
	"github.com/gh-prim/plaid/server/internal/metadata/sidecar"
)

// TMDB image sizes downloaded (R17).
const tmdbImages = "https://image.tmdb.org/t/p/"

var tmdbSize = map[string]string{
	"poster": "w780", "backdrop": "w1280", "logo": "original", "still": "w780", "portrait": "h632", "thumb": "w1280",
}

// selectedCount is how many images of a kind are selected (R15).
func selectedCount(kind string) int {
	if kind == "backdrop" {
		return 3
	}
	return 1
}

// eager kinds are downloaded right after a match (R17); the others on
// first request.
var eager = map[string]bool{"poster": true, "backdrop": true, "logo": true, "clearart": true}

// candidate is an artwork option for one owner and kind, in order.
type candidate struct {
	Source    string // local, tmdb, fanart
	URL       string
	LocalPath string
	Language  string
	Width     int
	Height    int
	Votes     float64
}

// owner is what artwork belongs to.
type owner struct {
	Type string // item, person, collection
	ID   string
}

// rankRemote orders provider images for a kind (R15, R16): posters and
// logos prefer the library language, then English, then no text;
// backdrops prefer no text, then the library language. Votes break ties.
func rankRemote(kind, lang string, cands []candidate) []candidate {
	tier := func(c candidate) int {
		if kind == "backdrop" || kind == "thumb" {
			switch c.Language {
			case "":
				return 0
			case lang:
				return 1
			}
			return 2
		}
		switch c.Language {
		case lang:
			return 0
		case "en":
			return 1
		case "":
			return 2
		}
		return 3
	}
	out := slices.Clone(cands)
	sort.SliceStable(out, func(i, j int) bool {
		if ti, tj := tier(out[i]), tier(out[j]); ti != tj {
			return ti < tj
		}
		if out[i].Votes != out[j].Votes {
			return out[i].Votes > out[j].Votes
		}
		return out[i].Width > out[j].Width
	})
	// Images in another language than the library's or English are left out
	// of posters and logos: a Korean poster in a French library is worse
	// than none.
	if kind != "backdrop" && kind != "thumb" {
		out = slices.DeleteFunc(out, func(c candidate) bool { return tier(c) == 3 })
	}
	return out
}

func tmdbCandidates(imgs []provider.Image) []candidate {
	out := make([]candidate, 0, len(imgs))
	for _, i := range imgs {
		out = append(out, candidate{Source: "tmdb", URL: i.Path, Language: i.Language, Width: i.Width, Height: i.Height, Votes: i.Votes})
	}
	return out
}

func fanartCandidates(imgs []fanart.Image, kind string, season int) []candidate {
	var out []candidate
	for _, i := range imgs {
		if i.Kind == kind && i.Season == season {
			out = append(out, candidate{Source: "fanart", URL: i.URL, Language: i.Language, Votes: float64(i.Likes)})
		}
	}
	return out
}

// localImages finds the images next to an item (R13) in dirs: folder
// images, and images named after one of the item's videos.
func localImages(dirs, bases []string) map[string][]candidate {
	out := map[string][]candidate{}
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			c := sidecar.Classify(e.Name())
			if c.Kind == "" || c.Kind == sidecar.Nfo {
				continue
			}
			if c.Scope == sidecar.ScopeVideo && !slices.Contains(bases, c.Base) || c.Scope == sidecar.ScopeSeason {
				continue
			}
			out[c.Kind] = append(out[c.Kind], candidate{Source: "local", LocalPath: filepath.Join(d, e.Name())})
		}
	}
	for _, list := range out {
		sort.Slice(list, func(i, j int) bool { return list[i].LocalPath < list[j].LocalPath })
	}
	return out
}

// seasonLocal finds a season's local poster: seasonNN-poster in the show
// folder, or poster/folder/cover in the season folder.
func seasonLocal(showDir string, seasonDirs []string, number int) []candidate {
	var out []candidate
	if showDir != "" {
		if entries, err := os.ReadDir(showDir); err == nil {
			for _, e := range entries {
				if c := sidecar.Classify(e.Name()); c.Scope == sidecar.ScopeSeason && c.Season == number {
					out = append(out, candidate{Source: "local", LocalPath: filepath.Join(showDir, e.Name())})
				}
			}
		}
	}
	for _, d := range seasonDirs {
		if d != showDir {
			out = append(out, localImages([]string{d}, nil)["poster"]...)
		}
	}
	return out
}

// syncArtwork stores an owner's candidates for a kind and selects the top
// ones, unless an admin locked the kind (R15, R30).
func (s *Store) syncArtwork(ctx context.Context, o owner, kind string, cands []candidate) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var locked int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM artwork WHERE owner_type = ? AND owner_id = ? AND kind = ? AND locked = 1`,
			o.Type, o.ID, kind).Scan(&locked); err != nil {
			return err
		}
		if locked > 0 {
			return nil
		}
		// Keep what's downloaded: rows are matched on their URL or path.
		type known struct {
			id, sha, ext, colors string
			w, h                 int
		}
		existing := map[string]known{}
		rows, err := tx.QueryContext(ctx, `SELECT id, COALESCE(remote_url, local_path), COALESCE(sha256, ''), COALESCE(ext, ''),
			COALESCE(colors, ''), COALESCE(width, 0), COALESCE(height, 0) FROM artwork WHERE owner_type = ? AND owner_id = ? AND kind = ?`,
			o.Type, o.ID, kind)
		if err != nil {
			return err
		}
		for rows.Next() {
			var k known
			var ref string
			if err := rows.Scan(&k.id, &ref, &k.sha, &k.ext, &k.colors, &k.w, &k.h); err != nil {
				_ = rows.Close()
				return err
			}
			existing[ref] = k
		}
		_ = rows.Close()
		if _, err := tx.ExecContext(ctx, `DELETE FROM artwork WHERE owner_type = ? AND owner_id = ? AND kind = ?`, o.Type, o.ID, kind); err != nil {
			return err
		}
		now := ts(s.now())
		n := selectedCount(kind)
		seen := map[string]bool{}
		for _, c := range cands {
			ref := c.URL
			if c.Source == "local" {
				ref = c.LocalPath
			}
			if ref == "" || seen[ref] {
				continue
			}
			seen[ref] = true
			k := existing[ref]
			id := k.id
			if id == "" {
				id = newID()
			}
			selected := n > 0
			if selected {
				n--
			}
			w, h := c.Width, c.Height
			if k.w > 0 {
				w, h = k.w, k.h
			}
			var remote, local any
			if c.Source == "local" {
				local = c.LocalPath
			} else {
				remote = c.URL
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO artwork (id, owner_type, owner_id, kind, source, remote_url, local_path,
				language, width, height, votes, sha256, ext, colors, selected, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, o.Type, o.ID, kind, c.Source, remote, local, nullString(c.Language), nullInt(w), nullInt(h), c.Votes,
				nullString(k.sha), nullString(k.ext), nullString(k.colors), selected, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// fetchSelected downloads (or inspects, for local files) the selected
// images of an owner that aren't stored yet, and computes their colors
// (R17, R21). Failures are logged: the image is fetched on first request.
func (s *Service) fetchSelected(ctx context.Context, o owner, kinds map[string]bool) {
	if s.art == nil {
		return
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT id, kind, source, COALESCE(remote_url, ''), COALESCE(local_path, ''),
		COALESCE(sha256, '') FROM artwork WHERE owner_type = ? AND owner_id = ? AND selected = 1`, o.Type, o.ID)
	if err != nil {
		s.log.Error("artwork: listing selected images failed", "err", err)
		return
	}
	type todo struct{ id, kind, source, url, path, sha string }
	var list []todo
	for rows.Next() {
		var t todo
		if err := rows.Scan(&t.id, &t.kind, &t.source, &t.url, &t.path, &t.sha); err == nil {
			list = append(list, t)
		}
	}
	_ = rows.Close()
	for _, t := range list {
		if !kinds[t.kind] || (t.sha != "" && t.source != "local") {
			continue
		}
		if _, err := s.storeImage(ctx, t.id); err != nil {
			s.log.Warn("artwork not fetched, will retry on request", "artwork", t.id, "err", err)
		}
	}
}

// storedImage is an artwork row ready to be served.
type storedImage struct {
	ID       string
	Kind     string
	Source   string
	URL      string
	Local    string
	Original artwork.Original
	Colors   string
	OwnerTyp string
	OwnerID  string
}

// ErrImageNotFound means no such artwork, or a local file that's gone.
var ErrImageNotFound = errors.New("image not found")

func (s *Store) loadImage(ctx context.Context, id string) (storedImage, error) {
	var (
		im       storedImage
		sha, ext string
		w, h     int
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, kind, source, COALESCE(remote_url, ''), COALESCE(local_path, ''),
		COALESCE(sha256, ''), COALESCE(ext, ''), COALESCE(width, 0), COALESCE(height, 0), COALESCE(colors, ''), owner_type, owner_id
		FROM artwork WHERE id = ?`, id).Scan(&im.ID, &im.Kind, &im.Source, &im.URL, &im.Local, &sha, &ext, &w, &h, &im.Colors,
		&im.OwnerTyp, &im.OwnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return im, ErrImageNotFound
	}
	im.Original = artwork.Original{SHA256: sha, Ext: ext, Width: w, Height: h}
	return im, err
}

// storeImage makes sure an artwork row's image is on disk (downloaded, or
// a local file still there and unchanged), with its colors.
func (s *Service) storeImage(ctx context.Context, id string) (storedImage, error) {
	im, err := s.store.loadImage(ctx, id)
	if err != nil {
		return im, err
	}
	var o artwork.Original
	switch im.Source {
	case "local":
		if o, err = artwork.Inspect(im.Local); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return im, ErrImageNotFound
			}
			return im, err
		}
		if o == im.Original && im.Colors != "" {
			return im, nil
		}
	default:
		if im.Original.SHA256 != "" {
			return im, nil
		}
		url := im.URL
		if im.Source == "tmdb" {
			url = tmdbImages + tmdbSize[im.Kind] + im.URL
		}
		if o, err = s.art.Download(ctx, url); err != nil {
			return im, err
		}
	}
	colors := ""
	if im.Kind != "logo" && im.Kind != "clearart" && im.Kind != "portrait" {
		path := im.Local
		if path == "" {
			path = s.art.OriginalPath(o)
		}
		if raw, err := os.ReadFile(path); err == nil { //nolint:gosec // our original or a library image
			if img, _, err := image.Decode(bytes.NewReader(raw)); err == nil {
				b, _ := json.Marshal(artwork.Palette(img))
				colors = string(b)
			}
		}
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE artwork SET sha256 = ?, ext = ?, width = ?, height = ?, colors = ?, fetched_at = ?
		WHERE id = ?`, o.SHA256, o.Ext, o.Width, o.Height, nullString(colors), ts(s.o.Now()), id); err != nil {
		return im, err
	}
	im.Original, im.Colors = o, colors
	return im, nil
}

// movieArtwork chooses a movie's artwork from its local files, TMDB and
// fanart.tv (R13, R15–R17).
func (s *Service) movieArtwork(ctx context.Context, p parsed, d *provider.Details) error {
	dirs, bases, err := s.store.itemDirs(ctx, p.ID)
	if err != nil {
		return err
	}
	local := localImages(dirs, bases)
	var tmdb provider.Images
	var fan []fanart.Image
	if d != nil {
		tmdb = d.Images
		if f := s.fanart(); f != nil {
			if fan, err = f.Movie(ctx, d.TMDBID); err != nil {
				s.log.Warn("fanart.tv images not fetched", "tmdb", d.TMDBID, "err", err)
			}
		}
	}
	lang := p.Locale.Language
	o := owner{"item", p.ID}
	kinds := map[string][]candidate{
		"poster":   append(rankRemote("poster", lang, tmdbCandidates(tmdb.Posters)), rankRemote("poster", lang, fanartCandidates(fan, fanart.Poster, -1))...),
		"backdrop": append(rankRemote("backdrop", lang, tmdbCandidates(tmdb.Backdrops)), rankRemote("backdrop", lang, fanartCandidates(fan, fanart.Backdrop, -1))...),
		"logo":     append(rankRemote("logo", lang, fanartCandidates(fan, fanart.Logo, -1)), rankRemote("logo", lang, tmdbCandidates(tmdb.Logos))...),
		"clearart": rankRemote("clearart", lang, fanartCandidates(fan, fanart.Clearart, -1)),
		"thumb":    rankRemote("thumb", lang, fanartCandidates(fan, fanart.Thumb, -1)),
		"banner":   nil,
	}
	for kind, remote := range kinds {
		if err := s.store.syncArtwork(ctx, o, kind, append(local[kind], remote...)); err != nil {
			return err
		}
	}
	s.fetchSelected(ctx, o, eager)
	return nil
}

// showArtwork does the same for a show, its seasons and episodes.
func (s *Service) showArtwork(ctx context.Context, p parsed, d *provider.Details, children []child, seasons map[int]*provider.Season) error {
	showDir, err := s.store.showDir(ctx, p.ID)
	if err != nil {
		return err
	}
	var local map[string][]candidate
	if showDir != "" {
		local = localImages([]string{showDir}, nil)
	}
	var tmdb provider.Images
	var fan []fanart.Image
	if d != nil {
		tmdb = d.Images
		if f := s.fanart(); f != nil && d.TVDBID != 0 {
			if fan, err = f.Show(ctx, d.TVDBID); err != nil {
				s.log.Warn("fanart.tv images not fetched", "tvdb", d.TVDBID, "err", err)
			}
		}
	}
	lang := p.Locale.Language
	o := owner{"item", p.ID}
	kinds := map[string][]candidate{
		"poster":   append(rankRemote("poster", lang, tmdbCandidates(tmdb.Posters)), rankRemote("poster", lang, fanartCandidates(fan, fanart.Poster, -1))...),
		"backdrop": append(rankRemote("backdrop", lang, tmdbCandidates(tmdb.Backdrops)), rankRemote("backdrop", lang, fanartCandidates(fan, fanart.Backdrop, -1))...),
		"logo":     append(rankRemote("logo", lang, fanartCandidates(fan, fanart.Logo, -1)), rankRemote("logo", lang, tmdbCandidates(tmdb.Logos))...),
		"clearart": rankRemote("clearart", lang, fanartCandidates(fan, fanart.Clearart, -1)),
		"thumb":    rankRemote("thumb", lang, fanartCandidates(fan, fanart.Thumb, -1)),
		"banner":   nil,
	}
	for kind, remote := range kinds {
		if err := s.store.syncArtwork(ctx, o, kind, append(local[kind], remote...)); err != nil {
			return err
		}
	}
	s.fetchSelected(ctx, o, eager)

	seasonOf := map[string]int{}
	for _, c := range children {
		if c.Kind != "season" {
			continue
		}
		seasonOf[c.ID] = c.Season
		dirs, err := s.store.childDirs(ctx, c.ID)
		if err != nil {
			return err
		}
		cands := seasonLocal(showDir, dirs, c.Season)
		if sn := seasons[c.Season]; sn != nil && sn.PosterPath != "" {
			cands = append(cands, candidate{Source: "tmdb", URL: sn.PosterPath})
		}
		cands = append(cands, rankRemote("poster", lang, fanartCandidates(fan, fanart.Poster, c.Season))...)
		so := owner{"item", c.ID}
		if err := s.store.syncArtwork(ctx, so, "poster", cands); err != nil {
			return err
		}
		s.fetchSelected(ctx, so, map[string]bool{"poster": true})
	}
	// Episode stills: a local <video>-thumb, else TMDB's, fetched lazily.
	for _, c := range children {
		if c.Kind != "episode" {
			continue
		}
		dirs, bases, err := s.store.itemDirs(ctx, c.ID)
		if err != nil {
			return err
		}
		cands := localImages(dirs, bases)["thumb"]
		if sn := seasons[seasonOf[c.ParentID]]; sn != nil {
			for _, ep := range sn.Episodes {
				if (c.Episode >= 0 && ep.Number == c.Episode || c.Episode < 0 && ep.AirDate == c.AirDate) && ep.StillPath != "" {
					cands = append(cands, candidate{Source: "tmdb", URL: ep.StillPath})
				}
			}
		}
		if err := s.store.syncArtwork(ctx, owner{"item", c.ID}, "still", cands); err != nil {
			return err
		}
	}
	return nil
}

// childDirs returns the folders of a season's episodes.
func (s *Store) childDirs(ctx context.Context, seasonID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT f.path FROM items e JOIN item_files x ON x.item_id = e.id
		JOIN media_files f ON f.id = x.file_id WHERE e.parent_id = ? AND f.missing_since IS NULL`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dirs []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if d := filepath.Dir(p); !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	return dirs, rows.Err()
}

// portraits records people's TMDB portraits, fetched lazily (R17).
func (s *Store) portraits(ctx context.Context, people []provider.Person) error {
	for _, p := range people {
		if p.ProfilePath == "" || p.TMDBID == 0 {
			continue
		}
		var id, current string
		if err := s.db.QueryRowContext(ctx, `SELECT id, COALESCE(portrait_id, '') FROM people WHERE tmdb_id = ?`, p.TMDBID).Scan(&id, &current); err != nil {
			continue
		}
		if current != "" {
			continue
		}
		if err := s.syncArtwork(ctx, owner{"person", id}, "portrait", []candidate{{Source: "tmdb", URL: p.ProfilePath}}); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE people SET portrait_id = (SELECT id FROM artwork WHERE owner_type = 'person'
			AND owner_id = ? AND selected = 1) WHERE id = ?`, id, id); err != nil {
			return err
		}
	}
	return nil
}

// SidecarsSeen is library.Hooks.Sidecars (R14): it records the .nfo and
// image files of a folder and refreshes the movies and shows whose files
// changed, so their artwork and .nfo fields follow the disk.
func (s *Service) SidecarsSeen(libraryID, folder string, files []library.Sidecar) {
	ctx := context.Background()
	changed, err := s.store.diffSidecars(ctx, libraryID, folder, files)
	if err != nil {
		s.log.Error("metadata: sidecar files not recorded", "err", err)
		return
	}
	if len(changed) == 0 {
		return
	}
	if err := s.store.refreshOwnersOf(ctx, libraryID, changed); err != nil {
		s.log.Error("metadata: refreshing items after a sidecar change failed", "err", err)
		return
	}
	s.queue.Wake()
}

// diffSidecars updates sidecar_files for one folder and returns the
// directories where a file appeared, changed or went away.
func (s *Store) diffSidecars(ctx context.Context, libraryID, folder string, files []library.Sidecar) ([]string, error) {
	prefix := strings.TrimSuffix(folder, string(filepath.Separator)) + string(filepath.Separator)
	known := map[string]string{} // path → size|mtime
	rows, err := s.db.QueryContext(ctx, `SELECT path, size, mtime FROM sidecar_files WHERE library_id = ? AND substr(path, 1, ?) = ?`,
		libraryID, len(prefix), prefix)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			p, mtime string
			size     int64
		)
		if err := rows.Scan(&p, &size, &mtime); err != nil {
			_ = rows.Close()
			return nil, err
		}
		known[p] = strconv.FormatInt(size, 10) + "|" + mtime
	}
	_ = rows.Close()
	var dirs []string
	add := func(p string) {
		if d := filepath.Dir(p); !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	now := ts(s.now())
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		for _, f := range files {
			sig := strconv.FormatInt(f.Size, 10) + "|" + ts(f.ModTime)
			if old, ok := known[f.Path]; ok {
				delete(known, f.Path)
				if old == sig {
					continue
				}
			}
			add(f.Path)
			if _, err := tx.ExecContext(ctx, `INSERT INTO sidecar_files (path, library_id, size, mtime, seen_at) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (path) DO UPDATE SET size = excluded.size, mtime = excluded.mtime, seen_at = excluded.seen_at`,
				f.Path, libraryID, f.Size, ts(f.ModTime), now); err != nil {
				return err
			}
		}
		for gone := range known {
			add(gone)
			if _, err := tx.ExecContext(ctx, `DELETE FROM sidecar_files WHERE path = ?`, gone); err != nil {
				return err
			}
		}
		return nil
	})
	return dirs, err
}

// refreshOwnersOf schedules an immediate refresh of the movies and shows
// with files in (or under) the given directories. Pending items are left
// to their first match.
func (s *Store) refreshOwnersOf(ctx context.Context, libraryID string, dirs []string) error {
	for _, d := range dirs {
		prefix := strings.TrimSuffix(d, string(filepath.Separator)) + string(filepath.Separator)
		if _, err := s.db.ExecContext(ctx, `WITH RECURSIVE up(id, parent) AS (
				SELECT i.id, i.parent_id FROM media_files f JOIN item_files x ON x.file_id = f.id JOIN items i ON i.id = x.item_id
				WHERE f.library_id = ? AND substr(f.path, 1, ?) = ?
				UNION SELECT p.id, p.parent_id FROM items p JOIN up ON p.id = up.parent)
			UPDATE item_metadata SET refresh_at = ? WHERE state <> 'pending'
			AND item_id IN (SELECT up.id FROM up JOIN items i ON i.id = up.id WHERE i.kind IN ('movie', 'show'))`,
			libraryID, len(prefix), prefix, ts(s.now())); err != nil {
			return err
		}
	}
	return nil
}

// Image returns an image resized for the client (R19). An image of an item
// the profile may not see is ErrImageNotFound (R25). A lazy image is
// fetched now; a provider error is returned as is.
func (s *Service) Image(ctx context.Context, id string, want int, limit access.Limit) (artwork.Variant, error) {
	if s.art == nil {
		return artwork.Variant{}, ErrImageNotFound
	}
	im, err := s.store.loadImage(ctx, id)
	if err != nil {
		return artwork.Variant{}, err
	}
	if im.OwnerTyp == "item" && !limit.Unrestricted() {
		var age sql.NullInt64
		err := s.store.db.QueryRowContext(ctx, `SELECT age_rating FROM item_metadata WHERE item_id = ?`, im.OwnerID).Scan(&age)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return artwork.Variant{}, err
		}
		var rating *int
		if age.Valid {
			n := int(age.Int64)
			rating = &n
		}
		if !limit.Allows(rating) {
			return artwork.Variant{}, ErrImageNotFound
		}
	}
	if im.Original.SHA256 == "" || im.Source == "local" {
		if im, err = s.storeImage(ctx, id); err != nil {
			return artwork.Variant{}, err
		}
	}
	src := artwork.Source{Original: im.Original, Alpha: im.Kind == "logo" || im.Kind == "clearart"}
	if im.Source == "local" {
		src.LocalPath = im.Local
	}
	if want <= 0 {
		want = 342
	}
	return s.art.Variant(ctx, src, want)
}

// IsProviderError reports errors that come from a provider or the network.
func IsProviderError(err error) bool {
	var se *httpx.StatusError
	return errors.Is(err, httpx.ErrUnreachable) || errors.Is(err, httpx.ErrInvalidKey) || errors.Is(err, httpx.ErrNotFound) ||
		errors.Is(err, httpx.ErrRequestFailed) || errors.As(err, &se)
}
