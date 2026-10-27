package handlers_test

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/playback"
	"github.com/gh-prim/plaid/server/internal/playback/testmedia"
)

// These tests drive F006's PR B acceptance criteria through the real HTTP
// stack, with real files analyzed by ffprobe and played over HLS by
// FFmpeg. They skip without FFmpeg.

var hlsTimings = playback.Timings{
	WaitAhead: 3, SegmentWait: 20 * time.Second, ThrottleAhead: 120_000, ResumeAhead: 60_000,
	IdleStop: 2 * time.Second, SlotIdle: time.Hour, KeepBehind: 300_000, Tick: 50 * time.Millisecond,
}

// hlsEnv is a movie library of generated clips.
type hlsEnv struct {
	*libEnv
	items map[string]string // title → item id
}

func newHLSEnv(t *testing.T) *hlsEnv {
	t.Helper()
	testmedia.FFmpeg(t)
	prober, err := library.FindFFprobe(context.Background(), "", nil)
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	timings := hlsTimings
	e := &hlsEnv{libEnv: newLibEnv(t, libOpts{prober: prober, hls: &timings}), items: map[string]string{}}
	movies := t.TempDir()
	for name, clip := range map[string]string{
		"Long (2001).mkv": testmedia.Long, "Subs (2002).mkv": testmedia.MKV, "Noindex (2003).ts": testmedia.TS,
		"Frames (2004).mp4": testmedia.MP4,
	} {
		link(t, testmedia.Clip(t, clip), filepath.Join(movies, name))
	}
	link(t, filepath.Join(testmedia.Dir(t), testmedia.ExtSRT), filepath.Join(movies, "Subs (2002).fr.forced.srt"))
	lib := e.create("Movies", "movies", movies)
	e.waitIdle(lib.ID)
	var page itemPage
	e.do("GET", "/libraries/"+lib.ID+"/items", nil, bearer(e.admin)).json(&page)
	for _, it := range page.Items {
		e.items[*it.Title] = it.ID
	}
	if len(e.items) != 4 {
		t.Fatalf("items: %v", e.items)
	}
	return e
}

// link hard-links a clip into the library, copying across file systems.
func link(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.Link(src, dst); err != nil {
		testmedia.Copy(t, src, dst)
	}
}

type hlsSession struct {
	ID        string   `json:"id"`
	Method    string   `json:"method"`
	Reasons   []string `json:"reasons"`
	URL       string   `json:"url"`
	Quality   string   `json:"quality"`
	Subtitles []struct {
		Index    int    `json:"index"`
		Selected bool   `json:"selected"`
		Forced   bool   `json:"forced"`
		Delivery string `json:"delivery"`
		URL      string `json:"url"`
	} `json:"subtitles"`
}

func (e *hlsEnv) startHLS(token, title string, body map[string]any) hlsSession {
	e.t.Helper()
	r := e.do("POST", "/items/"+e.items[title]+"/playback", body, bearer(token))
	e.expect(r, http.StatusCreated, "")
	var s hlsSession
	r.json(&s)
	return s
}

// get fetches a stream URL as a player would: no session.
func (e *hlsEnv) get(url string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest("GET", url, nil)
	r.RemoteAddr = "192.168.1.20:50000"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	return rec
}

// mediaPlaylist resolves the master's variant and lists segment durations.
func (e *hlsEnv) mediaPlaylist(master string) (url string, durations []float64) {
	e.t.Helper()
	r := e.get(master)
	if r.Code != http.StatusOK || r.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" ||
		r.Header().Get("Cache-Control") != "private, no-store" {
		e.t.Fatalf("master: %d %v %s", r.Code, r.Header(), r.Body)
	}
	lines := strings.Split(strings.TrimSpace(r.Body.String()), "\n")
	url = path.Join(path.Dir(master), lines[len(lines)-1])
	r = e.get(url)
	if r.Code != http.StatusOK {
		e.t.Fatalf("media playlist: %d %s", r.Code, r.Body)
	}
	sc := bufio.NewScanner(r.Body)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "#EXTINF:"); ok {
			d, _ := strconv.ParseFloat(strings.TrimSuffix(v, ","), 64)
			durations = append(durations, d)
		}
	}
	return url, durations
}

type probedSegment struct {
	firstVideo, lastVideo float64
	keyFirst              bool
	videoCodec            string
}

