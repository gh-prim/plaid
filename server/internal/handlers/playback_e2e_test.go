package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
)

// These tests drive F006's PR A acceptance criteria through the real HTTP
// stack: playback sessions, Direct Play over ranges, the stream token,
// and watch state.

// fixtureProber answers with F004's recorded ffprobe outputs: an H.264/AAC
// file for .mp4, a 4K HEVC HDR10 one for .mkv. Durations are two hours so
// positions mean something.
type fixtureProber struct{}

func (fixtureProber) Probe(_ context.Context, path string) (*library.ProbeResult, error) {
	name := "sdr_h264_stereo"
	if strings.HasSuffix(path, ".mkv") {
		name = "hevc_hdr10_multiaudio"
	}
	raw, err := os.ReadFile("../library/testdata/probe/" + name + ".json")
	if err != nil {
		return nil, err
	}
	res, err := library.ParseProbe(raw)
	if err != nil {
		return nil, err
	}
	res.DurationMs = 2 * 60 * 60 * 1000
	return res, nil
}

var (
	external = map[string]any{"preset": "external"}
	browser  = map[string]any{
		"protocols": []string{"http", "hls"}, "containers": []string{"mp4", "webm"},
		"video": []map[string]any{{"codec": "h264", "maxLevel": 52}},
		"audio": []map[string]any{{"codec": "aac", "maxChannels": 6}},
	}
)

type sessionJSON struct {
	ID              string   `json:"id"`
	Method          string   `json:"method"`
	Reasons         []string `json:"reasons"`
	URL             string   `json:"url"`
	FileID          string   `json:"fileId"`
	DurationMs      int64    `json:"durationMs"`
	StartPositionMs int64    `json:"startPositionMs"`
	Quality         string   `json:"quality"`
	AudioTracks     []struct {
		Index    int  `json:"index"`
		Selected bool `json:"selected"`
	} `json:"audioTracks"`
}

type userStateJSON struct {
	PositionMs      int64  `json:"positionMs"`
	DurationMs      *int64 `json:"durationMs"`
	Watched         bool   `json:"watched"`
	PlayCount       int    `json:"playCount"`
	WatchedEpisodes *int   `json:"watchedEpisodes"`
	TotalEpisodes   *int   `json:"totalEpisodes"`
}

// playEnv is a library with a 64 KiB "MP4", a "4K MKV" and a two-episode
// show, analyzed with the recorded outputs.
type playEnv struct {
	*libEnv
	movie, dune, show string
	content           []byte
}

func newPlayEnv(t *testing.T) *playEnv {
	t.Helper()
	e := &playEnv{libEnv: newLibEnv(t, libOpts{prober: fixtureProber{}})}
	movies := t.TempDir()
	e.content = make([]byte, 64<<10)
	_, _ = rand.Read(e.content)
	if err := os.WriteFile(filepath.Join(movies, "Paddington (2014).mp4"), e.content, 0o644); err != nil {
		t.Fatal(err)
	}
	touch(t, movies, "Dune (2021).mkv")
	shows := t.TempDir()
	touch(t, shows, "Dark/Season 1/Dark S01E01.mp4", "Dark/Season 1/Dark S01E02.mp4")
	ml, sl := e.create("Movies", "movies", movies), e.create("Shows", "shows", shows)
	e.waitIdle(ml.ID)
	e.waitIdle(sl.ID)
	var page itemPage
	e.do("GET", "/libraries/"+ml.ID+"/items", nil, bearer(e.admin)).json(&page)
	for _, it := range page.Items {
		switch *it.Title {
		case "Paddington":
			e.movie = it.ID
		case "Dune":
			e.dune = it.ID
		}
	}
	e.do("GET", "/libraries/"+sl.ID+"/items", nil, bearer(e.admin)).json(&page)
	e.show = page.Items[0].ID
	return e
}

func (e *playEnv) start(token, item string, body map[string]any) sessionJSON {
	e.t.Helper()
	r := e.do("POST", "/items/"+item+"/playback", body, bearer(token))
	e.expect(r, http.StatusCreated, "")
	var s sessionJSON
	r.json(&s)
	return s
}

