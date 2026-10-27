package playback

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/playback/keyframes"
	"github.com/gh-prim/plaid/server/internal/playback/testmedia"
)

// longMedia is the 20-minute clip as the decider would see it.
func longMedia(t *testing.T) Media {
	t.Helper()
	path := testmedia.Clip(t, testmedia.Long)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	kf, err := keyframes.ReadFile(path, "mkv")
	if err != nil {
		t.Fatal(err)
	}
	return Media{
		FileID: "long", Path: path, Size: st.Size(), ModTime: st.ModTime(), Container: "mkv",
		DurationMs: 1_200_000, Bitrate: 90_000,
		Video:            &library.Stream{Index: 0, Type: "video", Codec: "h264", Width: 426, Height: 240, Level: 21},
		Audio:            []library.Stream{{Index: 1, Type: "audio", Codec: "aac", Channels: 1, Default: true}},
		HasKeyframeIndex: true, Keyframes: kf,
	}
}

// mp4Media is the 60 s MP4 clip, whose H.264 has B-frames and an edit list.
func mp4Media(t *testing.T) Media {
	t.Helper()
	path := testmedia.Clip(t, testmedia.MP4)
	kf, err := keyframes.ReadFile(path, "mp4")
	if err != nil {
		t.Fatal(err)
	}
	return Media{
		FileID: "mp4", Path: path, Container: "mp4", DurationMs: 60_000, Bitrate: 150_000,
		Video:            &library.Stream{Index: 0, Type: "video", Codec: "h264", Width: 640, Height: 360},
		Audio:            []library.Stream{{Index: 1, Type: "audio", Codec: "aac", Channels: 1, Default: true}},
		HasKeyframeIndex: true, Keyframes: kf,
	}
}

func transcodeOf(m Media) Decision {
	return Decision{Method: Transcode, Media: m, Audio: &m.Audio[0], AudioCodec: "aac", AudioChannels: 2,
		Width: 426, Height: 240, VideoBitrate: 300_000, VideoCodec: "h264", Quality: "360p"}
}

func directStreamOf(m Media) Decision {
	return Decision{Method: DirectStream, Media: m, VideoCopy: true, Audio: &m.Audio[0], AudioCodec: "aac", AudioChannels: 2, Quality: QualityOriginal}
}

type packet struct {
	video bool
	pts   float64
	key   bool
}

// probe joins an init segment and pieces and lists their packets.
func probe(t *testing.T, init string, pieces ...string) []packet {
	t.Helper()
	var buf bytes.Buffer
	for _, p := range append([]string{init}, pieces...) {
		b, err := os.ReadFile(p) //nolint:gosec // test output
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
	}
	joined := filepath.Join(t.TempDir(), "joined.mp4")
	if err := os.WriteFile(joined, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "packet=codec_type,pts_time,flags", "-of", "csv=p=0", joined).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var pkts []packet
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, ",")
		if len(f) < 3 {
			continue
		}
		pts, err := strconv.ParseFloat(f[1], 64)
		if err != nil {
			continue
		}
		pkts = append(pkts, packet{video: f[0] == "video", pts: pts, key: strings.HasPrefix(f[2], "K")})
	}
	return pkts
}

func ptsRange(pkts []packet, video bool) (lo, hi float64) {
	lo, hi = 1e18, -1
	for _, p := range pkts {
		if p.video == video {
			lo, hi = min(lo, p.pts), max(hi, p.pts)
		}
	}
	return lo, hi
}