// probeSegment pipes init + segment into ffprobe, like a player would
// decode them.
func (e *hlsEnv) probeSegment(media string, n int) probedSegment {
	e.t.Helper()
	dir := path.Dir(media)
	var buf bytes.Buffer
	for _, u := range []string{dir + "/init.mp4", dir + "/" + strconv.Itoa(n) + ".m4s"} {
		r := e.get(u)
		if r.Code != http.StatusOK || r.Header().Get("Content-Type") != "video/mp4" {
			e.t.Fatalf("%s: %d %s", u, r.Code, r.Body)
		}
		buf.Write(r.Body.Bytes())
	}
	f := filepath.Join(e.t.TempDir(), "seg.mp4")
	if err := os.WriteFile(f, buf.Bytes(), 0o600); err != nil {
		e.t.Fatal(err)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
		"packet=pts_time,flags:stream=codec_name", "-of", "csv=p=0", f).Output()
	if err != nil {
		e.t.Fatalf("ffprobe segment %d: %v", n, err)
	}
	p := probedSegment{firstVideo: 1e18, lastVideo: -1}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, ",")
		if len(fields) == 1 {
			p.videoCodec = fields[0]
			continue
		}
		pts, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		if pts < p.firstVideo {
			p.firstVideo, p.keyFirst = pts, strings.HasPrefix(fields[1], "K")
		}
		p.lastVideo = max(p.lastVideo, pts)
	}
	return p
}

// ffmpegRunning reports whether an FFmpeg of this session still exists.
func ffmpegRunning(t *testing.T, sessionID string) bool {
	t.Helper()
	return exec.Command("pgrep", "-f", "plaid-"+sessionID).Run() == nil
}

func waitFor(t *testing.T, within time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not within %v", what, within)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestHLSDirectStreamSeek(t *testing.T) {
	e := newHLSEnv(t)
	s := e.startHLS(e.admin, "Long", map[string]any{"profile": browser})
	if s.Method != "direct_stream" || !slices.Contains(s.Reasons, "container_not_supported") || !strings.HasSuffix(s.URL, "/master.m3u8") {
		t.Fatalf("session: %+v", s)
	}
	media, durations := e.mediaPlaylist(s.URL)
	var (
		start float64
		n     int
	)
	for n = range durations {
		if start+durations[n] > 900 {
			break
		}
		start += durations[n]
	}
	if total := sum(durations); total < 1199 || total > 1201 {
		t.Fatalf("playlist covers %.1f s", total)
	}
	for i, d := range durations[:len(durations)-1] {
		if d < 2 || d > 10 {
			t.Errorf("segment %d lasts %.3f s", i, d)
		}
	}

	// The segment at 15:00 first: served within the seek target, from a
	// run that started there.
	began := time.Now()
	if r := e.get(path.Dir(media) + "/" + strconv.Itoa(n) + ".m4s"); r.Code != http.StatusOK {
		t.Fatalf("segment %d: %d %s", n, r.Code, r.Body)
	}
	took := time.Since(began)
	t.Logf("segment at 15:00 served in %v", took)
	if took > 10*time.Second {
		t.Errorf("first seek took %v", took)
	}
	if !strings.Contains(e.logs.String(), "segment="+strconv.Itoa(n)+" ") || strings.Contains(e.logs.String(), "segment=0 ") {
		t.Errorf("the run didn't start at segment %d:\n%s", n, e.logs.String())
	}
	p := e.probeSegment(media, n)
	if p.videoCodec != "h264" || !p.keyFirst || p.firstVideo < start-0.05 || p.firstVideo > start+0.1 {
		t.Errorf("segment %d: %+v, want h264 from %.3f s on a keyframe", n, p, start)
	}
}

