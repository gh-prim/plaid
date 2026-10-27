//go:build perf

package playback

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/playback/keyframes"
	"github.com/gh-prim/plaid/server/internal/playback/testmedia"
)

// TestPerf measures the spec's NFRs on generated clips (plan step 30):
// start (session to init and first segment served), seek (a far segment
// served) and transcode speed, per method. Not part of make test: run it
// with -tags perf, and in the throttled container for Pi-like numbers
// (CLAUDE.md). It reports; it fails only when something breaks.
func TestPerf(t *testing.T) {
	ctx := context.Background()
	ff, err := FindFFmpeg(ctx, testmedia.FFmpeg(t))
	if err != nil {
		t.Fatal(err)
	}
	timings := DefaultTimings
	timings.Tick = 100 * time.Millisecond
	s, _ := hlsServiceWith(t, timings, ff)
	media := func(clip, codec string, w, h, depth int, bitrate, durationMs int64) Media {
		path := testmedia.Clip(t, clip)
		kf, err := keyframes.ReadFile(path, "mkv")
		if err != nil {
			t.Fatal(err)
		}
		return Media{FileID: clip, Path: path, Container: "mkv", DurationMs: durationMs, Bitrate: bitrate,
			Video:            &library.Stream{Index: 0, Type: "video", Codec: codec, Width: w, Height: h, BitDepth: depth},
			Audio:            []library.Stream{{Index: 1, Type: "audio", Codec: "aac", Channels: 1, Default: true}},
			HasKeyframeIndex: true, Keyframes: kf}
	}
	hd := media(testmedia.Perf1080p, "h264", 1920, 1080, 8, 8_000_000, 60_000)
	to720 := func(hw *HWConfig) Decision {
		d := Decision{Method: Transcode, Media: hd, Audio: &hd.Audio[0], AudioCodec: "aac", AudioChannels: 2,
			Width: 1280, Height: 720, VideoBitrate: 4_000_000, VideoCodec: "h264", Quality: "720p"}
		if hw != nil {
			d.HWAccel = hw.Accel
		}
		return d
	}

	type result struct {
		name              string
		start, seek       time.Duration
		speed             float64
		startNFR, seekNFR time.Duration
		speedNFR          float64
	}
	var results []result
	measure := func(name string, d Decision, hw *HWConfig, seekTo int, startNFR, seekNFR time.Duration, speedNFR float64) {
		s.setHardware(HWStatus{})
		if hw != nil {
			s.setHardware(HWStatus{Active: hw.Accel, Device: hw.Device, Decoders: hw.Decoders, Tonemap: hw.Tonemap})
		}
		began := time.Now()
		sess := addSession(t, s, d)
		if _, err := s.Init(ctx, sess); err != nil {
			t.Fatalf("%s: init: %v", name, err)
		}
		if _, err := s.Segment(ctx, sess, 0); err != nil {
			t.Fatalf("%s: segment 0: %v", name, err)
		}
		r := result{name: name, start: time.Since(began), startNFR: startNFR, seekNFR: seekNFR, speedNFR: speedNFR}
		if speedNFR > 0 {
			// Speed: let the run go on from segment 1 for 10 s, or to its end.
			st := s.stream(sess.ID)
			st.mu.Lock()
			run := st.alive()
			st.mu.Unlock()
			if run != nil {
				wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				run.WaitPiece(wctx, st.layout.Pieces()-1)
				cancel()
				r.speed = run.Speed()
			}
		}
		if err := s.Stop(principalOf(sess), sess.ID, nil); err != nil {
			t.Fatal(err)
		}
		// Seek: a new session whose first request is a far segment, so
		// FFmpeg starts there (R14).
		far := addSession(t, s, d)
		began = time.Now()
		if _, err := s.Segment(ctx, far, seekTo); err != nil {
			t.Fatalf("%s: seek to segment %d: %v", name, seekTo, err)
		}
		r.seek = time.Since(began)
		if err := s.Stop(principalOf(far), far.ID, nil); err != nil {
			t.Fatal(err)
		}
		results = append(results, r)
	}

	if testmedia.HasEncoder(t, "libx265") {
		uhd := media(testmedia.Perf2160p, "hevc", 3840, 2160, 10, 20_000_000, 30_000)
		ds := directStreamOf(uhd)
		measure("Direct Stream 4K HEVC", ds, nil, layoutOf(ds).SegmentAt(24_000), 3*time.Second, 3*time.Second, 0)
	}
	ds := directStreamOf(hd)
	measure("Direct Stream 1080p H.264", ds, nil, layoutOf(ds).SegmentAt(48_000), 3*time.Second, 3*time.Second, 0)
	measure("Transcode 1080p→720p software", to720(nil), nil, 8, 6*time.Second, 6*time.Second, 1.0)
	if st, err := ProbeHardware(ctx, ff, HWAuto, ""); err == nil && st.Active != "" {
		hw := st.config()
		measure(fmt.Sprintf("Transcode 1080p→720p %s", hw.Accel), to720(hw), hw, 8, 4*time.Second, 4*time.Second, 3.0)
	}

	ok := func(good bool) string {
		if good {
			return "✅"
		}
		return "❌"
	}
	t.Log("| Case | Start | Seek | Speed |")
	t.Log("|------|-------|------|-------|")
	for _, r := range results {
		speed := "—"
		if r.speedNFR > 0 {
			speed = fmt.Sprintf("%.1f× %s (≥ %.1f×)", r.speed, ok(r.speed >= r.speedNFR), r.speedNFR)
		}
		t.Logf("| %s | %.2f s %s (< %v) | %.2f s %s (< %v) | %s |", r.name,
			r.start.Seconds(), ok(r.start < r.startNFR), r.startNFR, r.seek.Seconds(), ok(r.seek < r.seekNFR), r.seekNFR, speed)
	}
}
