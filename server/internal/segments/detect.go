package segments

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

// End credits are mostly text on black. The detector looks at the end of
// an episode two frames a second, keeps the frames that are mostly black,
// and takes the last long run of them that reaches the end; black runs
// just before it, with short gaps (credits over pictures, a logo), belong
// to the credits too.
const (
	// window is how much of the end is looked at.
	window = 10 * time.Minute
	// minRun is how long a black run must be to join the credits.
	minRun = 15.0
	// maxGap is the longest non-black stretch inside the credits.
	maxGap = 60.0
	// endSlack is how close to the end the last run must reach.
	endSlack = 8.0
	// minCredits is the shortest credits believed.
	minCredits = 30.0
	// frameStep is the time between two looked-at frames (2 per second).
	frameStep = 0.5
)

var blackLine = regexp.MustCompile(`blackframe.* t:([0-9.]+)`)

// detectCredits runs FFmpeg over the end of a file and returns where its
// credits start, in ms; ok is false when none were found.
func detectCredits(ctx context.Context, ffmpeg, path string, durationMs int64) (int64, bool, error) {
	duration := float64(durationMs) / 1000
	from := max(duration/2, duration-window.Seconds())
	// The admin-configured ffmpeg, on a library file; no shell.
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostats", "-threads", "2", //nolint:gosec // see above
		"-ss", strconv.FormatFloat(from, 'f', 3, 64), "-i", path, "-an", "-sn", "-dn",
		"-vf", "fps=2,scale=320:-2,blackframe=amount=80:threshold=32", "-f", "null", "-")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, false, err
	}
	if err := cmd.Start(); err != nil {
		return 0, false, err
	}
	// Lowest priority: playback and transcodes always come first.
	_ = syscall.Setpriority(syscall.PRIO_PROCESS, cmd.Process.Pid, 19)
	var black []float64
	sc := bufio.NewScanner(stderr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		if m := blackLine.FindStringSubmatch(sc.Text()); m != nil {
			if t, err := strconv.ParseFloat(m[1], 64); err == nil {
				black = append(black, from+t)
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		return 0, false, fmt.Errorf("ffmpeg blackframe: %w", err)
	}
	start, ok := creditsStart(black, duration)
	return int64(start * 1000), ok, nil
}

// creditsStart finds the credits from the times of black frames, in
// order, and the length of the file, both in seconds.
func creditsStart(black []float64, duration float64) (float64, bool) {
	type run struct{ start, end float64 }
	var runs []run
	for _, t := range black {
		if n := len(runs); n > 0 && t-runs[n-1].end <= 3*frameStep {
			runs[n-1].end = t
		} else {
			runs = append(runs, run{t, t})
		}
	}
	if len(runs) == 0 || runs[len(runs)-1].end < duration-endSlack {
		return 0, false
	}
	start := runs[len(runs)-1].start
	for i := len(runs) - 2; i >= 0; i-- {
		r := runs[i]
		if r.end-r.start < minRun || start-r.end > maxGap {
			break
		}
		start = r.start
	}
	if duration-start < minCredits || start < duration/2 {
		return 0, false
	}
	return start, true
}