// TestHLSFFmpegLifetime uses a 20-minute transcode, which is still
// running (or throttled) when the checks happen.
func TestHLSFFmpegLifetime(t *testing.T) {
	e := newHLSEnv(t)
	idle := e.startHLS(e.admin, "Long", map[string]any{"profile": browser, "forceMethod": "transcode"})
	media, _ := e.mediaPlaylist(idle.URL)
	if r := e.get(path.Dir(media) + "/0.m4s"); r.Code != http.StatusOK {
		t.Fatalf("segment: %d", r.Code)
	}
	if !ffmpegRunning(t, idle.ID) {
		t.Fatal("no FFmpeg after the first segment")
	}
	// No more requests: FFmpeg stops (after 2 s here, 60 s by default),
	// and the session stays.
	waitFor(t, 10*time.Second, "idle FFmpeg stopping", func() bool { return !ffmpegRunning(t, idle.ID) })
	e.expect(e.do("GET", "/playback/"+idle.ID, nil, bearer(e.admin)), http.StatusOK, "")
	e.expect(e.do("DELETE", "/playback/"+idle.ID, nil, bearer(e.admin)), http.StatusNoContent, "")

	// DELETE: FFmpeg is gone within 6 s and the folder is removed.
	s := e.startHLS(e.admin, "Long", map[string]any{"profile": browser, "forceMethod": "transcode"})
	media, _ = e.mediaPlaylist(s.URL)
	if r := e.get(path.Dir(media) + "/1.m4s"); r.Code != http.StatusOK {
		t.Fatalf("segment: %d", r.Code)
	}
	if !ffmpegRunning(t, s.ID) {
		t.Fatal("no FFmpeg after the first segment")
	}
	// The admin list shows the run's speed, and that it's software (R29).
	waitFor(t, 5*time.Second, "the transcode speed in the admin list", func() bool {
		var list struct {
			Sessions []struct {
				Speed   *float64
				Session struct {
					ID                   string
					HardwareAcceleration *string
				}
			}
		}
		e.do("GET", "/playback", nil, bearer(e.admin)).json(&list)
		for _, a := range list.Sessions {
			if a.Session.ID == s.ID && a.Session.HardwareAcceleration == nil && a.Speed != nil && *a.Speed > 0 {
				return true
			}
		}
		return false
	})
	dir := regexp.MustCompile(`(\S+plaid-` + s.ID + `-\d+)/%d\.m4s`).FindStringSubmatch(e.logs.String())
	if dir == nil {
		t.Fatal("no session folder in the logs")
	}
	began := time.Now()
	e.expect(e.do("DELETE", "/playback/"+s.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	waitFor(t, 6*time.Second, "FFmpeg ending after DELETE", func() bool { return !ffmpegRunning(t, s.ID) })
	t.Logf("FFmpeg gone %v after DELETE", time.Since(began))
	if _, err := os.Stat(dir[1]); !os.IsNotExist(err) {
		t.Errorf("session folder still there: %v", err)
	}
	if r := e.get(media); r.Code != http.StatusNotFound {
		t.Errorf("playlist after DELETE: %d", r.Code)
	}
}

func TestHLSTranscodeWithFFmpegAsClient(t *testing.T) {
	e := newHLSEnv(t)
	s := e.startHLS(e.admin, "Frames", map[string]any{"profile": browser, "forceMethod": "transcode"})
	if s.Method != "transcode" || !slices.Contains(s.Reasons, "forced_by_client") {
		t.Fatalf("session: %+v", s)
	}
	media, durations := e.mediaPlaylist(s.URL)
	if len(durations) != 10 || durations[0] != 6 || durations[9] != 6 {
		t.Fatalf("segments: %v", durations)
	}
	for n := range 3 {
		p := e.probeSegment(media, n)
		if !p.keyFirst || p.firstVideo < float64(n*6)-0.05 || p.firstVideo > float64(n*6)+0.05 || p.lastVideo > float64(n*6+6) {
			t.Errorf("segment %d: %+v", n, p)
		}
	}

	// FFmpeg as an HLS client plays the whole master playlist.
	srv := httptest.NewServer(e.h)
	defer srv.Close()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_name,codec_type",
		"-of", "csv=p=0", srv.URL+s.URL).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "h264,video") || !strings.Contains(string(out), "aac,audio") {
		t.Fatalf("ffprobe over HLS: %v\n%s", err, out)
	}
	if err := exec.Command("ffmpeg", "-v", "error", "-i", srv.URL+s.URL, "-t", "15", "-f", "null", "-").Run(); err != nil {
		t.Fatalf("ffmpeg playing 15 s over HLS: %v", err)
	}

	// The token never reaches the logs, FFmpeg command lines included.
	token := strings.Split(strings.TrimPrefix(s.URL, "/api/v1/stream/"), "/")[0]
	if logs := e.logs.String(); strings.Contains(logs, token) || !strings.Contains(logs, "ffmpeg run started") {
		t.Error("the playback token is in the logs, or no run was logged")
	}

}

