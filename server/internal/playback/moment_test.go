package playback

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// What the measuring filters print: a block per keyframe, a level per
// second; silence counts as the floor, the first keyframe's difference
// as nothing.
func TestParseWindowStats(t *testing.T) {
	video := `frame:0    pts:0       pts_time:0
lavfi.signalstats.YAVG=100
lavfi.signalstats.YDIF=0
frame:1    pts:48      pts_time:2
lavfi.signalstats.YAVG=60
lavfi.signalstats.YDIF=20
frame:2    pts:96      pts_time:4
lavfi.signalstats.YAVG=80
lavfi.signalstats.YDIF=10
`
	audio := `frame:0    pts:0       pts_time:0
lavfi.astats.Overall.RMS_level=-20
frame:1    pts:8000    pts_time:1
lavfi.astats.Overall.RMS_level=-inf
`
	st := parseWindowStats(video, audio, true)
	if st.keyframes != 3 || st.luma != 80 || st.change != 15 {
		t.Errorf("video: %+v, want 3 keyframes, luma 80, change 15", st)
	}
	if st.loud != -55 || st.swing != 35 || !st.sound {
		t.Errorf("audio: %+v, want loud -55, swing 35", st)
	}
	if st := parseWindowStats(video, "", true); st.loud != previewFloor {
		t.Errorf("a track that printed nothing is silent: %+v", st)
	}
	if st := parseWindowStats("", "", false); st.keyframes != 0 {
		t.Errorf("nothing decoded: %+v", st)
	}
}

// The liveliest window wins; a dark, washed-out or quiet one loses
// whatever else it has; windows not weighed are skipped.
func TestBestMoment(t *testing.T) {
	calm := windowStats{keyframes: 4, luma: 90, change: 5, loud: -30, swing: 2, sound: true}
	lively := windowStats{keyframes: 12, luma: 110, change: 25, loud: -18, swing: 8, sound: true}
	dark, washed, quiet := lively, lively, lively
	dark.luma, washed.luma, quiet.loud = 20, 240, -60
	dark.keyframes, washed.keyframes, quiet.keyframes = 20, 20, 20
	for _, c := range []struct {
		name  string
		stats []windowStats
		want  int
	}{
		{"lively", []windowStats{calm, lively, calm}, 1},
		{"not weighed", []windowStats{{}, calm, {}}, 1},
		{"dark", []windowStats{calm, dark, calm, calm}, 0},
		{"washed out", []windowStats{calm, washed, calm, calm}, 0},
		{"quiet", []windowStats{calm, quiet, calm, calm}, 0},
		{"all alike: the first", []windowStats{calm, calm}, 0},
		{"none", []windowStats{{}, {}}, -1},
	} {
		if got := bestMoment(c.stats); got != c.want {
			t.Errorf("%s: bestMoment = %d, want %d", c.name, got, c.want)
		}
	}
	// A silent file is judged on its pictures.
	a, b := calm, lively
	a.sound, b.sound, b.loud = false, false, previewFloor
	if got := bestMoment([]windowStats{a, b}); got != 1 {
		t.Errorf("silent file: bestMoment = %d, want 1", got)
	}
}

// On a real file, dark and silent but for one loud, busy stretch, the
// moment is that stretch; it's kept, so the next cut doesn't weigh again.
func TestPreviewMoment(t *testing.T) {
	ffPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	// 6 minutes: the candidates start every 7⅓ s from 36 s, the seventh
	// at 80 s, where the loud, busy stretch is.
	dir := t.TempDir()
	clip := filepath.Join(dir, "m.mp4")
	out, err := exec.Command(ffPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=160x90:r=12:d=360",
		"-f", "lavfi", "-i", "testsrc2=s=160x90:r=12:d=360",
		"-f", "lavfi", "-i", "anoisesrc=d=360:c=pink:r=48000:a=0.5",
		"-filter_complex", "[0][1]overlay=enable='between(t,80,104)'[v];"+
			"[2]volume=enable='not(between(t,80,104))':volume=0[a]",
		"-map", "[v]", "-map", "[a]", "-c:v", "libx264", "-preset", "ultrafast", "-g", "120",
		"-force_key_frames", "expr:between(t,80,104)*eq(mod(n,6),0)", "-c:a", "aac", clip).CombinedOutput()
	if err != nil {
		t.Fatalf("making the clip: %v: %s", err, out)
	}
	s := &Service{o: Options{FFmpeg: &FFmpeg{Path: ffPath}, Log: slog.New(slog.DiscardHandler)},
		previews: newPreviews(filepath.Join(dir, "previews"))}
	src := previewSource{fileID: "f", path: clip, duration: 6 * time.Minute, audio: 1, momentKey: "k"}
	if at := s.previewMoment(context.Background(), src, "2"); at != 80*time.Second {
		t.Fatalf("moment at %v, want 80s", at)
	}
	kept := filepath.Join(s.previews.moments, "k")
	if b, err := os.ReadFile(kept); err != nil || string(b) != "80000" {
		t.Fatalf("kept %q (%v), want 80000", b, err)
	}
	src.path = filepath.Join(dir, "gone.mp4")
	if at := s.previewMoment(context.Background(), src, "2"); at != 80*time.Second {
		t.Errorf("second time at %v, want the kept 80s", at)
	}
}
