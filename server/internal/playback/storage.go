package playback

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// sessionDirName matches the folders PLAID makes in the transcode folder:
// "plaid-<session id>-<generation>". Nothing else there is ever deleted.
var sessionDirName = regexp.MustCompile(`^plaid-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-\d+$`)

// Storage is the transcode folder (R27): one folder per session stream,
// emptied at startup, capped in size.
type Storage struct {
	Dir string
	// Free reports the free bytes of the folder's file system; tests
	// replace it.
	Free func(dir string) (uint64, error)

	defaultCap int64
}

// Open creates the folder, deletes the session folders a previous run
// left, and computes the default size cap from the free space.
func (st *Storage) Open() (purged int, err error) {
	if st.Free == nil {
		st.Free = freeSpace
	}
	if err := os.MkdirAll(st.Dir, 0o750); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(st.Dir)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() && sessionDirName.MatchString(e.Name()) {
			if err := os.RemoveAll(filepath.Join(st.Dir, e.Name())); err == nil {
				purged++
			}
		}
	}
	free, err := st.Free(st.Dir)
	if err != nil {
		return purged, err
	}
	st.defaultCap = DefaultStorageCap(free)
	return purged, nil
}

// DefaultCap is the cap computed at startup.
func (st *Storage) DefaultCap() int64 { return st.defaultCap }

// SessionDir is the folder of a session's stream generation.
func (st *Storage) SessionDir(sessionID string, gen int) string {
	return filepath.Join(st.Dir, "plaid-"+sessionID+"-"+strconv.Itoa(gen))
}

func freeSpace(dir string) (uint64, error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(dir, &s); err != nil {
		return 0, err
	}
	return uint64(s.Bavail) * uint64(s.Bsize), nil //nolint:gosec,unconvert // field types differ per OS
}

// dirSize is the total size of the files in a folder (not recursive:
// session folders are flat).
func dirSize(dir string) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var n int64
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() {
			n += info.Size()
		}
	}
	return n
}

// fileCache is a folder of generated files (WebVTT subtitles) capped in
// size, least recently used first out (R27). A file is built once even
// when several requests ask for it at the same time.
type fileCache struct {
	Dir string
	Max int64

	mu      sync.Mutex
	pending map[string]*sync.Mutex
}

// Get returns the path of the cached file name, building it with build
// (which writes to the path it's given) when it's missing.
func (c *fileCache) Get(ctx context.Context, name string, build func(ctx context.Context, dst string) error) (string, error) {
	path := filepath.Join(c.Dir, name)
	c.mu.Lock()
	if c.pending == nil {
		c.pending = map[string]*sync.Mutex{}
	}
	l, ok := c.pending[name]
	if !ok {
		l = &sync.Mutex{}
		c.pending[name] = l
	}
	c.mu.Unlock()
	l.Lock()
	defer l.Unlock()

	if _, err := os.Stat(path); err == nil {
		now := time.Now()
		_ = os.Chtimes(path, now, now) // recently used
		return path, nil
	}
	if err := os.MkdirAll(c.Dir, 0o750); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := build(ctx, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	c.prune(path)
	return path, nil
}

// prune deletes the least recently used files over the cap, never keep.
func (c *fileCache) prune(keep string) {
	type entry struct {
		path string
		size int64
		used time.Time
	}
	var (
		files []entry
		total int64
	)
	_ = filepath.WalkDir(c.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files = append(files, entry{p, info.Size(), info.ModTime()})
			total += info.Size()
		}
		return nil
	})
	if total <= c.Max {
		return
	}
	slices.SortFunc(files, func(a, b entry) int { return a.used.Compare(b.used) })
	for _, f := range files {
		if total <= c.Max {
			return
		}
		if f.path == keep {
			continue
		}
		if err := os.Remove(f.path); err == nil || errors.Is(err, fs.ErrNotExist) {
			total -= f.size
		}
	}
}