func TestHLSSubtitlesAndIndexes(t *testing.T) {
	e := newHLSEnv(t)
	s := e.startHLS(e.admin, "Subs", map[string]any{"profile": browser})
	if s.Method != "direct_stream" || !slices.Contains(s.Reasons, "audio_codec_not_supported") {
		t.Fatalf("session: %+v", s)
	}
	// The forced French file is selected (the audio is French), delivered
	// as WebVTT; every text track has a URL.
	var forced, srt string
	for _, sub := range s.Subtitles {
		if sub.URL == "" || sub.Delivery == "burn_in" {
			t.Errorf("subtitle %+v", sub)
		}
		if sub.Forced {
			forced = sub.URL
			if !sub.Selected || sub.Delivery != "external" {
				t.Errorf("forced track: %+v", sub)
			}
		} else if strings.HasSuffix(sub.URL, "/2.vtt") {
			srt = sub.URL
		}
	}
	if forced == "" || srt == "" || len(s.Subtitles) != 3 {
		t.Fatalf("subtitles: %+v", s.Subtitles)
	}
	for url, want := range map[string]string{forced: "Panneau", srt: "Bonjour"} {
		r := e.get(url)
		if r.Code != http.StatusOK || !strings.HasPrefix(r.Body.String(), "WEBVTT") || !strings.Contains(r.Body.String(), want) ||
			!strings.HasPrefix(r.Header().Get("Content-Type"), "text/vtt") {
			t.Errorf("%s: %d %s", url, r.Code, r.Body)
		}
	}
	master := e.get(s.URL).Body.String()
	if strings.Count(master, "TYPE=SUBTITLES") != 3 || !strings.Contains(master, `DEFAULT=YES,AUTOSELECT=YES,FORCED=YES,LANGUAGE="fr"`) {
		t.Errorf("master:\n%s", master)
	}
	base := strings.TrimSuffix(s.URL, "master.m3u8")
	if r := e.get(base + "subtitles/3.ass"); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "Dialogue:") {
		t.Errorf("ASS: %d %s", r.Code, r.Body)
	}
	if r := e.get(base + "subtitles/3.m3u8"); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "3.vtt") {
		t.Errorf("subtitle playlist: %d %s", r.Code, r.Body)
	}
	for _, bad := range []string{
		"subtitles/99.vtt", "subtitles/2.ass", "subtitles/-1.vtt", "subtitles/1.vtt", "video/99999.m4s",
		"video/1e3.m4s", "video/../../../../etc/passwd", "video/x.m4s", "nothing",
	} {
		if r := e.get(base + bad); r.Code < 400 {
			t.Errorf("%s: %d", bad, r.Code)
		}
	}

	// A file without a keyframe index is transcoded.
	ts := e.startHLS(e.admin, "Noindex", map[string]any{"profile": browser})
	if ts.Method != "transcode" || !slices.Contains(ts.Reasons, "keyframe_index_missing") {
		t.Errorf("TS: %+v", ts)
	}
}

