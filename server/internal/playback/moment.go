package playback

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The preview's moment: no public service says where a title is at its
// best, so the candidate windows (previewCandidatesAt) are weighed on the
// file itself. Only keyframes are decoded, at a thumbnail's size, so it
// costs a few seconds even on a 4K HEVC; the sound is measured once a
// second.
const (
	// previewDark and previewBright: a window this dark or washed out
	// reads badly behind the hero's text.
	previewDark   = 40
	previewBright = 215
	// previewQuiet (dBFS): a window this quiet is a lull.
	previewQuiet = -45
	// previewFloor is where silence (-inf dBFS) is counted.
	previewFloor = -90
	// previewUnfit is taken off a dark, washed-out or quiet window's
	// score: it only wins when every window is as bad.
	previewUnfit = 100
)

// windowStats is what a candidate window looked and sounded like.
type windowStats struct {
	// keyframes: encoders put one on most cuts, so more means livelier.
	keyframes int
	// luma is the mean brightness (0–255); change, the mean difference
	// from one keyframe to the next.
	luma, change float64
	// loud is the mean loudness in dBFS, swing its spread, per second;
	// sound says the file has a track to measure.
	loud, swing float64
	sound       bool
}

// previewMoment is where the preview starts: the file's moment weighed
// before, else the best of the candidate windows, kept for next time. When
// nothing could be weighed, the middle candidate, not kept.
func (s *Service) previewMoment(ctx context.Context, src previewSource, threads string) time.Duration {
	ats := previewCandidatesAt(src.duration)
	path := filepath.Join(s.previews.moments, src.momentKey)
	if b, err := os.ReadFile(path); err == nil { //nolint:gosec // our cache
		if ms, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}
	stats := make([]windowStats, len(ats))
	for i, at := range ats {
		st, err := weighWindow(ctx, s.o.FFmpeg, src, at, threads)
		if ctx.Err() != nil {
			return ats[len(ats)/2]
		}
		if err != nil {
			s.o.Log.Debug("preview window not weighed", "file_id", src.fileID, "at", at, "err", err)
			continue
		}
		stats[i] = st
	}
	best := bestMoment(stats)
	if best < 0 {
		s.o.Log.Warn("preview moment not weighed", "file_id", src.fileID)
		return ats[len(ats)/2]
	}
	at := ats[best]
	if err := writeFileAtomic(path, []byte(strconv.FormatInt(at.Milliseconds(), 10))); err != nil {
		s.o.Log.Warn("preview moment not kept", "file_id", src.fileID, "err", err)
	}
	return at
}

