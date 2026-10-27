package artwork

import (
	"container/list"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// touchEvery limits how often a served variant's mtime is updated, so the
// LRU order survives restarts without a write per request (R20).
const touchEvery = 24 * time.Hour

// variantCache tracks variants on disk, least recently served first, and
// evicts past its byte cap. Its index is rebuilt from the directory.
type variantCache struct {
	dir string
	max int64
	log *slog.Logger

	mu      sync.Mutex
	loaded  bool
	order   *list.List // front = most recent
	entries map[string]*list.Element
	size    int64
}

type variantEntry struct {
	key     string // sha/width.ext
	size    int64
	touched time.Time
}

func newVariantCache(dir string, maxBytes int64, log *slog.Logger) *variantCache {
	return &variantCache{dir: dir, max: maxBytes, log: log, order: list.New(), entries: map[string]*list.Element{}}
}

func (c *variantCache) path(key string) string { return filepath.Join(c.dir, filepath.FromSlash(key)) }

// load indexes what's on disk, oldest mtime last. Called under mu.
func (c *variantCache) load() {
	if c.loaded {
		return
	}
	c.loaded = true
	var found []variantEntry
	_ = filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		rel, _ := filepath.Rel(c.dir, p)
		found = append(found, variantEntry{key: filepath.ToSlash(rel), size: info.Size(), touched: info.ModTime()})
		return nil
	})
	sort.Slice(found, func(i, j int) bool { return found[i].touched.After(found[j].touched) }) // most recent first
	for _, e := range found {
		c.entries[e.key] = c.order.PushBack(&variantEntry{key: e.key, size: e.size, touched: e.touched})
		c.size += e.size
	}
}

func (c *variantCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	el, ok := c.entries[key]
	if !ok {
		return "", false
	}
	c.order.MoveToFront(el)
	e := el.Value.(*variantEntry)
	if now := time.Now(); now.Sub(e.touched) > touchEvery {
		e.touched = now
		_ = os.Chtimes(c.path(key), now, now)
	}
	return c.path(key), true
}

func (c *variantCache) put(key string, body []byte) (string, error) {
	p := c.path(key)
	if err := writeAtomic(p, body); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	if el, ok := c.entries[key]; ok {
		c.size -= el.Value.(*variantEntry).size
		c.order.Remove(el)
	}
	c.entries[key] = c.order.PushFront(&variantEntry{key: key, size: int64(len(body)), touched: time.Now()})
	c.size += int64(len(body))
	for c.size > c.max && c.order.Len() > 1 {
		el := c.order.Back()
		e := el.Value.(*variantEntry)
		if err := os.Remove(c.path(e.key)); err != nil && !os.IsNotExist(err) {
			c.log.Warn("artwork variant not evicted", "path", c.path(e.key), "err", err)
			break
		}
		c.order.Remove(el)
		delete(c.entries, e.key)
		c.size -= e.size
	}
	return p, nil
}

// dropSHA forgets (and deletes) every variant of an original.
func (c *variantCache) dropSHA(sha string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.load()
	for key, el := range c.entries {
		if strings.HasPrefix(key, sha+"/") {
			_ = os.Remove(c.path(key))
			c.size -= el.Value.(*variantEntry).size
			c.order.Remove(el)
			delete(c.entries, key)
		}
	}
	_ = os.Remove(filepath.Join(c.dir, sha))
}