func TestHLSLimitsAndChanges(t *testing.T) {
	e := newHLSEnv(t)
	bob := e.member(e.admin, "bob")
	e.expect(e.do("GET", "/settings/playback", nil, bearer(bob.Token)), http.StatusForbidden, "forbidden")
	e.expect(e.do("PATCH", "/settings/playback", map[string]any{"maxTranscodes": 1}, bearer(bob.Token)), http.StatusForbidden, "forbidden")
	e.expect(e.do("PATCH", "/settings/playback", map[string]any{"localNetworks": []string{"nope"}}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	var set struct {
		MaxTranscodes, MaxTranscodesDefault int
		EncoderPreset                       string
	}
	r := e.do("PATCH", "/settings/playback", map[string]any{"maxTranscodes": 1}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&set)
	if set.MaxTranscodes != 1 || set.MaxTranscodesDefault < 1 || set.EncoderPreset != "fast" {
		t.Fatalf("settings: %+v", set)
	}

	a := e.startHLS(e.admin, "Frames", map[string]any{"profile": browser, "forceMethod": "transcode"})
	r = e.do("POST", "/items/"+e.items["Long"]+"/playback", map[string]any{"profile": browser, "forceMethod": "transcode"}, bearer(e.admin))
	e.expect(r, http.StatusServiceUnavailable, "transcode_limit_reached")
	if r.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After")
	}
	// Direct Stream with audio conversion still starts.
	c := e.startHLS(e.admin, "Subs", map[string]any{"profile": browser})
	if c.Method != "direct_stream" {
		t.Fatalf("c: %+v", c)
	}
	// A PATCH needing a transcode is refused; the session keeps its stream.
	r = e.do("PATCH", "/playback/"+c.ID, map[string]any{"positionMs": 30000, "quality": "360p"}, bearer(e.admin))
	e.expect(r, http.StatusServiceUnavailable, "transcode_limit_reached")
	var same hlsSession
	e.do("GET", "/playback/"+c.ID, nil, bearer(e.admin)).json(&same)
	if same.URL != c.URL || e.get(c.URL).Code != http.StatusOK {
		t.Fatal("a refused change touched the session")
	}
	// An unchanged decision keeps the URL.
	r = e.do("PATCH", "/playback/"+c.ID, map[string]any{"positionMs": 30000, "quality": "original"}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&same)
	if same.URL != c.URL {
		t.Error("an unchanged decision got a new URL")
	}
	// Someone else can't change it.
	e.expect(e.do("PATCH", "/playback/"+c.ID, map[string]any{"positionMs": 0}, bearer(bob.Token)), http.StatusNotFound, "playback_session_not_found")

	// Once the slot is free, the change goes through with a new URL; the
	// old one dies at once.
	e.expect(e.do("DELETE", "/playback/"+a.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	var changed hlsSession
	r = e.do("PATCH", "/playback/"+c.ID, map[string]any{"positionMs": 30000, "quality": "360p"}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&changed)
	if changed.Method != "transcode" || changed.URL == c.URL || changed.Quality != "360p" {
		t.Fatalf("changed: %+v", changed)
	}
	if code := e.get(c.URL).Code; code != http.StatusNotFound {
		t.Errorf("old URL: %d", code)
	}
	media, _ := e.mediaPlaylist(changed.URL)
	if p := e.probeSegment(media, 5); p.firstVideo < 29.95 || p.firstVideo > 30.05 {
		t.Errorf("segment at the new position: %+v", p)
	}

	// A folder over its cap refuses new streams until it fits again.
	e.expect(e.do("PATCH", "/settings/playback", map[string]any{"transcodeDirMaxBytes": 1, "maxTranscodes": nil}, bearer(e.admin)), http.StatusOK, "")
	waitFor(t, 5*time.Second, "storage full", func() bool {
		return e.do("POST", "/items/"+e.items["Long"]+"/playback", map[string]any{"profile": browser}, bearer(e.admin)).Code == http.StatusServiceUnavailable
	})
	e.expect(e.do("POST", "/items/"+e.items["Long"]+"/playback", map[string]any{"profile": browser}, bearer(e.admin)),
		http.StatusServiceUnavailable, "transcode_storage_full")
	e.expect(e.do("DELETE", "/playback/"+c.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	waitFor(t, 5*time.Second, "storage back under its cap", func() bool {
		return e.do("POST", "/items/"+e.items["Long"]+"/playback", map[string]any{"profile": browser}, bearer(e.admin)).Code == http.StatusCreated
	})
}

func TestHLSDiesWithItsDeviceSession(t *testing.T) {
	e := newHLSEnv(t)
	phone := e.login("ada", adminPassword)
	s := e.startHLS(phone.Token, "Long", map[string]any{"profile": browser, "forceMethod": "transcode"})
	media, _ := e.mediaPlaylist(s.URL)
	if r := e.get(path.Dir(media) + "/0.m4s"); r.Code != http.StatusOK {
		t.Fatalf("segment: %d", r.Code)
	}
	if !ffmpegRunning(t, s.ID) {
		t.Fatal("no FFmpeg after the first segment")
	}
	e.expect(e.do("DELETE", "/me/sessions/"+phone.Session.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	if r := e.get(media); r.Code != http.StatusNotFound || !strings.Contains(r.Body.String(), "playback_session_not_found") {
		t.Errorf("playlist after the sign-out: %d %s", r.Code, r.Body)
	}
	waitFor(t, 6*time.Second, "FFmpeg ending with its session", func() bool { return !ffmpegRunning(t, s.ID) })
}

func sum(xs []float64) float64 {
	var t float64
	for _, x := range xs {
		t += x
	}
	return t
}