func runTo(t *testing.T, spec RunSpec, piece int) *Run {
	t.Helper()
	r, err := StartRun(testmedia.FFmpeg(t), spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if !r.WaitPiece(ctx, piece) {
		t.Fatalf("piece %d never came: %v\n%s", piece, r.Err(), r.Stderr())
	}
	return r
}

// TestRestartContinuity is the plan's step 16 spike: a run restarted at
// segment N writes the same init segment as a run from 0, and its
// segment N starts where the first run's segment N-1 ends.
func TestRestartContinuity(t *testing.T) {
	m, b := longMedia(t), mp4Media(t)
	for _, c := range []struct {
		name     string
		decision Decision
		layout   Layout
		seg      int
		frame    float64
	}{
		{"transcode", transcodeOf(m), TranscodeLayout(m.DurationMs), 10, 1.0 / 12},
		{"direct stream", directStreamOf(m), StreamLayout(m.Keyframes, m.DurationMs), 10, 1.0 / 12},
		{"direct stream with B-frames", directStreamOf(b), StreamLayout(b.Keyframes, b.DurationMs), 5, 1.0 / 24},
	} {
		t.Run(c.name, func(t *testing.T) {
			seg, frame := c.seg, c.frame
			s := c.layout.Segments[seg]
			a := RunSpec{Decision: c.decision, Layout: c.layout, Dir: filepath.Join(t.TempDir(), "a"), RunID: 1}
			runA := runTo(t, a, s.First+(s.Last-s.First)) // all of segment 9, and 10 for comparison
			runA.Stop()
			b := a
			b.Dir, b.RunID, b.StartPiece = filepath.Join(t.TempDir(), "b"), 2, s.First
			runB := runTo(t, b, s.Last)
			runB.Stop()

			initA, _ := os.ReadFile(filepath.Join(a.Dir, "init.mp4")) //nolint:gosec // test output
			initB, _ := os.ReadFile(filepath.Join(b.Dir, "init.mp4")) //nolint:gosec // test output
			if len(initA) == 0 || !bytes.Equal(initA, initB) {
				t.Fatalf("init segments differ (%d and %d bytes)", len(initA), len(initB))
			}

			pieces := func(dir string, sg Segment) []string {
				var out []string
				for p := sg.First; p < sg.Last; p++ {
					out = append(out, filepath.Join(dir, strconv.Itoa(p)+".m4s"))
				}
				return out
			}
			before := probe(t, filepath.Join(a.Dir, "init.mp4"), pieces(a.Dir, c.layout.Segments[seg-1])...)
			fromA := probe(t, filepath.Join(a.Dir, "init.mp4"), pieces(a.Dir, s)...)
			fromB := probe(t, filepath.Join(b.Dir, "init.mp4"), pieces(b.Dir, s)...)
			_, endBefore := ptsRange(before, true)
			startA, _ := ptsRange(fromA, true)
			startB, _ := ptsRange(fromB, true)
			want := float64(s.StartMs) / 1000
			if d := startB - want; d < -frame/2 || d > frame/2 {
				t.Errorf("restarted segment %d starts at %.3f s, want %.3f s", seg, startB, want)
			}
			if d := startB - startA; d < -frame/2 || d > frame/2 {
				t.Errorf("segment %d starts at %.3f s from run A, %.3f s from run B", seg, startA, startB)
			}
			if gap := startB - endBefore; gap < frame/2 || gap > 1.5*frame {
				t.Errorf("gap of %.3f s between segment %d (run A) and %d (run B)", gap, seg-1, seg)
			}
			if !fromB[firstVideo(fromB)].key {
				t.Error("the restarted segment doesn't start on a keyframe")
			}
			// Audio lines up with video: no muxer shift of either.
			aLo, _ := ptsRange(fromB, false)
			if d := aLo - want; d < -0.1 || d > 0.1 {
				t.Errorf("audio of the restarted segment starts at %.3f s, want %.3f s ± 100 ms", aLo, want)
			}
		})
	}
}

func firstVideo(pkts []packet) int {
	best := -1
	for i, p := range pkts {
		if p.video && (best < 0 || p.pts < pkts[best].pts) {
			best = i
		}
	}
	return best
}

func TestRunStopAndPause(t *testing.T) {
	m := longMedia(t)
	spec := RunSpec{Decision: transcodeOf(m), Layout: TranscodeLayout(m.DurationMs), Dir: t.TempDir()}
	r := runTo(t, spec, 1)

	r.Pause()
	if !r.Paused() {
		t.Fatal("not paused")
	}
	at := r.Produced()
	time.Sleep(1500 * time.Millisecond)
	if r.Produced() > at+1 { // one piece may have been in its rename
		t.Fatalf("a paused run went from piece %d to %d", at, r.Produced())
	}
	r.Resume()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if !r.WaitPiece(ctx, at+3) {
		t.Fatal("a resumed run didn't continue")
	}

	// A stopped run ends on SIGTERM even while paused, well before the
	// SIGKILL grace period.
	r.Pause()
	start := time.Now()
	r.Stop()
	if d := time.Since(start); d >= stopGrace {
		t.Errorf("stop took %v", d)
	}
	if err := syscall.Kill(r.Pid(), 0); err == nil {
		t.Error("ffmpeg still exists after Stop")
	}
	if r.Err() != nil {
		t.Errorf("a stopped run isn't a failure: %v", r.Err())
	}
}

func TestRunFailure(t *testing.T) {
	m := longMedia(t)
	m.Path = filepath.Join(t.TempDir(), "missing.mkv")
	r, err := StartRun(testmedia.FFmpeg(t), RunSpec{Decision: transcodeOf(m), Layout: TranscodeLayout(m.DurationMs), Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	<-r.Done()
	if r.Err() == nil || !strings.Contains(r.Err().Error(), "missing.mkv") {
		t.Fatalf("err %v, want the stderr of a failed run", r.Err())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if r.WaitPiece(ctx, 0) {
		t.Fatal("a failed run has no pieces")
	}
}