// stream requests a stream URL the way a player does: no session at all.
func (e *playEnv) stream(method, url string, h map[string]string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest(method, url, nil)
	r.RemoteAddr = "192.168.1.20:50000"
	for k, v := range h {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	return rec
}

func (e *playEnv) userState(token, item string) userStateJSON {
	e.t.Helper()
	var it struct {
		UserState *userStateJSON `json:"userState"`
	}
	r := e.do("GET", "/items/"+item, nil, bearer(token))
	e.expect(r, http.StatusOK, "")
	r.json(&it)
	if it.UserState == nil {
		e.t.Fatal("no userState")
	}
	return *it.UserState
}

func TestDirectPlayThroughTheAPI(t *testing.T) {
	e := newPlayEnv(t)
	s := e.start(e.admin, e.movie, map[string]any{"profile": external, "startPositionMs": 600000})
	if s.Method != "direct_play" || len(s.Reasons) != 0 || !strings.HasPrefix(s.URL, "/api/v1/stream/") || !strings.HasSuffix(s.URL, "/file") {
		t.Fatalf("session = %+v", s)
	}
	if s.StartPositionMs != 600000 || s.DurationMs != 7_200_000 || s.Quality != "original" {
		t.Errorf("session = %+v", s)
	}

	r := e.stream("GET", s.URL, map[string]string{"Range": "bytes=1000-1999"})
	if r.Code != http.StatusPartialContent || !bytes.Equal(r.Body.Bytes(), e.content[1000:2000]) {
		t.Fatalf("range: %d, %d bytes", r.Code, r.Body.Len())
	}
	for k, want := range map[string]string{
		"Content-Range": "bytes 1000-1999/65536", "Accept-Ranges": "bytes", "Content-Type": "video/mp4",
		"Referrer-Policy": "no-referrer", "Content-Disposition": `inline; filename="Paddington (2014).mp4"`,
	} {
		if got := r.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	etag := r.Header().Get("ETag")
	if r := e.stream("HEAD", s.URL, nil); r.Code != http.StatusOK || r.Header().Get("Content-Length") != "65536" || r.Body.Len() != 0 {
		t.Errorf("HEAD: %d, length %q, body %d", r.Code, r.Header().Get("Content-Length"), r.Body.Len())
	}
	if r := e.stream("GET", s.URL, map[string]string{"Range": "bytes=999999-"}); r.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("range past the end: %d", r.Code)
	}
	if r := e.stream("GET", s.URL, map[string]string{"Range": "bytes=0-9", "If-Range": etag}); r.Code != http.StatusPartialContent {
		t.Errorf("If-Range with the current ETag: %d", r.Code)
	}
	// Only GET and HEAD are public: anything else needs a session first.
	if r := e.stream("POST", s.URL, nil); r.Code != http.StatusUnauthorized {
		t.Errorf("POST on a stream: %d", r.Code)
	}
	for _, bad := range []string{
		strings.TrimSuffix(s.URL, "file") + "../../../../etc/passwd",
		strings.TrimSuffix(s.URL, "file") + "file/..%2f..%2fetc%2fpasswd",
		"/api/v1/stream/forged-token/file",
	} {
		if r := e.stream("GET", bad, nil); r.Code == http.StatusOK || r.Code == http.StatusPartialContent {
			t.Errorf("%s: %d", bad, r.Code)
		}
	}
	token := strings.Split(strings.TrimPrefix(s.URL, "/api/v1/stream/"), "/")[0]
	if strings.Contains(e.logs.String(), token) {
		t.Error("the playback token is in the logs")
	}

	if r := e.do("DELETE", "/playback/"+s.ID, nil, bearer(e.admin)); r.Code != http.StatusNoContent {
		t.Fatalf("stop: %d %s", r.Code, r.Body)
	}
	if r := e.stream("GET", s.URL, nil); r.Code != http.StatusNotFound || !strings.Contains(r.Body.String(), "playback_session_not_found") {
		t.Errorf("after the stop: %d %s", r.Code, r.Body)
	}
}

func TestStreamDiesWithItsDeviceSession(t *testing.T) {
	e := newPlayEnv(t)
	phone := e.login("ada", adminPassword)
	s := e.start(phone.Token, e.movie, map[string]any{"profile": external})
	if r := e.stream("GET", s.URL, map[string]string{"Range": "bytes=0-9"}); r.Code != http.StatusPartialContent {
		t.Fatalf("before: %d", r.Code)
	}
	e.expect(e.do("DELETE", "/me/sessions/"+phone.Session.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	if r := e.stream("GET", s.URL, map[string]string{"Range": "bytes=0-9"}); r.Code != http.StatusNotFound {
		t.Errorf("after signing the phone out: %d", r.Code)
	}
	e.expect(e.do("GET", "/playback/"+s.ID, nil, bearer(e.admin)), http.StatusNotFound, "playback_session_not_found")
}

func TestPlaybackSessionsBelongToTheirUser(t *testing.T) {
	e := newPlayEnv(t)
	bob := e.member(e.admin, "bob")
	s := e.start(e.admin, e.movie, map[string]any{"profile": external})
	e.expect(e.do("GET", "/playback/"+s.ID, nil, bearer(e.admin)), http.StatusOK, "")
	e.expect(e.do("GET", "/playback/"+s.ID, nil, bearer(bob.Token)), http.StatusNotFound, "playback_session_not_found")
	e.expect(e.do("DELETE", "/playback/"+s.ID, nil, bearer(bob.Token)), http.StatusNotFound, "playback_session_not_found")
	e.expect(e.do("GET", "/playback", nil, bearer(bob.Token)), http.StatusForbidden, "forbidden")

	b := e.start(bob.Token, e.movie, map[string]any{"profile": external})
	var list struct {
		Sessions []struct {
			Username, DeviceName, ProfileName string
			Remote                            bool
			Session                           sessionJSON
		}
	}
	r := e.do("GET", "/playback", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&list)
	if len(list.Sessions) != 2 || list.Sessions[1].Username != "bob" || list.Sessions[1].Session.ID != b.ID || list.Sessions[1].Remote {
		t.Errorf("admin list = %+v", list.Sessions)
	}
	e.expect(e.do("DELETE", "/playback/"+b.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
}

func TestPlaybackErrors(t *testing.T) {
	e := newPlayEnv(t)
	e.expect(e.do("POST", "/items/"+e.dune+"/playback", map[string]any{"profile": browser}, bearer(e.admin)),
		http.StatusServiceUnavailable, "ffmpeg_missing")
	e.expect(e.do("POST", "/items/"+e.show+"/playback", map[string]any{"profile": external}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "not_playable")
	e.expect(e.do("POST", "/items/nope/playback", map[string]any{"profile": external}, bearer(e.admin)),
		http.StatusNotFound, "not_found")
	e.expect(e.do("POST", "/items/"+e.movie+"/playback", map[string]any{"profile": external, "fileId": "nope"}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	e.expect(e.do("POST", "/items/"+e.movie+"/playback", map[string]any{"profile": external, "audioStreamIndex": 42}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	e.expect(e.do("POST", "/items/"+e.movie+"/playback", map[string]any{"profile": external, "quality": "8k"}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")

	// Rated 16: a 12+ profile can't play it, and can't tell it exists.
	e.rate(e.movie, 16)
	r := e.do("POST", "/me/profiles", map[string]any{"name": "Kids", "maxAge": 12}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var kids struct{ ID string }
	r.json(&kids)
	// Two profiles now: a new session starts with none picked.
	tv := e.login("ada", adminPassword)
	e.expect(e.do("POST", "/items/"+e.movie+"/playback", map[string]any{"profile": external}, bearer(tv.Token)),
		http.StatusForbidden, "profile_required")
	e.expect(e.do("POST", "/session/profile", map[string]any{"profileId": kids.ID}, bearer(tv.Token)), http.StatusOK, "")
	e.expect(e.do("POST", "/items/"+e.movie+"/playback", map[string]any{"profile": external}, bearer(tv.Token)),
		http.StatusNotFound, "not_found")
}

func TestWatchStateThroughTheAPI(t *testing.T) {
	e := newPlayEnv(t)
	progress := func(id string, pos int64, state string) {
		t.Helper()
		e.clock.advance(11 * time.Second) // past the 10 s write throttle
		e.expect(e.do("POST", "/playback/"+id+"/progress", map[string]any{"positionMs": pos, "state": state}, bearer(e.admin)),
			http.StatusNoContent, "")
	}
	s := e.start(e.admin, e.movie, map[string]any{"profile": external})
	progress(s.ID, 3_600_000, "playing")
	if st := e.userState(e.admin, e.movie); st.PositionMs != 3_600_000 || st.Watched || st.DurationMs == nil || *st.DurationMs <= st.PositionMs {
		t.Errorf("halfway = %+v", st)
	}
	var page struct{ Items []struct{ ID string } }
	e.do("GET", "/me/continue-watching", nil, bearer(e.admin)).json(&page)
	if len(page.Items) != 1 || page.Items[0].ID != e.movie {
		t.Errorf("continue watching = %+v", page.Items)
	}

	// Stopping at 91%: watched, position reset, out of continue watching.
	e.expect(e.do("DELETE", "/playback/"+s.ID, map[string]any{"positionMs": 6_552_000}, bearer(e.admin)), http.StatusNoContent, "")
	if st := e.userState(e.admin, e.movie); !st.Watched || st.PositionMs != 0 || st.PlayCount != 1 {
		t.Errorf("after 91%% = %+v", st)
	}
	e.do("GET", "/me/continue-watching", nil, bearer(e.admin)).json(&page)
	if len(page.Items) != 0 {
		t.Errorf("continue watching after the end = %+v", page.Items)
	}

	// VLC on a direct link never reports: watch state stays as it was.
	v := e.start(e.admin, e.dune, map[string]any{"profile": external})
	e.expect(e.do("DELETE", "/playback/"+v.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	if st := e.userState(e.admin, e.dune); st.PlayCount != 0 || st.PositionMs != 0 {
		t.Errorf("unreported session changed watch state: %+v", st)
	}

	// A whole show.
	e.expect(e.do("PUT", "/items/"+e.show+"/watched", nil, bearer(e.admin)), http.StatusNoContent, "")
	if st := e.userState(e.admin, e.show); !st.Watched || st.WatchedEpisodes == nil || *st.WatchedEpisodes != 2 || *st.TotalEpisodes != 2 {
		t.Errorf("show marked watched = %+v", st)
	}
	e.expect(e.do("DELETE", "/items/"+e.show+"/watched", nil, bearer(e.admin)), http.StatusNoContent, "")
	if st := e.userState(e.admin, e.show); st.Watched || *st.WatchedEpisodes != 0 {
		t.Errorf("show unmarked = %+v", st)
	}
	e.expect(e.do("PUT", "/items/nope/watched", nil, bearer(e.admin)), http.StatusNotFound, "not_found")
	e.expect(e.do("POST", "/playback/"+s.ID+"/progress", map[string]any{"positionMs": 1, "state": "playing"}, bearer(e.admin)),
		http.StatusNotFound, "playback_session_not_found")
}
