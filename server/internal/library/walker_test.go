package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// writeTree creates files (relative, slash-separated) with small contents
// under root. A size suffix isn't needed: tests that care use writeSized.
func writeTree(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("content of "+f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func walkRels(t *testing.T, root string) []string {
	t.Helper()
	res, err := Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	var rels []string
	for _, f := range res.Files {
		rels = append(rels, f.Rel)
	}
	return rels
}

func TestWalkBuiltInIgnoreRules(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root,
		"Keep (2019)/Keep (2019).mkv",
		"Keep (2019)/Keep (2019).MP4",
		".hidden/Movie.mkv",
		"Movie/.hidden.mkv",
		"@eaDir/Movie.mkv",
		"#recycle/Movie.mkv",
		"$RECYCLE.BIN/Movie.mkv",
		"System Volume Information/Movie.mkv",
		"lost+found/Movie.mkv",
		"Movie/Extras/Bonus.mkv",
		"Movie/Featurettes/Bonus.mkv",
		"Movie/Behind The Scenes/Bonus.mkv",
		"Movie/Deleted Scenes/Bonus.mkv",
		"Movie/trailers/Bonus.mkv",
		"Movie/Interviews/Bonus.mkv",
		"Movie/Shorts/Bonus.mkv",
		"Movie/Movie.mkv.part",
		"Movie/Movie.mkv.!qB",
		"Movie/Movie.mkv.crdownload",
		"Movie/Movie.tmp",
		"Movie/Movie-sample.mkv",
		"Movie/Sample/sample.mkv",
		"Movie/notes.txt",
		"Movie/Samples of Life (2020).mkv",
	)
	got := walkRels(t, root)
	want := []string{"Keep (2019)/Keep (2019).MP4", "Keep (2019)/Keep (2019).mkv", "Movie/Samples of Life (2020).mkv"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestIgnoredFileSampleSize(t *testing.T) {
	if !ignoredFile("Movie.sample.mkv", 10<<20) {
		t.Error("small sample not ignored")
	}
	if ignoredFile("Movie.sample.mkv", 400<<20) {
		t.Error("a 400 MB file named sample is a real file")
	}
}

func TestWalkPlaidIgnore(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root,
		"a.mkv",
		"Sub/one.mkv",
		"Sub/keep.mkv",
		"Sub/one.mp4",
		"Sub/Deep/two.mkv",
		"Sub/Deep/keep.mkv",
		"Other/three.mkv",
		"Skipped/four.mkv",
	)
	if err := os.WriteFile(filepath.Join(root, "Sub", ".plaidignore"), []byte("# comment\n*.mkv\n!keep.mkv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".plaidignore"), []byte("Skipped/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A deeper file re-includes what its parent excluded.
	if err := os.WriteFile(filepath.Join(root, "Sub", "Deep", ".plaidignore"), []byte("!two.mkv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := walkRels(t, root)
	want := []string{"Other/three.mkv", "Sub/Deep/keep.mkv", "Sub/Deep/two.mkv", "Sub/keep.mkv", "Sub/one.mp4", "a.mkv"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestWalkSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeTree(t, root, "Movie/Movie.mkv")
	writeTree(t, outside, "Outside.mkv")
	links := map[string]string{
		filepath.Join(root, "loop"):         root,                                      // directory loop
		filepath.Join(root, "Movie", "up"):  filepath.Join(root, "Movie"),              // another loop
		filepath.Join(root, "passwd.mkv"):   "/etc/passwd",                             // escape
		filepath.Join(root, "escape.mkv"):   filepath.Join(outside, "Outside.mkv"),     // escape
		filepath.Join(root, "outdir"):       outside,                                   // symlinked dir
		filepath.Join(root, "inside.mkv"):   filepath.Join(root, "Movie", "Movie.mkv"), // allowed
		filepath.Join(root, "dangling.mkv"): filepath.Join(root, "nope.mkv"),
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	got := walkRels(t, root)
	want := []string{"Movie/Movie.mkv", "inside.mkv"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestWalkFolderErrors(t *testing.T) {
	ctx := context.Background()
	var fe *FolderError

	_, err := Walk(ctx, filepath.Join(t.TempDir(), "missing"))
	if !errors.As(err, &fe) || fe.Status != FolderUnavailable {
		t.Errorf("missing folder: err = %v, want unavailable", err)
	}

	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		locked := t.TempDir()
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(locked, 0o755) })
		_, err = Walk(ctx, locked)
		if !errors.As(err, &fe) || fe.Status != FolderUnreadable {
			t.Errorf("chmod 000 folder: err = %v, want unreadable", err)
		}
	}
}

func TestWalkUnreadableSubfolderIsSkipped(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	root := t.TempDir()
	writeTree(t, root, "A/a.mkv", "B/b.mkv")
	if err := os.Chmod(filepath.Join(root, "B"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(root, "B"), 0o755) })
	res, err := Walk(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || res.Files[0].Rel != "A/a.mkv" {
		t.Errorf("files = %+v", res.Files)
	}
	if !slices.Equal(res.SkippedDirs, []string{"B"}) {
		t.Errorf("skipped = %v, want [B]", res.SkippedDirs)
	}
}

func TestWalkExternalSubtitles(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root,
		"Movie (2019)/Movie (2019).mkv",
		"Movie (2019)/Movie (2019).srt",
		"Movie (2019)/Movie (2019).fr.forced.srt",
		"Movie (2019)/Movie (2019).en.sdh.srt",
		"Movie (2019)/Movie (2019).eng.ass",
		"Movie (2019)/Movie (2019).french.ssa",
		"Movie (2019)/Movie (2019).ger.vtt",
		"Movie (2019)/Movie (2019).pt-BR.srt",
		"Movie (2019)/Movie (2019).en.hi.srt",
		"Movie (2019)/Movie (2019).hi.srt",
		"Movie (2019)/Movie (2019).fra.cc.srt",
		"Movie (2019)/Movie (2019).en.sub",
		"Movie (2019)/Movie (2019).en.idx",
		"Movie (2019)/Movie (2019).orphan.sub",
		"Movie (2019)/Other.srt",
	)
	res, err := Walk(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("files = %+v", res.Files)
	}
	type sub struct {
		name, codec, lang  string
		forced, sdh, image bool
	}
	var got []sub
	for _, s := range res.Files[0].Subtitles {
		got = append(got, sub{filepath.Base(s.Path), s.Codec, s.Language, s.Forced, s.SDH, s.Image})
	}
	want := []sub{
		{"Movie (2019).en.hi.srt", "subrip", "eng", false, true, false},
		{"Movie (2019).en.sdh.srt", "subrip", "eng", false, true, false},
		{"Movie (2019).en.sub", "dvd_subtitle", "eng", false, false, true},
		{"Movie (2019).eng.ass", "ass", "eng", false, false, false},
		{"Movie (2019).fr.forced.srt", "subrip", "fre", true, false, false},
		{"Movie (2019).fra.cc.srt", "subrip", "fre", false, true, false},
		{"Movie (2019).french.ssa", "ssa", "fre", false, false, false},
		{"Movie (2019).ger.vtt", "webvtt", "ger", false, false, false},
		{"Movie (2019).hi.srt", "subrip", "hin", false, false, false},
		{"Movie (2019).pt-BR.srt", "subrip", "por", false, false, false},
		{"Movie (2019).srt", "subrip", "und", false, false, false},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestMatchSubtitlesLongestVideoWins(t *testing.T) {
	subs := matchSubtitles("/m", []string{"Movie.mkv", "Movie.Part2.mkv"}, []string{"Movie.Part2.en.srt", "Movie.en.srt"})
	if len(subs["Movie.mkv"]) != 1 || len(subs["Movie.Part2.mkv"]) != 1 {
		t.Errorf("subs = %+v", subs)
	}
}

func TestIgnoreStackDirectoryPattern(t *testing.T) {
	s := ignoreStack{"": parseIgnoreFile("Old/\n")}
	if !s.ignored("Old", true) {
		t.Error("Old/ should match the directory")
	}
	if s.ignored("Old", false) {
		t.Error("Old/ shouldn't match a file named Old")
	}
}
