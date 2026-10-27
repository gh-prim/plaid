//go:build linux

package playback

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/playback/testmedia"
)

// crashChildEnv makes the test binary act as a PLAID that starts a
// transcode and waits to be killed.
const crashChildEnv = "PLAID_TEST_CRASH_CHILD"

// TestCrashChild is the child half of TestNoFFmpegSurvivesACrash.
func TestCrashChild(t *testing.T) {
	dir := os.Getenv(crashChildEnv)
	if dir == "" {
		t.Skip("only runs as TestNoFFmpegSurvivesACrash's child")
	}
	s := New(Options{FFmpeg: &FFmpeg{Path: testmedia.FFmpeg(t)}, TranscodeDir: dir, CacheDir: t.TempDir(),
		Free: func(string) (uint64, error) { return 100 << 30, nil }})
	s.Start(context.Background())
	sess := addSession(t, s, transcodeOf(longMedia(t)))
	if _, err := s.Segment(context.Background(), sess, 0); err != nil {
		t.Fatal(err)
	}
	fmt.Println("READY")
	time.Sleep(time.Minute) // killed long before
}

// TestNoFFmpegSurvivesACrash kills a PLAID mid-transcode with SIGKILL:
// Pdeathsig takes its FFmpeg down with it, and the next start empties
// the transcode folder (R28).
func TestNoFFmpegSurvivesACrash(t *testing.T) {
	testmedia.Clip(t, testmedia.Long) // built here, not in the child
	dir := filepath.Join(t.TempDir(), "transcodes")
	child := exec.Command(os.Args[0], "-test.run=^TestCrashChild$", "-test.v") //nolint:gosec // the test binary itself
	child.Env = append(os.Environ(), crashChildEnv+"="+dir)
	out, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	ready := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if sc.Text() == "READY" {
				ready <- true
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("the child failed before starting FFmpeg")
		}
	case <-time.After(2 * time.Minute):
		_ = child.Process.Kill()
		t.Fatal("the child never started FFmpeg")
	}
	if len(ffmpegsIn(t, dir)) == 0 {
		t.Fatal("no FFmpeg running for the child")
	}

	_ = child.Process.Signal(syscall.SIGKILL)
	_ = child.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for len(ffmpegsIn(t, dir)) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("FFmpeg survived its PLAID: pids %v", ffmpegsIn(t, dir))
		}
		time.Sleep(50 * time.Millisecond)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("the crashed run left no folder to clean up")
	}
	st := &Storage{Dir: dir, Free: func(string) (uint64, error) { return 100 << 30, nil }}
	if purged, err := st.Open(); err != nil || purged != len(entries) {
		t.Fatalf("purged %d of %d: %v", purged, len(entries), err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left after the start: %v", entries)
	}
}

// ffmpegsIn lists the pids of processes whose command line mentions dir.
func ffmpegsIn(t *testing.T, dir string) []string {
	t.Helper()
	procs, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatal(err)
	}
	var pids []string
	for _, p := range procs {
		cmdline, err := os.ReadFile(filepath.Join("/proc", p.Name(), "cmdline"))
		if err != nil || !strings.Contains(string(cmdline), dir) || !strings.Contains(string(cmdline), "ffmpeg") {
			continue
		}
		// A zombie waiting for its reaper isn't running.
		if stat, err := os.ReadFile(filepath.Join("/proc", p.Name(), "stat")); err == nil {
			if f := strings.Fields(string(stat)); len(f) > 2 && f[2] == "Z" {
				continue
			}
		}
		pids = append(pids, p.Name())
	}
	return pids
}
