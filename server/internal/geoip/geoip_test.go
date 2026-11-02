package geoip

import (
	"bytes"
	"compress/gzip"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

// testDB is a tiny database in DB-IP's layout: 81.2.69.0/24 is London.
func testDB(t *testing.T) []byte {
	t.Helper()
	w, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "DBIP-City-Lite", RecordSize: 24})
	if err != nil {
		t.Fatal(err)
	}
	_, network, _ := net.ParseCIDR("81.2.69.0/24")
	err = w.Insert(network, mmdbtype.Map{
		"country": mmdbtype.Map{"iso_code": mmdbtype.String("GB")},
		"city":    mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String("London")}},
		"location": mmdbtype.Map{
			"latitude": mmdbtype.Float64(51.5), "longitude": mmdbtype.Float64(-0.12),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := w.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownloadAndLookup(t *testing.T) {
	db := testDB(t)
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		// This month isn't out yet: last month's file is used.
		if !strings.Contains(r.URL.Path, "2026-10") {
			http.NotFound(w, r)
			return
		}
		gz := gzip.NewWriter(w)
		_, _ = gz.Write(db)
		_ = gz.Close()
	}))
	defer srv.Close()

	l := &Locator{Dir: t.TempDir(), URL: srv.URL + "/dbip-city-lite-%s.mmdb.gz",
		Now: func() time.Time { return time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC) }}
	if _, ok := l.Lookup("81.2.69.160"); ok || l.Ready() {
		t.Fatal("found a place without a database")
	}
	if !l.stale() {
		t.Fatal("a missing database isn't stale")
	}
	if err := l.Download(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 2 {
		t.Errorf("asked %v, want this month then last month", asked)
	}
	p, ok := l.Lookup("81.2.69.160")
	if !ok || p.Country != "GB" || p.City != "London" || p.Lat != 51.5 {
		t.Errorf("lookup = %+v, %v", p, ok)
	}
	if l.stale() {
		t.Error("a fresh database is stale")
	}
	for _, ip := range []string{"192.168.1.20", "10.0.0.1", "100.100.1.1", "127.0.0.1", "fe80::1", "::ffff:192.168.1.2", "nonsense", "8.8.8.8"} {
		if p, ok := l.Lookup(ip); ok {
			t.Errorf("%s placed at %+v", ip, p)
		}
	}
}

func TestDownloadRefusesGarbage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte("not a database"))
		_ = gz.Close()
	}))
	defer srv.Close()
	l := &Locator{Dir: t.TempDir(), URL: srv.URL + "/%s"}
	if err := l.Download(context.Background()); err == nil || l.Ready() {
		t.Fatalf("garbage accepted: %v", err)
	}
}

func TestPublic(t *testing.T) {
	for ip, want := range map[string]bool{"81.2.69.160": true, "2a01:cb00::1": true, "192.168.0.1": false,
		"172.16.4.4": false, "100.64.0.1": false, "::1": false, "169.254.1.1": false} {
		if got := Public(netip.MustParseAddr(ip)); got != want {
			t.Errorf("Public(%s) = %v", ip, got)
		}
	}
}
