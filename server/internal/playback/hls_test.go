package playback

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/playback/testmedia"
	"github.com/gh-prim/plaid/server/internal/settings"
)

// syncBuffer is a log sink tests can read while the service writes.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// hlsService is a service with FFmpeg, a transcode folder and short
// timings, without a library: tests add sessions to its manager directly.
func hlsService(t *testing.T, timings Timings, free uint64) (*Service, *syncBuffer) {
	t.Helper()
	path := testmedia.FFmpeg(t)
	logs := &syncBuffer{}
	database, err := db.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	s := New(Options{
		DB: database, Settings: settings.New(database),
		FFmpeg: &FFmpeg{Path: path}, TranscodeDir: filepath.Join(t.TempDir(), "transcodes"), CacheDir: t.TempDir(),
		Free: func(string) (uint64, error) { return free, nil }, Timings: &timings,
		Log: slog.New(slog.NewTextHandler(logs, nil)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	t.Cleanup(func() { cancel(); s.Close() })
	return s, logs
}

// addSession registers a session for a decision, with its stream, the
// way StartSession does once the decision is made.
func addSession(t *testing.T, s *Service, d Decision) Session {
	t.Helper()
	slot, err := s.admit(d)
	if err != nil {
		t.Fatal(err)
	}
	sess := s.manager.Add(Session{UserID: "u", ProfileID: "p", ItemID: "i", Decision: d})
	s.newStream(sess, 0, slot)
	return sess
}

var fastTimings = Timings{
	WaitAhead: 3, SegmentWait: 20 * time.Second, ThrottleAhead: 120_000, ResumeAhead: 60_000,
	IdleStop: time.Hour, SlotIdle: time.Hour, KeepBehind: 300_000, Tick: 50 * time.Millisecond,
}

func TestSegmentRequests(t *testing.T) {
	s, logs := hlsService(t, fastTimings, 100<<30)
	m := longMedia(t)
	sess := addSession(t, s, directStreamOf(m))
	st := s.stream(sess.ID)
	ctx := context.Background()

	// Ten players asking for the same segment at once start FFmpeg once.
	var (
		wg     sync.WaitGroup
		failed atomic.Int32
	)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if files, err := s.Segment(ctx, sess, 150); err != nil || len(files) == 0 {
				failed.Add(1)
			}
		}()
	}
	wg.Wait()
	if failed.Load() > 0 || st.runs != 1 {
		t.Fatalf("%d failed requests, %d runs", failed.Load(), st.runs)
	}
	// It started at the segment asked for (15:00), not at 0.
	if want := "segment=150 "; !strings.Contains(logs.String(), want) {
		t.Fatalf("no run started at segment 150:\n%s", logs.String())
	}
	if _, err := os.Stat(st.initPath()); err != nil {
		t.Fatal("no init segment")
	}

	// The next few segments come from the same run.
	for n := 151; n <= 154; n++ {
		if _, err := s.Segment(ctx, sess, n); err != nil {
			t.Fatalf("segment %d: %v", n, err)
		}
	}
	if st.runs != 1 {
		t.Fatalf("%d runs for consecutive segments", st.runs)
	}
	// Far ahead: a new run there. Back before anything on disk: another.
	if _, err := s.Segment(ctx, sess, 190); err != nil || st.runs != 2 {
		t.Fatalf("seek ahead: %v, %d runs", err, st.runs)
	}
	if _, err := s.Segment(ctx, sess, 20); err != nil || st.runs != 3 {
		t.Fatalf("seek back: %v, %d runs", err, st.runs)
	}
	// Segment 152 is still on disk: served without a run.
	if _, err := s.Segment(ctx, sess, 152); err != nil || st.runs != 3 {
		t.Fatalf("segment on disk: %v, %d runs", err, st.runs)
	}
	// Out of range.
	for _, n := range []int{-1, len(st.layout.Segments)} {
		if _, err := s.Segment(ctx, sess, n); err != ErrNoSuchSegment {
			t.Errorf("segment %d: %v", n, err)
		}
	}
}

func TestThrottle(t *testing.T) {
	timings := fastTimings
	timings.ThrottleAhead, timings.ResumeAhead = 60_000, 30_000
	s, _ := hlsService(t, timings, 100<<30)
	m := longMedia(t)
	sess := addSession(t, s, transcodeOf(m))
	st := s.stream(sess.ID)
	ctx := context.Background()
	if _, err := s.Segment(ctx, sess, 0); err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	run := st.run
	st.mu.Unlock()

	// FFmpeg races ahead, then pauses once a minute ahead of segment 0.
	deadline := time.Now().Add(time.Minute)
	for !run.Paused() {
		if time.Now().After(deadline) {
			t.Fatalf("never paused; produced %d", run.Produced())
		}
		time.Sleep(20 * time.Millisecond)
	}
	at := run.Produced()
	if ahead := st.layout.PieceStartMs[at]; ahead < 60_000 || ahead > 90_000 {
		t.Errorf("paused %d ms ahead", ahead)
	}
	// A request near the end of what's produced resumes it.
	if _, err := s.Segment(ctx, sess, at-1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if run.Paused() {
		t.Fatal("still paused with the client 6 s behind it")
	}
	if !run.WaitPiece(ctx, at+3) {
		t.Fatal("resumed run made no progress")
	}

}

func TestIdleStop(t *testing.T) {
	timings := fastTimings
	timings.IdleStop = time.Second
	s, _ := hlsService(t, timings, 100<<30)
	sess := addSession(t, s, directStreamOf(longMedia(t)))
	st := s.stream(sess.ID)
	ctx := context.Background()
	if _, err := s.Segment(ctx, sess, 0); err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	run := st.run
	st.mu.Unlock()
	// No requests for 1 s: the run stops, the session stays.
	select {
	case <-run.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("idle run still running")
	}
	if _, ok := s.manager.Activity(sess.ID); !ok {
		t.Fatal("the session ended with its run")
	}
	// The next request starts a run again, where it's asked for.
	if _, err := s.Segment(ctx, sess, 100); err != nil || st.runs != 2 {
		t.Fatalf("after the idle stop: %v, %d runs", err, st.runs)
	}
}

func TestSlots(t *testing.T) {
	s, _ := hlsService(t, fastTimings, 100<<30)
	s.setConfig(func(c *Config) { c.MaxTranscodes = 1 })
	m := longMedia(t)
	a := addSession(t, s, transcodeOf(m))
	if _, err := s.admit(transcodeOf(m)); err != ErrTranscodeLimit {
		t.Fatalf("second transcode: %v", err)
	}
	// Direct Stream with audio conversion takes no slot.
	if slot, err := s.admit(directStreamOf(m)); err != nil || slot {
		t.Fatalf("direct stream: %v, slot %v", err, slot)
	}
	// Ending the session frees its slot.
	if err := s.manager.End(principalOf(a), a.ID, nil); err != nil {
		t.Fatal(err)
	}
	if slot, err := s.admit(transcodeOf(m)); err != nil || !slot {
		t.Fatalf("after the end: %v", err)
	}
}

func TestSlotReleasedAfterInactivity(t *testing.T) {
	timings := fastTimings
	timings.SlotIdle, timings.IdleStop = 300*time.Millisecond, 100*time.Millisecond
	s, _ := hlsService(t, timings, 100<<30)
	s.setConfig(func(c *Config) { c.MaxTranscodes = 1 })
	m := longMedia(t)
	a := addSession(t, s, transcodeOf(m))
	if _, err := s.Segment(context.Background(), a, 0); err != nil {
		t.Fatal(err)
	}
	// Paused a while: the run stops, then the slot goes.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if slot, _ := s.admit(transcodeOf(m)); slot {
			s.releaseSlot()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("slot never released")
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Coming back with the slot taken by someone else fails cleanly.
	b := addSession(t, s, transcodeOf(m))
	if _, err := s.Segment(context.Background(), a, 1); err != ErrTranscodeLimit {
		t.Fatalf("resuming without a slot: %v", err)
	}
	_ = b
}

func TestStorageCap(t *testing.T) {
	s, _ := hlsService(t, fastTimings, 100<<30)
	m := longMedia(t)
	a := addSession(t, s, directStreamOf(m))
	ctx := context.Background()
	for n := 0; n < 12; n++ {
		if _, err := s.Segment(ctx, a, n); err != nil {
			t.Fatal(err)
		}
	}
	st := s.stream(a.ID)
	size := dirSize(st.dir)
	// A cap just under what's there: the oldest segments go, not the
	// current one, and the folder isn't "full".
	s.setConfig(func(c *Config) { c.TranscodeDirMaxBytes = size - 1 })
	s.housekeep()
	if _, ok := st.onDisk(st.layout.Segments[0]); ok {
		t.Error("segment 0 survived the cap")
	}
	if _, ok := st.onDisk(st.layout.Segments[11]); !ok {
		t.Error("the current segment was deleted")
	}
	if s.storageFull() {
		t.Error("full after deleting old segments")
	}
	// A cap nothing fits under: new streams are refused.
	s.setConfig(func(c *Config) { c.TranscodeDirMaxBytes = 1 })
	s.housekeep()
	if !s.storageFull() {
		t.Fatal("not full")
	}
	if _, err := s.admit(directStreamOf(m)); err != ErrTranscodeStorage {
		t.Fatalf("new stream with a full folder: %v", err)
	}
	// Ending the session empties its folder; the folder fits again.
	if err := s.manager.End(principalOf(a), a.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(st.dir); !os.IsNotExist(err) {
		t.Errorf("session folder still there: %v", err)
	}
	s.housekeep()
	if s.storageFull() {
		t.Error("still full with nothing in the folder")
	}
}

func TestStoragePurgeAtStart(t *testing.T) {
	dir := t.TempDir()
	mine := filepath.Join(dir, "plaid-0193a5b2-7c1d-7e2f-8a9b-0c1d2e3f4a5b-0")
	other := filepath.Join(dir, "plaid-photos")
	for _, d := range []string{mine, other} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "1.m4s"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	st := &Storage{Dir: dir, Free: func(string) (uint64, error) { return 200 << 30, nil }}
	purged, err := st.Open()
	if err != nil || purged != 1 {
		t.Fatalf("purged %d: %v", purged, err)
	}
	if _, err := os.Stat(mine); !os.IsNotExist(err) {
		t.Error("a session folder survived the start")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("a folder PLAID didn't make was deleted")
	}
	if st.DefaultCap() != 10<<30 {
		t.Errorf("cap %d with 200 GiB free", st.DefaultCap())
	}
	// R27's examples: 3 GiB free → the 1 GiB floor; 200 GiB → 10 GiB.
	if DefaultStorageCap(3<<30) != 1<<30 || DefaultStorageCap(200<<30) != 10<<30 || DefaultStorageCap(20<<30) != 4<<30 {
		t.Error("default cap formula")
	}
}

func TestSubtitleFiles(t *testing.T) {
	s, _ := hlsService(t, fastTimings, 100<<30)
	path := testmedia.Clip(t, testmedia.MKV)
	ext := filepath.Join(testmedia.Dir(t), testmedia.ExtSRT)
	m := Media{FileID: "subs", Path: path, Container: "mkv", DurationMs: 60_000, Subtitles: []Subtitle{
		{Index: 2, Codec: "subrip", Language: "fre"},
		{Index: 3, Codec: "ass", Language: "eng"},
		{Index: 1000, Codec: "subrip", Language: "fre", Forced: true, ExternalPath: ext},
		{Index: 1001, Codec: "hdmv_pgs_subtitle", Image: true},
	}}
	sess := Session{Decision: Decision{Method: DirectStream, Media: m}}
	ctx := context.Background()
	read := func(index int, format string) string {
		t.Helper()
		p, err := s.SubtitleFile(ctx, sess, index, format)
		if err != nil {
			t.Fatalf("%d.%s: %v", index, format, err)
		}
		b, _ := os.ReadFile(p) //nolint:gosec // test output
		return string(b)
	}
	for _, c := range []struct {
		index  int
		format string
		want   []string
	}{
		{2, SubtitleVTT, []string{"WEBVTT", "00:01.000 --> 00:04.000", "Bonjour", "00:30.500 --> 00:33.250"}},
		{3, SubtitleVTT, []string{"WEBVTT", "00:02.000 --> 00:05.500", "Hello", "00:40.000 --> 00:42.000"}},
		{3, SubtitleASS, []string{"[Events]", "Dialogue: 0,0:00:02.00,0:00:05.50"}},
		{1000, SubtitleVTT, []string{"WEBVTT", "00:00.500 --> 00:02.000", "Panneau"}},
	} {
		got := read(c.index, c.format)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%d.%s has no %q:\n%s", c.index, c.format, w, got)
			}
		}
	}
	// Cached: the second read is the same file.
	p1, _ := s.SubtitleFile(ctx, sess, 2, SubtitleVTT)
	st1, _ := os.Stat(p1)
	p2, _ := s.SubtitleFile(ctx, sess, 2, SubtitleVTT)
	if st2, _ := os.Stat(p2); p1 != p2 || !st1.ModTime().Equal(st2.ModTime()) && st2.ModTime().Before(st1.ModTime()) {
		t.Error("not cached")
	}
	for _, c := range []struct {
		index  int
		format string
	}{{99, SubtitleVTT}, {1001, SubtitleVTT}, {2, SubtitleASS}, {2, "srt"}} {
		if _, err := s.SubtitleFile(ctx, sess, c.index, c.format); err != ErrNoSuchSubtitle {
			t.Errorf("%d.%s: %v", c.index, c.format, err)
		}
	}
}

func TestMasterPlaylist(t *testing.T) {
	m := goldenMedia()
	m.Video.Profile, m.Video.Level = "High", 41
	ds := Session{Decision: Decision{Method: DirectStream, Media: m, Audio: &m.Audio[0], AudioCodec: "aac", AudioChannels: 2,
		Subtitle: &m.Subtitles[3], SubtitleDelivery: DeliveryExternal}}
	want := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="fre",DEFAULT=NO,AUTOSELECT=YES,FORCED=NO,LANGUAGE="fr",URI="subtitles/3.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="eng",DEFAULT=NO,AUTOSELECT=YES,FORCED=NO,LANGUAGE="en",URI="subtitles/5.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="fre (forced)",DEFAULT=YES,AUTOSELECT=YES,FORCED=YES,LANGUAGE="fr",URI="subtitles/1000.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=12000000,RESOLUTION=1920x1080,CODECS="avc1.640029,mp4a.40.2",SUBTITLES="subs"
video/index.m3u8
`
	if got := MasterPlaylist(ds); got != want {
		t.Errorf("direct stream:\n%s\nwant:\n%s", got, want)
	}

	tc := Session{Decision: Decision{Method: Transcode, Media: Media{Video: &library.Stream{Codec: "hevc"}, Audio: m.Audio},
		Audio: &m.Audio[1], AudioCopy: true, Width: 3840, Height: 2160, VideoBitrate: 20_000_000}}
	want = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-STREAM-INF:BANDWIDTH=20640000,RESOLUTION=3840x2160,CODECS="avc1.640033,ac-3"
video/index.m3u8
`
	if got := MasterPlaylist(tc); got != want {
		t.Errorf("transcode:\n%s\nwant:\n%s", got, want)
	}
}

func TestSettingsValidation(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	store := settings.New(d)
	for _, c := range []struct {
		u     ConfigUpdate
		field string
	}{
		{ConfigUpdate{MaxTranscodes: ptr(0)}, "maxTranscodes"},
		{ConfigUpdate{MaxTranscodes: ptr(33)}, "maxTranscodes"},
		{ConfigUpdate{RemoteMaxBitrate: ptr(int64(-1))}, "remoteMaxBitrate"},
		{ConfigUpdate{LocalNetworks: &[]string{"10.0.0.0/8", "not-a-cidr"}}, "localNetworks"},
		{ConfigUpdate{TranscodeDirMaxBytes: ptr(int64(0))}, "transcodeDirMaxBytes"},
		{ConfigUpdate{EncoderPreset: ptr("ultra")}, "encoderPreset"},
	} {
		var fe *FieldError
		if err := SaveConfig(ctx, store, c.u); !errors.As(err, &fe) || fe.Field != c.field {
			t.Errorf("%+v: %v, want a %s error", c.u, err, c.field)
		}
	}
	// A valid change, then a reset to the default.
	err = SaveConfig(ctx, store, ConfigUpdate{MaxTranscodes: ptr(3), LocalNetworks: &[]string{"100.64.1.2/10"},
		EncoderPreset: ptr(PresetQuality), TranscodeDirMaxBytes: ptr(int64(5 << 30))})
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(ctx, store)
	if err != nil || c.MaxTranscodes != 3 || c.EncoderPreset != PresetQuality || c.TranscodeDirMaxBytes != 5<<30 ||
		len(c.LocalNetworks) != 1 || c.LocalNetworks[0].String() != "100.64.0.0/10" {
		t.Fatalf("saved: %+v, %v", c, err)
	}
	if err := SaveConfig(ctx, store, ConfigUpdate{ResetMaxTranscodes: true, ResetTranscodeDirMaxBytes: true}); err != nil {
		t.Fatal(err)
	}
	if c, _ := LoadConfig(ctx, store); c.MaxTranscodes != 0 || c.TranscodeDirMaxBytes != 0 || c.EncoderPreset != PresetQuality {
		t.Fatalf("after reset: %+v", c)
	}
	if DefaultMaxTranscodes(4, false) != 1 || DefaultMaxTranscodes(8, false) != 2 || DefaultMaxTranscodes(2, true) != 4 {
		t.Error("default max transcodes")
	}
}

func ptr[T any](v T) *T { return &v }

// setConfig changes the settings in effect, as a saved change would.
func (s *Service) setConfig(change func(*Config)) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	change(&s.cfg)
}

func principalOf(s Session) auth.Principal { return auth.Principal{UserID: s.UserID, ProfileID: s.ProfileID} }
