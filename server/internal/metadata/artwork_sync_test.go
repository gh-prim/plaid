package metadata

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/library"
)

type artRow struct {
	ID, Kind, Source, Lang, SHA, Colors string
	Selected, Locked                    bool
}

func (e *applyEnv) art(ownerID, kind string) []artRow {
	e.t.Helper()
	rows, err := e.db.Query(`SELECT id, kind, source, COALESCE(language, ''), COALESCE(sha256, ''), COALESCE(colors, ''), selected, locked
		FROM artwork WHERE owner_id = ? AND kind = ? ORDER BY rowid`, ownerID, kind)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []artRow
	for rows.Next() {
		var r artRow
		if err := rows.Scan(&r.ID, &r.Kind, &r.Source, &r.Lang, &r.SHA, &r.Colors, &r.Selected, &r.Locked); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func selected(rows []artRow) []artRow {
	return slices.DeleteFunc(slices.Clone(rows), func(r artRow) bool { return !r.Selected })
}

func (e *applyEnv) writeJPEG(rel string, c color.NRGBA) {
	e.t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 60, 90))
	for y := range 90 {
		for x := range 60 {
			img.SetNRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		e.t.Fatal(err)
	}
	e.write(rel, b.String())
}

// tree lists a folder's files with their size, to prove nothing is written.
func tree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			info, _ := d.Info()
			out = append(out, p+" "+info.ModTime().String())
		}
		return nil
	})
	return out
}

func TestMovieArtwork(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true, withArtwork)
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "x")
	e.scan()
	id := e.item(`kind = 'movie'`)
	poster := selected(e.art(id, "poster"))
	if len(poster) != 1 || poster[0].Source != "tmdb" || poster[0].SHA == "" || !strings.Contains(poster[0].Colors, "dominant") {
		t.Errorf("poster = %+v", poster)
	}
	if bd := selected(e.art(id, "backdrop")); len(bd) != 3 || bd[0].SHA == "" || bd[0].Lang != "" {
		t.Errorf("backdrops = %+v, want 3 downloaded, the first without text", bd)
	}
	for _, kind := range []string{"logo", "clearart"} {
		got := selected(e.art(id, kind))
		if len(got) != 1 || got[0].Source != "fanart" || got[0].SHA == "" || got[0].Colors != "" {
			t.Errorf("%s = %+v, want fanart.tv first, downloaded, no colors", kind, got)
		}
	}
	// Portraits are recorded but only fetched when asked for (R17).
	var portraits, fetched int
	_ = e.db.QueryRow(`SELECT COUNT(*), COUNT(sha256) FROM artwork WHERE kind = 'portrait'`).Scan(&portraits, &fetched)
	if portraits == 0 || fetched != 0 {
		t.Errorf("portraits: %d recorded, %d fetched", portraits, fetched)
	}
	files, _ := filepath.Glob(filepath.Join(e.svc.o.DataDir, "artwork", "originals", "*", "*"))
	if len(files) != 6 {
		t.Errorf("%d originals stored, want 6 (poster, 3 backdrops, logo, clearart)", len(files))
	}
}

func TestLocalArtworkFollowsTheDisk(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true, withArtwork)
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "x")
	e.writeJPEG("The Matrix (1999)/poster.jpg", color.NRGBA{R: 200, A: 255})
	before := tree(t, e.root)
	e.scan()
	id := e.item(`kind = 'movie'`)
	if p := selected(e.art(id, "poster")); len(p) != 1 || p[0].Source != "local" || p[0].SHA == "" {
		t.Errorf("poster = %+v, want the local file", p)
	}
	if after := tree(t, e.root); !slices.Equal(before, after) {
		t.Errorf("library folder changed:\n%v\n%v", before, after)
	}

	// A backdrop added after the match is picked up by the next scan.
	e.writeJPEG("The Matrix (1999)/fanart.jpg", color.NRGBA{B: 200, A: 255})
	e.scan()
	if bd := selected(e.art(id, "backdrop")); len(bd) != 3 || bd[0].Source != "local" {
		t.Errorf("backdrops = %+v, want the local one first", bd)
	}
	// Removing the local poster falls back to TMDB.
	if err := os.Remove(filepath.Join(e.root, "The Matrix (1999)", "poster.jpg")); err != nil {
		t.Fatal(err)
	}
	e.scan()
	if p := selected(e.art(id, "poster")); len(p) != 1 || p[0].Source != "tmdb" {
		t.Errorf("poster after removing the file = %+v", p)
	}
}

