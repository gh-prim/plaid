// Package geoip places a remote stream on the map (admin monitoring): a
// city and approximate coordinates from the client's address, with the
// free DB-IP Lite city database (CC BY 4.0), downloaded once a month
// into the data folder. Local and private addresses are never looked up.
package geoip

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// FileName is the database inside the folder.
const FileName = "dbip-city-lite.mmdb"

// DefaultURL is DB-IP's monthly free file; %s is the month, 2026-11.
const DefaultURL = "https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz"

// refreshAfter is the database's age that triggers a new download.
const refreshAfter = 32 * 24 * time.Hour

// Place is where an address is, roughly (a city, never a street).
type Place struct {
	Country string // ISO 3166-1 alpha-2
	City    string
	Lat     float64
	Lon     float64
}

// Locator looks addresses up. Its zero value finds nothing.
type Locator struct {
	Dir string
	// URL is the download template (DefaultURL); tests point it at a stub.
	URL    string
	Client *http.Client
	Log    *slog.Logger
	Now    func() time.Time

	mu     sync.RWMutex
	reader *maxminddb.Reader
}

type record struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

// Open loads the database already downloaded, if any.
func (l *Locator) Open() error {
	r, err := maxminddb.Open(filepath.Join(l.Dir, FileName))
	if err != nil {
		return err
	}
	l.mu.Lock()
	old := l.reader
	l.reader = r
	l.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Ready says whether lookups can find anything.
func (l *Locator) Ready() bool {
	if l == nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.reader != nil
}

// Lookup places a public address; ok is false for a private one, an
// unknown one, or without a database.
func (l *Locator) Lookup(ip string) (Place, bool) {
	if l == nil {
		return Place{}, false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || !Public(addr) {
		return Place{}, false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.reader == nil {
		return Place{}, false
	}
	var rec record
	if err := l.reader.Lookup(net.IP(addr.AsSlice()), &rec); err != nil || rec.Country.ISOCode == "" {
		return Place{}, false
	}
	city := rec.City.Names["en"]
	return Place{Country: rec.Country.ISOCode, City: city, Lat: rec.Location.Latitude, Lon: rec.Location.Longitude}, true
}

// Public is false for loopback, private, CGNAT, link-local and other
// addresses that say nothing about a place.
func Public(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified() {
		return false
	}
	return !netip.MustParsePrefix("100.64.0.0/10").Contains(a)
}

// Run opens the database, downloads it when missing or older than a
// month, then checks again every day, until ctx ends.
func (l *Locator) Run(ctx context.Context) {
	if err := l.Open(); err != nil && !errors.Is(err, os.ErrNotExist) {
		l.log().Warn("geoip database unreadable", "err", err)
	}
	for {
		if l.stale() {
			if err := l.Download(ctx); err != nil {
				l.log().Warn("geoip download failed", "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

func (l *Locator) stale() bool {
	st, err := os.Stat(filepath.Join(l.Dir, FileName))
	return err != nil || l.now().Sub(st.ModTime()) > refreshAfter
}

// Download fetches this month's database (last month's while this one
// isn't out yet) and swaps it in.
func (l *Locator) Download(ctx context.Context) error {
	if err := os.MkdirAll(l.Dir, 0o750); err != nil {
		return err
	}
	now := l.now().UTC()
	var last error
	for _, month := range []time.Time{now, now.AddDate(0, -1, 0)} {
		if last = l.fetch(ctx, fmt.Sprintf(l.url(), month.Format("2006-01"))); last == nil {
			l.log().Info("geoip database updated", "month", month.Format("2006-01"))
			return l.Open()
		}
	}
	return last
}

func (l *Locator) fetch(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := l.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("geoip: %s answered %d", url, res.StatusCode)
	}
	gz, err := gzip.NewReader(res.Body)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(l.Dir, "dbip-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	// The file is about 130 MB; a cap keeps a bad answer from filling the disk.
	if _, err := io.Copy(tmp, io.LimitReader(gz, 1<<30)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Refuse a file that doesn't open as a database.
	r, err := maxminddb.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("geoip: downloaded file unreadable: %w", err)
	}
	_ = r.Close()
	return os.Rename(tmp.Name(), filepath.Join(l.Dir, FileName))
}

func (l *Locator) url() string {
	if l.URL != "" {
		return l.URL
	}
	return DefaultURL
}

func (l *Locator) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

func (l *Locator) log() *slog.Logger {
	if l.Log != nil {
		return l.Log
	}
	return slog.Default()
}
