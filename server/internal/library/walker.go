package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// WalkedFile is a video found on disk.
type WalkedFile struct {
	Path      string // absolute
	Rel       string // slash-separated, relative to the library folder
	Size      int64
	ModTime   time.Time
	Subtitles []ExternalSubtitle
}

// WalkResult is the outcome of walking one library folder.
type WalkResult struct {
	Files []WalkedFile
	// SkippedDirs are sub-folders (slash-separated, relative) that couldn't
	// be read. Files stored under them must not be marked missing.
	SkippedDirs []string
}

// FolderError means a library folder can't be scanned at all (R17). Status
// is FolderUnavailable or FolderUnreadable.
type FolderError struct {
	Status string
	Err    error
}

func (e *FolderError) Error() string { return fmt.Sprintf("folder %s: %v", e.Status, e.Err) }
func (e *FolderError) Unwrap() error { return e.Err }

// isIOError reports errors that mean the storage itself is failing or gone
// (dead disk, dropped network mount), as opposed to one bad entry.
func isIOError(err error) bool {
	return errors.Is(err, syscall.EIO) || errors.Is(err, syscall.ENOTCONN) ||
		errors.Is(err, syscall.ESTALE) || errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENXIO)
}

func folderError(err error) *FolderError {
	if errors.Is(err, fs.ErrPermission) {
		return &FolderError{Status: FolderUnreadable, Err: err}
	}
	return &FolderError{Status: FolderUnavailable, Err: err}
}

// checkRoot verifies a library folder exists, is a directory and can be listed.
func checkRoot(root string) error {
	st, err := os.Stat(root)
	if err != nil {
		return folderError(err)
	}
	if !st.IsDir() {
		return &FolderError{Status: FolderUnavailable, Err: fmt.Errorf("%s is not a directory", root)}
	}
	f, err := os.Open(root) //nolint:gosec // library folders are admin-configured paths
	if err != nil {
		return folderError(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.ReadDir(1); err != nil && !errors.Is(err, io.EOF) {
		return folderError(err)
	}
	return nil
}

// Walk lists the videos of a library folder with their external subtitles,
// applying the ignore rules (R4–R6). It never follows symlinked directories
// and follows a symlinked file only when its target stays inside the folder.
// It only reads: nothing is ever written under root (R23).
//
// A *FolderError is returned when the folder itself is missing, unreadable or
// failing; the caller must then leave the folder's files alone.
func Walk(ctx context.Context, root string) (*WalkResult, error) {
	if err := checkRoot(root); err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, folderError(err)
	}

	res := &WalkResult{}
	stack := ignoreStack{}
	videos := map[string][]string{} // dir rel → video names
	others := map[string][]string{} // dir rel → subtitle and .idx names
	infos := map[string]fs.FileInfo{}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		if err != nil {
			if rel == "" || isIOError(err) {
				return folderError(err)
			}
			// Permission denied or an entry that vanished: leave its subtree alone.
			res.SkippedDirs = append(res.SkippedDirs, rel)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if d.IsDir() {
			if rel != "" && (ignoredDir(name) || stack.ignored(rel, true)) {
				return filepath.SkipDir
			}
			content, err := os.ReadFile(filepath.Join(path, ignoreFileName)) //nolint:gosec // inside the library folder
			if err == nil {
				stack[rel] = parseIgnoreFile(string(content))
			}
			return nil
		}

		ext := lowerExt(name)
		if !IsVideo(name) && !isSubtitle(name) && ext != ".idx" {
			return nil
		}
		var info fs.FileInfo
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(path)
			if err != nil || !within(realRoot, target) {
				return nil
			}
			if info, err = os.Stat(target); err != nil || !info.Mode().IsRegular() {
				return nil
			}
		} else if d.Type().IsRegular() {
			if info, err = d.Info(); err != nil {
				return nil
			}
		} else {
			return nil
		}
		if ignoredFile(name, info.Size()) || stack.ignored(rel, false) {
			return nil
		}
		dir := pathDir(rel)
		if IsVideo(name) {
			videos[dir] = append(videos[dir], name)
			infos[rel] = info
		} else {
			others[dir] = append(others[dir], name)
		}
		return nil
	})
	if walkErr != nil {
		var fe *FolderError
		if errors.As(walkErr, &fe) {
			return nil, fe
		}
		return nil, walkErr
	}
	// The drive may have gone away during the walk.
	if err := checkRoot(root); err != nil {
		return nil, err
	}

	for dir, names := range videos {
		subs := matchSubtitles(filepath.Join(root, filepath.FromSlash(dir)), names, others[dir])
		for _, n := range names {
			rel := joinRel(dir, n)
			info := infos[rel]
			res.Files = append(res.Files, WalkedFile{
				Path:      filepath.Join(root, filepath.FromSlash(rel)),
				Rel:       rel,
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				Subtitles: subs[n],
			})
		}
	}
	sort.Slice(res.Files, func(i, j int) bool { return res.Files[i].Rel < res.Files[j].Rel })
	return res, nil
}

// within reports whether target is root or below it.
func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func pathDir(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

func joinRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