func TestLockedArtworkIsKept(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true, withArtwork)
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "x")
	e.scan()
	id := e.item(`kind = 'movie'`)
	posters := e.art(id, "poster")
	pick := posters[len(posters)-1]
	if _, err := e.db.Exec(`UPDATE artwork SET selected = (id = ?), locked = 1 WHERE owner_id = ? AND kind = 'poster'`, pick.ID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE item_metadata SET refresh_at = '2000-01-01T00:00:00Z' WHERE item_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	e.drain()
	if p := selected(e.art(id, "poster")); len(p) != 1 || p[0].ID != pick.ID {
		t.Errorf("locked poster replaced by %+v", p)
	}
}

func TestShowArtwork(t *testing.T) {
	e := newApplyEnv(t, library.TypeShows, "en", "US", true, withArtwork)
	e.write("Breaking Bad/Season 01/Breaking Bad S01E01.mkv", "x")
	e.scan()
	show := e.item(`kind = 'show'`)
	season := e.item(`kind = 'season'`)
	episode := e.item(`kind = 'episode'`)
	if p := selected(e.art(show, "poster")); len(p) != 1 || p[0].SHA == "" {
		t.Errorf("show poster = %+v", p)
	}
	if l := selected(e.art(show, "logo")); len(l) != 1 || l[0].Source != "fanart" {
		t.Errorf("show logo = %+v (fanart.tv, by TVDB id)", l)
	}
	if p := selected(e.art(season, "poster")); len(p) != 1 || p[0].Source != "tmdb" || p[0].SHA == "" {
		t.Errorf("season poster = %+v", p)
	}
	still := selected(e.art(episode, "still"))
	if len(still) != 1 || still[0].SHA != "" {
		t.Fatalf("still = %+v, want one, not fetched yet", still)
	}
	// Asking for it fetches it.
	v, err := e.svc.Image(context.Background(), still[0].ID, 300, access.Limit{})
	if err != nil || v.Width != 342 {
		t.Fatalf("still = %+v, %v", v, err)
	}
	if s := selected(e.art(episode, "still")); s[0].SHA == "" {
		t.Error("still not stored after its first request")
	}
}

func TestImageAccessAndErrors(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true, withArtwork)
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "x")
	e.scan()
	id := e.item(`kind = 'movie'`)
	poster := selected(e.art(id, "poster"))[0]
	ctx := context.Background()
	twelve := 12
	if _, err := e.svc.Image(ctx, poster.ID, 300, access.Limit{MaxAge: &twelve}); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("an R-rated poster for a 12 profile: %v", err)
	}
	if _, err := e.svc.Image(ctx, "nope", 300, access.Limit{}); !errors.Is(err, ErrImageNotFound) {
		t.Errorf("unknown image: %v", err)
	}
	portrait := ""
	_ = e.db.QueryRow(`SELECT id FROM artwork WHERE kind = 'portrait' LIMIT 1`).Scan(&portrait)
	e.srv.Fail.Store(503)
	defer e.srv.Fail.Store(0)
	if _, err := e.svc.Image(ctx, portrait, 185, access.Limit{}); err == nil || !IsProviderError(err) {
		t.Errorf("lazy image while the provider fails: %v", err)
	}
	// Stored images keep working offline.
	if v, err := e.svc.Image(ctx, poster.ID, 300, access.Limit{}); err != nil || v.Path == "" {
		t.Errorf("stored poster offline: %+v, %v", v, err)
	}
}

func TestArtworkCleanupKeepsUsedOriginals(t *testing.T) {
	e := newApplyEnv(t, library.TypeMovies, "en", "US", true, withArtwork)
	e.write("The Matrix (1999)/The Matrix (1999).mkv", "x")
	e.scan()
	stray := filepath.Join(e.svc.o.DataDir, "artwork", "originals", "ff", strings.Repeat("f", 64)+".jpg")
	if err := os.MkdirAll(filepath.Dir(stray), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := e.svc.art.Cleanup(ctx, func(sha string) (bool, error) {
		var used int
		err := e.db.QueryRow(`SELECT COUNT(*) FROM artwork WHERE sha256 = ? AND source <> 'local'`, sha).Scan(&used)
		return used > 0, err
	})
	if err != nil || n != 1 {
		t.Errorf("cleanup removed %d (%v), want only the stray file", n, err)
	}
}
