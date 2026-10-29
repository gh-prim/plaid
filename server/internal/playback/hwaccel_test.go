package playback

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gh-prim/plaid/server/internal/clientinfo"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/playback/testmedia"
)

// fakeFFmpeg is testdata/fakeffmpeg.sh in front of the real FFmpeg, with
// one render node, failing on the given rules.
func fakeFFmpeg(t *testing.T, fail string) *FFmpeg {
	t.Helper()
	t.Setenv("REAL_FFMPEG", testmedia.FFmpeg(t))
	t.Setenv("FAKE_FFMPEG_FAIL", fail)
	nodes := renderNodes
	renderNodes = func() []string { return []string{"/dev/dri/renderD128"} }
	t.Cleanup(func() { renderNodes = nodes })
	path, err := filepath.Abs(filepath.Join("testdata", "fakeffmpeg.sh"))
	if err != nil {
		t.Fatal(err)
	}
	ff, err := FindFFmpeg(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return ff
}

func TestProbeHardware(t *testing.T) {
	ctx := context.Background()
	allH264 := "h264_vaapi h264_qsv h264_nvenc h264_v4l2m2m h264_videotoolbox"
	for _, c := range []struct {
		name, fail, choice string
		active             string
		encoders, decoders []string
		reason             string
		err                error
	}{
		{name: "auto keeps the first that works", choice: HWAuto, active: HWVAAPI,
			encoders: []string{"h264_vaapi", "hevc_vaapi"}, decoders: []string{"h264", "hevc"}},
		{name: "auto moves on when VAAPI fails", fail: "h264_vaapi", choice: HWAuto, active: HWQSV,
			encoders: []string{"h264_qsv"}, decoders: []string{"h264"}},
		{name: "auto falls back to software", fail: allH264, choice: HWAuto, reason: HWReasonNotFound},
		{name: "an explicit choice that fails is refused", fail: "h264_vaapi", choice: HWVAAPI, reason: HWReasonNotFound, err: ErrHWAccelUnavailable},
		{name: "HEVC encoding is optional", fail: "hevc_vaapi", choice: HWVAAPI, active: HWVAAPI,
			encoders: []string{"h264_vaapi"}, decoders: []string{"h264"}},
		{name: "a failed decode keeps the encoder", fail: "vaapi+null", choice: HWVAAPI, active: HWVAAPI,
			encoders: []string{"h264_vaapi", "hevc_vaapi"}},
		{name: "V4L2 M2M decodes in software", choice: HWV4L2M2M, active: HWV4L2M2M, encoders: []string{"h264_v4l2m2m"}},
		{name: "none tests nothing", choice: HWNone},
	} {
		t.Run(c.name, func(t *testing.T) {
			ff := fakeFFmpeg(t, c.fail)
			st, err := ProbeHardware(ctx, ff, c.choice, "")
			if !errors.Is(err, c.err) {
				t.Fatalf("err = %v, want %v", err, c.err)
			}
			if st.Configured != c.choice || st.Active != c.active || st.DisabledReason != c.reason ||
				!slices.Equal(st.Encoders, c.encoders) || !slices.Equal(st.Decoders, c.decoders) {
				t.Fatalf("status = %+v", st)
			}
			if c.active == HWVAAPI && st.Device != "/dev/dri/renderD128" {
				t.Errorf("device = %q", st.Device)
			}
		})
	}

	t.Run("VAAPI without a render node", func(t *testing.T) {
		ff := fakeFFmpeg(t, "")
		renderNodes = func() []string { return nil }
		if _, err := ProbeHardware(ctx, ff, HWVAAPI, ""); !errors.Is(err, ErrHWAccelUnavailable) {
			t.Fatalf("err = %v", err)
		}
		if st, err := ProbeHardware(ctx, ff, HWVAAPI, "/dev/dri/renderD129"); err != nil || st.Device != "/dev/dri/renderD129" {
			t.Fatalf("explicit device: %+v, %v", st, err)
		}
	})
}

func TestHardwareSettings(t *testing.T) {
	ctx := context.Background()
	s, _ := hlsServiceWith(t, fastTimings, fakeFFmpeg(t, "h264_vaapi"))
	// An explicit accelerator that fails its test is refused, and nothing
	// is saved (acceptance: 422 hw_accel_unavailable).
	if _, err := s.SaveSettings(ctx, ConfigUpdate{HWAccel: ptr(HWVAAPI)}); !errors.Is(err, ErrHWAccelUnavailable) {
		t.Fatalf("vaapi: %v", err)
	}
	if v := s.Settings(); v.HWAccel != HWAuto || v.Hardware.Active != "" {
		t.Fatalf("after a refused change: %+v", v)
	}
	// Another one works and is used; the transcode default goes to 4.
	v, err := s.SaveSettings(ctx, ConfigUpdate{HWAccel: ptr(HWQSV), HWDevice: ptr("/dev/dri/renderD129")})
	if err != nil || v.HWAccel != HWQSV || v.HWDevice != "/dev/dri/renderD129" || v.Hardware.Active != HWQSV || v.MaxTranscodesDefault != 4 {
		t.Fatalf("qsv: %+v, %v", v, err)
	}
	// auto with nothing working is accepted: software, with the reason.
	s.o.FFmpeg = fakeFFmpeg(t, "h264_vaapi h264_qsv h264_nvenc h264_v4l2m2m h264_videotoolbox")
	v, err = s.SaveSettings(ctx, ConfigUpdate{HWAccel: ptr(HWAuto), ResetHWDevice: true})
	if err != nil || v.Hardware.Active != "" || v.Hardware.DisabledReason != HWReasonNotFound || v.HWDevice != "" {
		t.Fatalf("auto: %+v, %v", v, err)
	}
	for _, bad := range []ConfigUpdate{{HWAccel: ptr("cuda")}, {HWDevice: ptr("dri/renderD128")}, {HWDevice: ptr("/dev/../dri")}} {
		var fe *FieldError
		if _, err := s.SaveSettings(ctx, bad); !errors.As(err, &fe) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}

// TestHardwareFallback: a hardware run that fails early restarts in
// software where it stopped; the third failure in 10 minutes disables
// hardware (R24).
func TestHardwareFallback(t *testing.T) {
	ctx := context.Background()
	ff := fakeFFmpeg(t, "h264_vaapi+hls")
	s, logs := hlsServiceWith(t, fastTimings, ff)
	rec := &fakeProblems{}
	s.o.Problems = rec
	st, err := ProbeHardware(ctx, ff, HWAuto, "")
	if err != nil || st.Active != HWVAAPI {
		t.Fatalf("probe: %+v, %v", st, err)
	}
	s.setHardware(st)
	m := mp4Media(t)
	for i := 1; i <= hwMaxFailures; i++ {
		d := transcodeOf(m)
		if d.HWAccel = s.accelFor(d); d.HWAccel != HWVAAPI {
			t.Fatalf("session %d: accelerator %q", i, d.HWAccel)
		}
		sess := addSession(t, s, d)
		files, err := s.Segment(ctx, sess, 1)
		if err != nil || len(files) == 0 {
			t.Fatalf("session %d: segment after the fallback: %v\n%s", i, err, logs.String())
		}
		got, _ := s.manager.Get(principalOf(sess), sess.ID)
		if got.Decision.HWAccel != "" {
			t.Fatalf("session %d still reports %q", i, got.Decision.HWAccel)
		}
		if run := s.stream(sess.ID).run; run == nil || run.Spec.HW != nil || !strings.Contains(strings.Join(run.Spec.Args(), " "), "libx264") {
			t.Fatalf("session %d: the run after the fallback isn't software", i)
		}
	}
	if n := strings.Count(logs.String(), "code=hw_fallback"); n != hwMaxFailures {
		t.Fatalf("%d fallbacks logged:\n%s", n, logs.String())
	}
	if hw := s.Hardware(); hw.Active != "" || hw.DisabledReason != HWReasonRepeatedFailures {
		t.Fatalf("after %d failures: %+v", hwMaxFailures, hw)
	}
	if a := s.accelFor(transcodeOf(m)); a != "" {
		t.Fatalf("new transcodes still use %q", a)
	}
	// A settings change tests again and turns hardware back on.
	if v, err := s.SaveSettings(ctx, ConfigUpdate{HWAccel: ptr(HWAuto)}); err != nil || v.Hardware.Active != HWVAAPI {
		t.Fatalf("settings change: %+v, %v", v.Hardware, err)
	}
	// F007 R29: every software restart, one fallback, resolved by the test.
	want := append(slices.Repeat([]string{"report ffmpeg_restarted"}, hwMaxFailures),
		"report hw_fallback vaapi", "resolve hw_fallback vaapi")
	// The fallback is reported inside the third restart, before it.
	want[hwMaxFailures-1], want[hwMaxFailures] = want[hwMaxFailures], want[hwMaxFailures-1]
	if !slices.Equal(rec.codes(), want) {
		t.Errorf("problems = %q, want %q", rec.codes(), want)
	}
}

// fakeProblems records reported problems (F007 R29).
type fakeProblems struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeProblems) add(verb, code string, p map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.TrimSpace(verb+" "+code+" "+p["accel"]))
}
func (f *fakeProblems) Report(code string, p map[string]string)  { f.add("report", code, p) }
func (f *fakeProblems) Resolve(code string, p map[string]string) { f.add("resolve", code, p) }
func (f *fakeProblems) codes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// TestRetestHardware: a retest replaces a fallback, and only one runs at
// a time (F007 R28).
func TestRetestHardware(t *testing.T) {
	ctx := context.Background()
	s := New(Options{})
	release, started := make(chan struct{}), make(chan struct{})
	s.probeHardware = func(_ context.Context, _ *FFmpeg, choice, _ string) (HWStatus, error) {
		close(started)
		<-release
		return HWStatus{Configured: choice, Active: HWVAAPI}, nil
	}
	s.hw = HWStatus{Configured: HWAuto, DisabledReason: HWReasonRepeatedFailures}
	s.hwFellBack = HWVAAPI
	rec := &fakeProblems{}
	s.o.Problems = rec

	done := make(chan HWStatus)
	go func() {
		st, err := s.RetestHardware(ctx)
		if err != nil {
			t.Error(err)
		}
		done <- st
	}()
	<-started
	if _, err := s.RetestHardware(ctx); !errors.Is(err, ErrHWTestRunning) {
		t.Errorf("second test: %v, want ErrHWTestRunning", err)
	}
	close(release)
	if st := <-done; st.Active != HWVAAPI || st.DisabledReason != "" {
		t.Errorf("after the test: %+v", st)
	}
	if got := rec.codes(); !slices.Equal(got, []string{"resolve hw_fallback vaapi"}) {
		t.Errorf("problems = %q", got)
	}
}

