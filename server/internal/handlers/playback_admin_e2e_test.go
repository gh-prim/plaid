package handlers_test

import (
	"bytes"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/tmdbtest"
)

// TestStreamArtwork: the playing item's artwork through the playback
// token, an episode falling back to its show's, and nothing once the
// session ends (F006 R40).
func TestStreamArtwork(t *testing.T) {
	e := &playEnv{libEnv: newLibEnv(t, libOpts{prober: fixtureProber{}, tmdb: tmdbtest.New(t)})}
	movies, shows := t.TempDir(), t.TempDir()
	touch(t, movies, "Alien (1979) {tmdb-348}/Alien (1979) {tmdb-348}.mp4")
	touch(t, shows, "Breaking Bad/Season 01/Breaking.Bad.S01E01.720p.mp4")
	ml, sl := e.create("Movies", "movies", movies), e.create("Shows", "shows", shows)
	e.waitIdle(ml.ID)
	e.waitIdle(sl.ID)

	// Wait for the movie's poster and the show's backdrop.
	var alien, episode string
	for deadline := time.Now().Add(metadataWait); time.Now().Before(deadline) && (alien == "" || episode == ""); time.Sleep(20 * time.Millisecond) {
		type art struct{ ImageID string }
		var page struct {
			Items []struct {
				ID       string
				Poster   *art
				Backdrop *art
			}
		}
		e.do("GET", "/libraries/"+ml.ID+"/items", nil, bearer(e.admin)).json(&page)
		if len(page.Items) == 1 && page.Items[0].Poster != nil {
			alien = page.Items[0].ID
		}
		e.do("GET", "/libraries/"+sl.ID+"/items", nil, bearer(e.admin)).json(&page)
		if len(page.Items) == 1 && page.Items[0].Backdrop != nil {
			e.do("GET", "/libraries/"+sl.ID+"/items?kind=episode", nil, bearer(e.admin)).json(&page)
			if len(page.Items) == 1 {
				episode = page.Items[0].ID
			}
		}
	}
	if alien == "" || episode == "" {
		t.Fatalf("artwork never showed up (movie %q, episode %q)", alien, episode)
	}

	s := e.start(e.admin, alien, map[string]any{"profile": external})
	art := strings.TrimSuffix(s.URL, "file") + "artwork/"
	r := e.art(art+"poster?w=300", nil)
	e.expect(r, http.StatusOK, "")
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(r.Body.Bytes())); err != nil || cfg.Width != 342 {
		t.Errorf("poster at w=300: %dpx (%v)", cfg.Width, err)
	}
	if h := r.Header(); h.Get("Content-Type") != "image/jpeg" || h.Get("ETag") == "" || h.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("headers = %v", h)
	}
	e.expect(e.art(art+"poster", map[string]string{"If-None-Match": r.Header().Get("ETag")}), http.StatusNotModified, "")
	e.expect(e.art(art+"backdrop", nil), http.StatusOK, "")
	// Only the playing item's poster, backdrop and still: nothing else is
	// reachable through the token.
	e.expect(e.art(art+"still", nil), http.StatusNotFound, "not_found")
	e.expect(e.art(art+"logo", nil), http.StatusNotFound, "not_found")
	e.expect(e.art(art+"poster?w=0", nil), http.StatusUnprocessableEntity, "validation_failed")

	// An episode has no poster of its own: its season's or show's.
	ep := e.start(e.admin, episode, map[string]any{"profile": external})
	epArt := strings.TrimSuffix(ep.URL, "file") + "artwork/"
	e.expect(e.art(epArt+"poster", nil), http.StatusOK, "")
	e.expect(e.art(epArt+"backdrop", nil), http.StatusOK, "")

	// It stops with the session.
	e.expect(e.do("DELETE", "/playback/"+s.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	e.expect(e.art(art+"poster", nil), http.StatusNotFound, "playback_session_not_found")
}

// art gets a stream URL as a resp, for expect.
func (e *playEnv) art(url string, h map[string]string) resp {
	return resp{e.stream("GET", url, h), e.t}
}

// TestHardwareSettingsThroughTheAPI: the hardware status in the
// capabilities and the settings, and a failing explicit choice (F006 R24,
// R30, R31). No machine running the tests has VAAPI (CI has no GPU; a
// Mac's FFmpeg has no VAAPI), so it's refused there.
func TestHardwareSettingsThroughTheAPI(t *testing.T) {
	e := newHLSEnv(t)
	var caps struct {
		HwAccel struct {
			Configured string
			Encoders   []string
		}
	}
	r := e.do("GET", "/server/capabilities", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&caps)
	if caps.HwAccel.Configured != "auto" || caps.HwAccel.Encoders == nil {
		t.Errorf("capabilities hwAccel = %+v", caps.HwAccel)
	}

	e.expect(e.do("PATCH", "/settings/playback", map[string]any{"hwAccel": "vaapi", "hwDevice": "/dev/dri/renderD199"}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "hw_accel_unavailable")
	e.expect(e.do("PATCH", "/settings/playback", map[string]any{"hwAccel": "cuda"}, bearer(e.admin)), http.StatusUnprocessableEntity, "validation_failed")
	e.expect(e.do("PATCH", "/settings/playback", map[string]any{"hwDevice": "renderD128"}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")

	var set struct {
		HwAccel  string
		HwDevice *string
		Hardware struct {
			Configured string
			Active     *string
		}
	}
	r = e.do("PATCH", "/settings/playback", map[string]any{"hwAccel": "none"}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&set)
	if set.HwAccel != "none" || set.HwDevice != nil || set.Hardware.Configured != "none" || set.Hardware.Active != nil {
		t.Errorf("after hwAccel none: %+v", set)
	}
	bob := e.member(e.admin, "bob")
	e.expect(e.do("PATCH", "/settings/playback", map[string]any{"hwAccel": "auto"}, bearer(bob.Token)), http.StatusForbidden, "forbidden")
}
