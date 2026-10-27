// Package tmdbtest serves the recorded TMDB and fanart.tv responses
// (testdata/tmdb and testdata/fanart, written by
// scripts/record-metadata-fixtures.sh) and generated images from a local
// TLS server, for tests only. A request with no recording fails the test,
// so tests can't silently depend on the network.
package tmdbtest

import (
	"bytes"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/gh-prim/plaid/server/internal/metadata/fanart"
	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdb"
)

// Keys tests use.
const (
	Key       = "test-key-0000"
	FanartKey = "fanart-test-key"
)

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
	fanartPath = regexp.MustCompile(`^/v3/(movies|tv)/(\d+)$`)
	imagePath  = regexp.MustCompile(`^/(?:t/p/([a-z0-9]+)|fanart)/([^/]+)$`)
)

// New starts a server; it stops with the test.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		if m := imagePath.FindStringSubmatch(r.URL.Path); m != nil {
			s.mu.Lock()
			s.paths = append(s.paths, r.URL.Path)
			s.mu.Unlock()
			if code := s.Fail.Load(); code != 0 {
				w.WriteHeader(int(code))
				return
			}
			serveImage(w, m[1], m[2])
			return
		}
		q := r.URL.Query()
		if m := fanartPath.FindStringSubmatch(r.URL.Path); m != nil {
			if q.Get("api_key") != FanartKey {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			s.mu.Lock()
			s.paths = append(s.paths, r.URL.Path)
			s.mu.Unlock()
			body, err := os.ReadFile(filepath.Join(Dir(), "..", "fanart", m[1]+"-"+m[2]+".json")) //nolint:gosec // test fixtures
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(body) //nolint:gosec // recorded JSON, test only
			return
		}
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
	return tmdb.New(s.httpClient("tmdb"), s.URL, Key)
}

// serveImage answers a generated image: PNG for .png names, JPEG else.
// TMDB sizes (w780, h632, original) set the dimensions; the color comes
// from the name, so the same URL always gives the same bytes.
func serveImage(w http.ResponseWriter, size, name string) {
	width, height := 1000, 1500
	switch {
	case size == "w1280":
		width, height = 1280, 720
	case strings.HasPrefix(size, "w"):
		width, _ = strconv.Atoi(size[1:])
		height = width * 3 / 2
	case strings.HasPrefix(size, "h"):
		height, _ = strconv.Atoi(size[1:])
		width = height * 2 / 3
	}
	if strings.HasSuffix(name, ".png") {
		width, height = 800, 310 // logos and clearart
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	sum := h.Sum32()
	c := color.NRGBA{R: uint8(sum & 0xff), G: uint8(sum >> 8 & 0xff), B: uint8(sum >> 16 & 0xff), A: 255} //nolint:gosec // masked to 8 bits
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	if strings.HasSuffix(name, ".png") {
		// A transparent band, like a logo's empty space.
		draw.Draw(img, image.Rect(0, 0, width/4, height), image.Transparent, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if strings.HasSuffix(name, ".png") {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(&buf, img)
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
		_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	}
	_, _ = w.Write(buf.Bytes())
}

// Fanart returns a fanart.tv client pointed at s.
func (s *Server) Fanart() *fanart.Client {
	return fanart.New(s.httpClient("fanart"), s.URL+"/v3", FanartKey, "")
}

// ImageHosts maps the real image hosts to s, for the artwork downloader.
func (s *Server) ImageHosts() map[string]string {
	return map[string]string{"image.tmdb.org": s.URL, "assets.fanart.tv": s.URL}
}

func (s *Server) httpClient(name string) *httpx.Client {
	return httpx.New(httpx.Options{
		Name: name, Hosts: []string{"127.0.0.1"}, Transport: s.Server.Client().Transport,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// HTTP returns an httpx client allowed to reach s, for downloads.
func (s *Server) HTTP() *httpx.Client { return s.httpClient("images") }