// TestHardwareTranscode runs a real transcode on the machine's own
// accelerator when it has one (VideoToolbox on a Mac, VAAPI on an Intel
// box); CI has none and skips.
func TestHardwareTranscode(t *testing.T) {
	ctx := context.Background()
	ff, err := FindFFmpeg(ctx, testmedia.FFmpeg(t))
	if err != nil {
		t.Fatal(err)
	}
	st, err := ProbeHardware(ctx, ff, HWAuto, "")
	if err != nil || st.Active == "" {
		t.Skipf("no working accelerator here (%+v, %v)", st, err)
	}
	s, logs := hlsServiceWith(t, fastTimings, ff)
	s.setHardware(st)
	m := mp4Media(t)
	d := transcodeOf(m)
	d.HWAccel = s.accelFor(d)
	sess := addSession(t, s, d)
	init, err := s.Init(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	files, err := s.Segment(ctx, sess, 1)
	if err != nil {
		t.Fatalf("segment 1: %v\n%s", err, logs.String())
	}
	if strings.Contains(logs.String(), "hw_fallback") {
		t.Fatalf("fell back to software:\n%s", logs.String())
	}
	pkts := probe(t, init, files...)
	lo, hi := ptsRange(pkts, true)
	if i := firstVideo(pkts); i < 0 || !pkts[i].key || lo < 5.9 || lo > 6.1 || hi > 12.1 {
		t.Fatalf("segment 1 on %s: video %.3f–%.3f, starts on a keyframe: %v", st.Active, lo, hi, i >= 0 && pkts[i].key)
	}
	t.Logf("%s: encoders %v, decoders %v", st.Active, st.Encoders, st.Decoders)
}

// TestTonemap: the HDR10 clip transcoded for an SDR client comes out as
// BT.709, unflagged (R23, plan step 26).
func TestTonemap(t *testing.T) {
	ctx := context.Background()
	ff, err := FindFFmpeg(ctx, testmedia.FFmpeg(t))
	if err != nil {
		t.Fatal(err)
	}
	if !ff.HasFilter("zscale") {
		t.Skip("this FFmpeg has no zscale (no libzimg): HDR is flagged colors_may_be_wrong instead")
	}
	if !testmedia.HasEncoder(t, "libx265") {
		t.Skip("no libx265 for the HDR10 clip")
	}
	s, _ := hlsServiceWith(t, fastTimings, ff)
	m := Media{
		FileID: "hdr", Path: testmedia.Clip(t, testmedia.HEVCHDR), Container: "mkv", DurationMs: 60_000, Bitrate: 400_000, HDR: library.HDR10,
		Video: &library.Stream{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 640, Height: 360, BitDepth: 10},
		Audio: []library.Stream{{Index: 1, Type: "audio", Codec: "aac", Channels: 1, Default: true}},
	}
	lim, err := s.limits(ctx, clientinfo.Info{})
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decide(m, Request{}, Profile{Protocols: []string{ProtocolHLS}, Video: []VideoCap{{Codec: "h264"}},
		Audio: []AudioCap{{Codec: "aac", MaxChannels: 2}}}, lim)
	if err != nil || d.Method != Transcode || !d.Tonemap || len(d.Warnings) != 0 {
		t.Fatalf("decision: %s tonemap=%v warnings=%v, %v", d.Method, d.Tonemap, d.Warnings, err)
	}
	sess := addSession(t, s, d)
	init, err := s.Init(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	files, err := s.Segment(ctx, sess, 0)
	if err != nil {
		t.Fatal(err)
	}
	joined := filepath.Join(t.TempDir(), "joined.mp4")
	cat := exec.Command("sh", append([]string{"-c", `cat "$@" > "$0"`, joined, init}, files...)...)
	if out, err := cat.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
		"stream=pix_fmt,color_transfer,color_primaries", "-of", "default=nw=1", joined).Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pix_fmt=yuv420p", "color_transfer=bt709", "color_primaries=bt709"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("no %s in:\n%s", want, out)
		}
	}
}
