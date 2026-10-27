// Package tmdbtest serves the recorded TMDB responses
// (testdata/tmdb, written by scripts/record-metadata-fixtures.sh) from a
// local TLS server, for tests only. A request with no recording fails the
// test, so tests can't silently depend on the network.
package tmdbtest

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdb"
)

// Key is the API key tests use.
const Key = "test-key-0000"

// Dir is the fixtures directory.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "tmdb")
}

// Server replays fixtures. Fail, when set, makes every request answer that
// status (0 = replay normally).
type Server struct {
	*httptest.Server
	Fail     atomic.Int64
	requests atomic.Int64
	mu       sync.Mutex
	paths    []string
	failOn   string
	failCode int
}

// FailPaths makes requests whose path contains substr answer code; an
// empty substr stops it.
func (s *Server) FailPaths(substr string, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failOn, s.failCode = substr, code
}

// Requests returns how many requests reached the server.
func (s *Server) Requests() int { return int(s.requests.Load()) }

// Paths returns the request paths with their query (without the key), in order.
func (s *Server) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...)
}

var (
	seasonPath = regexp.MustCompile(`^/tv/(\d+)/season/(\d+)$`)
	detailPath = regexp.MustCompile(`^/(movie|tv|collection)/(\d+)$`)
	findPath   = regexp.MustCompile(`^/find/(tt\d+)$`)
)

// New starts a server; it stops with the test.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		q := r.URL.Query()
		if q.Get("api_key") != Key {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		q.Del("api_key")
		s.mu.Lock()
		s.paths = append(s.paths, r.URL.Path+"?"+q.Encode())
		failOn, failCode := s.failOn, s.failCode
		s.mu.Unlock()
		if failOn != "" && strings.Contains(r.URL.Path, failOn) {
			w.WriteHeader(failCode)
			return
		}
		if code := s.Fail.Load(); code != 0 {
			w.WriteHeader(int(code))
			return
		}
		file := fixture(r.URL.Path, q)
		if file == "" {
			t.Errorf("tmdbtest: no fixture for %s?%s", r.URL.Path, q.Encode())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := os.ReadFile(filepath.Join(Dir(), file)) //nolint:gosec // test fixtures, names built from fixed patterns
		if os.IsNotExist(err) {
			if strings.HasPrefix(file, "search/") {
				// The recording script only keeps searches it ran.
				t.Errorf("tmdbtest: search not recorded: %s", file)
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			t.Errorf("tmdbtest: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body) //nolint:gosec // recorded JSON, test only
	}))
	t.Cleanup(s.Close)
	return s
}

func fixture(path string, q map[string][]string) string {
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	lang := get("language")
	switch {
	case strings.HasPrefix(path, "/search/"):
		kind := strings.TrimPrefix(path, "/search/")
		year := get("year") + get("first_air_date_year")
		if year == "" {
			year = "any"
		}
		return "search/" + kind + "-" + Slug(get("query")) + "-" + year + ".json"
	case seasonPath.MatchString(path):
		m := seasonPath.FindStringSubmatch(path)
		return "tv/" + m[1] + "-s" + m[2] + "-" + lang + ".json"
	case detailPath.MatchString(path):
		m := detailPath.FindStringSubmatch(path)
		return m[1] + "/" + m[2] + "-" + lang + ".json"
	case findPath.MatchString(path):
		return "find/" + findPath.FindStringSubmatch(path)[1] + ".json"
	}
	return ""
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slug names search fixtures the way the recording script does.
func Slug(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.Trim(nonAlnum.ReplaceAllString(b.String(), "-"), "-")
}

// Client returns a TMDB client pointed at s.
func (s *Server) Client() *tmdb.Client {
	h := httpx.New(httpx.Options{
		Name: "tmdb", Hosts: []string{"127.0.0.1"}, Transport: s.Server.Client().Transport,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return tmdb.New(h, s.URL, Key)
}
