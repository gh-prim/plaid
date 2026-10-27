package library

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Folder validation codes (R2), also the API error codes.
const (
	CodeFolderNotFound   = "folder_not_found"
	CodeFolderUnreadable = "folder_unreadable"
)

// FolderInvalidError rejects a library folder (R2).
type FolderInvalidError struct {
	Code string // CodeFolderNotFound or CodeFolderUnreadable
	Path string // as given
	Err  error
}

func (e *FolderInvalidError) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Path, e.Err)
}

func (e *FolderInvalidError) Unwrap() error { return e.Err }

// FolderOverlapError means a folder is inside, contains or equals another
// library folder (R2).
type FolderOverlapError struct {
	Path  string // cleaned path being saved
	Other string // existing or sibling folder it overlaps
}

func (e *FolderOverlapError) Error() string {
	return fmt.Sprintf("folder %s overlaps %s", e.Path, e.Other)
}

// ErrBadPath rejects a relative path or one with a ".." segment.
var ErrBadPath = errors.New("path must be absolute, with no .. segment")

// hasDotDot reports whether p has a ".." segment.
func hasDotDot(p string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(p), "/"), "..")
}

// ValidateFolder checks that p is an absolute path to a directory PLAID can
// list and traverse, and returns it cleaned with symlinks resolved.
func ValidateFolder(p string) (string, error) {
	invalid := func(code string, err error) error { return &FolderInvalidError{Code: code, Path: p, Err: err} }
	if !filepath.IsAbs(p) || hasDotDot(p) {
		return "", invalid(CodeFolderNotFound, ErrBadPath)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(p))
	if err != nil {
		return "", invalid(statCode(err), err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", invalid(statCode(err), err)
	}
	if !info.IsDir() {
		return "", invalid(CodeFolderNotFound, errors.New("not a directory"))
	}
	if err := canList(resolved); err != nil {
		return "", invalid(CodeFolderUnreadable, err)
	}
	return resolved, nil
}

func statCode(err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return CodeFolderUnreadable
	}
	return CodeFolderNotFound
}

// canList opens a directory and reads one entry: that needs both read and
// execute permission.
func canList(dir string) error {
	f, err := os.Open(dir) //nolint:gosec // reading a directory listing is the point
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.ReadDir(1); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// overlaps reports whether a and b are the same folder or one contains the other.
func overlaps(a, b string) bool {
	return a == b || within(a, b) || within(b, a)
}

// checkOverlap rejects folders that overlap each other or a folder of
// another library. existing maps paths to library ids; selfID is the
// library being saved ("" on creation).
func checkOverlap(folders []string, existing map[string]string, selfID string) error {
	for i, f := range folders {
		for _, g := range folders[i+1:] {
			if overlaps(f, g) {
				return &FolderOverlapError{Path: g, Other: f}
			}
		}
		for path, lib := range existing {
			if lib != selfID && overlaps(f, path) {
				return &FolderOverlapError{Path: f, Other: path}
			}
		}
	}
	return nil
}

// BrowseEntry is one sub-folder in the folder picker (R3).
type BrowseEntry struct {
	Name     string
	Path     string
	Readable bool
}

// BrowseResult is a folder picker listing.
type BrowseResult struct {
	// Path is "" when listing the configured roots.
	Path string
	// Parent is "" at "/", at a root, or when listing the roots.
	Parent  string
	Entries []BrowseEntry
}

// Browse lists the sub-folders of p, directory names only (R3, NFR
// security). With roots, p must be inside one of them (after resolving
// symlinks) and an empty p lists the roots; without, an empty p means "/".
func Browse(p string, roots []string) (BrowseResult, error) {
	if p == "" && len(roots) > 0 {
		res := BrowseResult{}
		for _, r := range roots {
			res.Entries = append(res.Entries, BrowseEntry{Name: r, Path: r, Readable: canList(r) == nil})
		}
		return res, nil
	}
	if p == "" {
		p = string(filepath.Separator)
	}
	if !filepath.IsAbs(p) || hasDotDot(p) {
		return BrowseResult{}, ErrBadPath
	}
	p = filepath.Clean(p)
	atRoot := false
	if len(roots) > 0 {
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			return BrowseResult{}, ErrNotFound
		}
		inside := false
		for _, r := range roots {
			rr, err := filepath.EvalSymlinks(r)
			if err != nil {
				continue
			}
			if resolved == rr || within(rr, resolved) {
				inside = true
			}
			atRoot = atRoot || p == r
		}
		if !inside {
			return BrowseResult{}, ErrNotFound
		}
	}
	des, err := os.ReadDir(p)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return BrowseResult{}, &FolderInvalidError{Code: CodeFolderUnreadable, Path: p, Err: err}
		}
		return BrowseResult{}, ErrNotFound
	}
	res := BrowseResult{Path: p, Entries: []BrowseEntry{}}
	if parent := filepath.Dir(p); parent != p && !atRoot {
		res.Parent = parent
	}
	for _, de := range des {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		full := filepath.Join(p, de.Name())
		if !de.IsDir() {
			if de.Type()&fs.ModeSymlink == 0 {
				continue
			}
			if info, err := os.Stat(full); err != nil || !info.IsDir() {
				continue
			}
		}
		res.Entries = append(res.Entries, BrowseEntry{Name: de.Name(), Path: full, Readable: canList(full) == nil})
	}
	slices.SortFunc(res.Entries, func(a, b BrowseEntry) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return res, nil
}
