package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// treeHash fingerprints everything under root: every path with its size,
// modification time and mode, plus the content of files under 1 MiB. Any
// file created, changed, renamed or deleted changes it (R23).
func treeHash(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		line := fmt.Sprintf("%s|%d|%d|%s", rel, info.Size(), info.ModTime().UnixNano(), info.Mode())
		if info.Mode().IsRegular() && info.Size() < 1<<20 {
			f, err := os.Open(path) //nolint:gosec // test tree
			if err != nil {
				return err
			}
			h := sha256.New()
			_, err = io.Copy(h, f)
			_ = f.Close()
			if err != nil {
				return err
			}
			line += "|" + hex.EncodeToString(h.Sum(nil))
		}
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// TestScansNeverWriteToLibraryFolders runs full scans, rescans, a move and a
// deletion, and checks every scan leaves the tree exactly as it found it.
// It uses the real ffprobe when one is installed.
func TestScansNeverWriteToLibraryFolders(t *testing.T) {
	e := newScanEnv(t, TypeMovies)
	if p, err := FindFFprobe(context.Background(), "", discardLog()); err == nil {
		e.scan.Prober = p
		if ffmpeg, err := exec.LookPath("ffmpeg"); err == nil {
			dir := e.path("Real (2024)")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25",
				"-t", "1", "-c:v", "mpeg4", filepath.Join(dir, "Real (2024).mkv")) //nolint:gosec // test tool
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("ffmpeg: %v: %s", err, out)
			}
		}
	}
	e.write("The Matrix (1999)/The Matrix (1999).mkv", strings.Repeat("m", 3<<20))
	e.write("The Matrix (1999)/The Matrix (1999).fr.forced.srt", "1\n00:00:01,000 --> 00:00:02,000\nSalut\n")
	e.write("Movie (2019) - 1080p.mkv", "a")
	e.write("Movie (2019) - 2160p.mkv", "b")
	e.write("Broken (2001).mkv", "broken")
	e.write("@eaDir/thumb.mkv", "x")
	e.write("Extras/Trailer.mkv", "x")
	e.write(".plaidignore", "*.tmp.mkv\n")
	e.write("partial.tmp.mkv", "x")
	e.write("unknown.mkv", "x")

	scan := func(what string, full bool) {
		t.Helper()
		before := treeHash(t, e.root)
		e.run(ScanOptions{Full: full})
		if after := treeHash(t, e.root); after != before {
			t.Fatalf("%s changed the library tree", what)
		}
	}
	scan("first full scan", true)
	scan("rescan", false)
	scan("full rescan", true)
	if err := os.Rename(e.path("The Matrix (1999)"), e.path("Matrix")); err != nil {
		t.Fatal(err)
	}
	scan("scan after a move", false)
	if err := os.Remove(e.path("Movie (2019) - 1080p.mkv")); err != nil {
		t.Fatal(err)
	}
	scan("scan after a deletion", false)
	e.now = e.now.Add(31 * 24 * time.Hour)
	scan("scan purging after 30 days", false)
}