// weighWindow measures one window. The two measuring filters write to
// their own files, in a folder of their own the run starts in, so their
// lines don't mix and no path needs escaping in the filter graph.
func weighWindow(ctx context.Context, ff *FFmpeg, src previewSource, at time.Duration, threads string) (windowStats, error) {
	tmp, err := os.MkdirTemp("", "plaid-moment-")
	if err != nil {
		return windowStats{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	cmd := exec.CommandContext(ctx, ff.Path, previewWeighArgs(src, at, threads)...) //nolint:gosec // argument list
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		return windowStats{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	video, err := os.ReadFile(filepath.Join(tmp, "video.txt")) //nolint:gosec // our temp folder
	if err != nil {
		return windowStats{}, err
	}
	audio, _ := os.ReadFile(filepath.Join(tmp, "audio.txt")) //nolint:gosec // our temp folder
	return parseWindowStats(string(video), string(audio), src.audio >= 0), nil
}

// previewWeighArgs decodes the window's keyframes at a thumbnail's size
// in grey, and its sound at a low rate, one second at a time; each
// filter prints what it measured to a file of the current folder.
func previewWeighArgs(src previewSource, at time.Duration, threads string) []string {
	secs := func(d time.Duration) string { return strconv.FormatFloat(d.Seconds(), 'f', 3, 64) }
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-filter_threads", threads,
		"-threads", threads, "-skip_frame", "nokey", "-ss", secs(at), "-t", secs(previewLength), "-i", src.path,
		// With keyframes only, -t lets frames past the window through: trim.
		"-map", "0:v:0", "-vf", "trim=end=" + secs(previewLength) +
			",scale=96:-2,format=gray,signalstats,metadata=mode=print:file=video.txt"}
	if src.audio >= 0 {
		args = append(args, "-map", "0:"+strconv.Itoa(src.audio), "-af",
			"aresample=8000,asetnsamples=n=8000,"+
				"astats=metadata=1:reset=1:measure_overall=RMS_level:measure_perchannel=none,"+
				"ametadata=mode=print:file=audio.txt")
	}
	return append(args, "-f", "null", "-")
}

// parseWindowStats reads what the measuring filters printed: one block
// per keyframe (YAVG, YDIF), one RMS level per second of sound.
func parseWindowStats(video, audio string, sound bool) windowStats {
	st := windowStats{sound: sound}
	var luma, change float64
	diffs := 0
	value := func(line, key string) (float64, bool) {
		v, ok := strings.CutPrefix(line, key+"=")
		if !ok {
			return 0, false
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	}
	sc := bufio.NewScanner(strings.NewReader(video))
	for sc.Scan() {
		line := sc.Text()
		if v, ok := value(line, "lavfi.signalstats.YAVG"); ok {
			st.keyframes++
			luma += v
		} else if v, ok := value(line, "lavfi.signalstats.YDIF"); ok && st.keyframes > 1 {
			// The first keyframe has nothing before it.
			diffs++
			change += v
		}
	}
	if st.keyframes > 0 {
		st.luma = luma / float64(st.keyframes)
	}
	if diffs > 0 {
		st.change = change / float64(diffs)
	}
	var levels []float64
	sc = bufio.NewScanner(strings.NewReader(audio))
	for sc.Scan() {
		if v, ok := value(sc.Text(), "lavfi.astats.Overall.RMS_level"); ok {
			levels = append(levels, max(v, previewFloor)) // -inf is silence
		}
	}
	st.loud, st.swing = meanSpread(levels)
	if sound && len(levels) == 0 {
		st.loud = previewFloor
	}
	return st
}

// bestMoment picks the liveliest window: many cuts, much change from
// shot to shot, loud and with swings in the sound, each counted against
// the other windows of the same file; dark, washed-out or quiet windows
// lose. It returns -1 when no window was weighed (no keyframe).
func bestMoment(stats []windowStats) int {
	var keyframes, change, loud, swing []float64
	for _, st := range stats {
		if st.keyframes == 0 {
			continue
		}
		keyframes = append(keyframes, float64(st.keyframes))
		change = append(change, st.change)
		loud = append(loud, st.loud)
		swing = append(swing, st.swing)
	}
	best, bestScore := -1, math.Inf(-1)
	for i, st := range stats {
		if st.keyframes == 0 {
			continue
		}
		score := zScore(float64(st.keyframes), keyframes) + zScore(st.change, change)
		if st.sound {
			score += zScore(st.loud, loud) + zScore(st.swing, swing)/2
			if st.loud < previewQuiet {
				score -= previewUnfit
			}
		}
		if st.luma < previewDark || st.luma > previewBright {
			score -= previewUnfit
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

// meanSpread is the mean and standard deviation of xs.
func meanSpread(xs []float64) (mean, spread float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	for _, x := range xs {
		spread += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(spread / float64(len(xs)))
}

// zScore is how many standard deviations x is above the mean of xs; 0
// when they're all alike.
func zScore(x float64, xs []float64) float64 {
	mean, spread := meanSpread(xs)
	if spread < 1e-9 {
		return 0
	}
	return (x - mean) / spread
}

// writeFileAtomic writes data to path through a temporary file, so a
// reader never sees half of it.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
