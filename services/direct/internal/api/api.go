// Package api serves the getplaid.direct HTTP API (/api/v1), short links
// (/<label>) and the health check.
package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gh-prim/plaid/services/direct/internal/probe"
	"github.com/gh-prim/plaid/services/direct/internal/ratelimit"
	"github.com/gh-prim/plaid/services/direct/internal/slug"
	"github.com/gh-prim/plaid/services/direct/internal/store"
)

// ACMETTL is how long DNS-01 values live when the server doesn't clear them.
const ACMETTL = time.Hour

const (
	maxBody       = 8 << 10
	maxName       = 200
	maxVersion    = 40
	maxACMEValues = 4
	maxACMEValue  = 128
)

// Store is what the API needs from the database.
type Store interface {
	Create(ctx context.Context, srv *store.Server, newID func() string) error
	Get(ctx context.Context, id string) (store.Server, error)
	Update(ctx context.Context, srv store.Server) error
	Delete(ctx context.Context, id string) error
	SetACME(ctx context.Context, id string, values []string, expires time.Time) error
	ClearACME(ctx context.Context, id string) error
	Ping(ctx context.Context) error
}

// Checker runs reachability probes.
type Checker interface {
	Check(ctx context.Context, ip netip.Addr, port int) probe.Result
}

// Config wires the handler.
type Config struct {
	Store          Store
	Prober         Checker
	Zone           string         // e.g. getplaid.direct
	SiteURL        string         // linked from the short-link 404 page
	TrustedProxies []netip.Prefix // peers whose X-Forwarded-For is believed
	Logger         *slog.Logger
	Now            func() time.Time   // defaults to time.Now
	RegisterLimit  *ratelimit.Limiter // per source IP; 5 per hour when nil
	ProbeLimit     *ratelimit.Limiter // per server; 10 per hour when nil
	NewID          func() string      // defaults to slug.NewID
}

type handler struct {
	cfg Config
	log *slog.Logger
}

// New returns the service's HTTP handler.
func New(cfg Config) http.Handler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.RegisterLimit == nil {
		cfg.RegisterLimit = ratelimit.New(5, time.Hour)
	}
	if cfg.ProbeLimit == nil {
		cfg.ProbeLimit = ratelimit.New(10, time.Hour)
	}
	if cfg.NewID == nil {
		cfg.NewID = slug.NewID
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	h := &handler{cfg: cfg, log: cfg.Logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /api/v1/ip", h.ip)
	mux.HandleFunc("POST /api/v1/servers", h.register)
	mux.HandleFunc("PUT /api/v1/servers/{id}", h.auth(h.update))
	mux.HandleFunc("DELETE /api/v1/servers/{id}", h.auth(h.unregister))
	mux.HandleFunc("PUT /api/v1/servers/{id}/acme-challenge", h.auth(h.setACME))
	mux.HandleFunc("DELETE /api/v1/servers/{id}/acme-challenge", h.auth(h.clearACME))
	mux.HandleFunc("POST /api/v1/servers/{id}/reachability", h.auth(h.reachability))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})
	mux.HandleFunc("GET /{label}", h.shortLink)
	mux.HandleFunc("GET /{$}", h.home)
	return h.logRequests(mux)
}

// serverView is the JSON shape of a server, shared by register and update.
type serverView struct {
	ID       string `json:"id"`
	Token    string `json:"token,omitempty"`
	Hostname string `json:"hostname"`
	URL      string `json:"url"`
	ShortURL string `json:"shortUrl"`
	PublicIP string `json:"publicIp"`
	Port     int    `json:"port"`
}

func (h *handler) view(s store.Server, ip netip.Addr) serverView {
	host := s.Label() + "." + h.cfg.Zone
	return serverView{
		ID:       s.ID,
		Hostname: host,
		URL:      httpsURL(host, s.Port),
		ShortURL: "https://" + h.cfg.Zone + "/" + s.Label(),
		PublicIP: ip.String(),
		Port:     s.Port,
	}
}

// httpsURL leaves the port out when it is the HTTPS default.
func httpsURL(host string, port int) string {
	if port == 443 {
		return "https://" + host
	}
	return "https://" + host + ":" + strconv.Itoa(port)
}

type serverBody struct {
	Name    *string `json:"name"`
	Port    *int    `json:"port"`
	Version *string `json:"version"`
}

func (b serverBody) validate() string {
	if b.Name != nil && (len(*b.Name) > maxName || !utf8.ValidString(*b.Name)) {
		return "name must be valid UTF-8, at most 200 bytes"
	}
	if b.Port != nil && (*b.Port < 1 || *b.Port > 65535) {
		return "port must be between 1 and 65535"
	}
	if b.Version != nil && (len(*b.Version) > maxVersion || !printable(*b.Version)) {
		return "version must be printable ASCII, at most 40 characters"
	}
	return ""
}

func printable(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	ip, ok := h.clientIP(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "unknown source address")
		return
	}
	var body serverBody
	if !decode(w, r, &body, false) {
		return
	}
	if body.Port == nil {
		writeError(w, http.StatusBadRequest, "bad_request", "port is required")
		return
	}
	if msg := body.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, "bad_request", msg)
		return
	}
	now := h.cfg.Now()
	if !h.cfg.RegisterLimit.Allow(limitKey(ip), now) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many registrations from this address, try again later")
		return
	}

	token, hash := newToken()
	srv := store.Server{
		Slug:      slug.Make(deref(body.Name)),
		TokenHash: hash,
		Port:      *body.Port,
		Version:   deref(body.Version),
		CreatedAt: now,
	}
	srv.Seen(ip, now)
	if err := h.cfg.Store.Create(r.Context(), &srv, h.cfg.NewID); err != nil {
		h.internal(w, r, err)
		return
	}
	h.log.Info("server registered", "id", srv.ID, "label", srv.Label(), "ip", ip.String(), "port", srv.Port, "version", srv.Version)
	v := h.view(srv, ip)
	v.Token = token
	writeJSON(w, http.StatusCreated, v)
}

func (h *handler) update(w http.ResponseWriter, r *http.Request, srv store.Server, ip netip.Addr) {
	var body serverBody
	if !decode(w, r, &body, true) {
		return
	}
	if msg := body.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, "bad_request", msg)
		return
	}
	if body.Name != nil {
		srv.Slug = slug.Make(*body.Name)
	}
	if body.Port != nil {
		srv.Port = *body.Port
	}
	if body.Version != nil {
		srv.Version = *body.Version
	}
	srv.Seen(ip, h.cfg.Now())
	if err := h.cfg.Store.Update(r.Context(), srv); err != nil {
		h.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.view(srv, ip))
}

func (h *handler) unregister(w http.ResponseWriter, r *http.Request, srv store.Server, _ netip.Addr) {
	if err := h.cfg.Store.Delete(r.Context(), srv.ID); err != nil {
		h.internal(w, r, err)
		return
	}
	h.log.Info("server unregistered", "id", srv.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) setACME(w http.ResponseWriter, r *http.Request, srv store.Server, _ netip.Addr) {
	var body struct {
		Values []string `json:"values"`
	}
	if !decode(w, r, &body, false) {
		return
	}
	if len(body.Values) < 1 || len(body.Values) > maxACMEValues {
		writeError(w, http.StatusBadRequest, "bad_request", "values must hold 1 to 4 entries")
		return
	}
	for _, v := range body.Values {
		if !base64URL(v) {
			writeError(w, http.StatusBadRequest, "bad_request", "each value must be 1 to 128 base64url characters")
			return
		}
	}
	if err := h.cfg.Store.SetACME(r.Context(), srv.ID, body.Values, h.cfg.Now().Add(ACMETTL)); err != nil {
		h.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func base64URL(s string) bool {
	if s == "" || len(s) > maxACMEValue {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return true
}

func (h *handler) clearACME(w http.ResponseWriter, r *http.Request, srv store.Server, _ netip.Addr) {
	if err := h.cfg.Store.ClearACME(r.Context(), srv.ID); err != nil {
		h.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) reachability(w http.ResponseWriter, r *http.Request, srv store.Server, ip netip.Addr) {
	if !h.cfg.ProbeLimit.Allow(srv.ID, h.cfg.Now()) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many reachability checks for this server, try again later")
		return
	}
	// The caller's own address is the server's public IP by definition.
	res := h.cfg.Prober.Check(r.Context(), ip, srv.Port)
	writeJSON(w, http.StatusOK, struct {
		Reachable bool   `json:"reachable"`
		PublicIP  string `json:"publicIp"`
		Port      int    `json:"port"`
		Error     string `json:"error,omitempty"`
	}{res.Reachable, ip.String(), srv.Port, res.Error})
}

func (h *handler) ip(w http.ResponseWriter, r *http.Request) {
	ip, ok := h.clientIP(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "unknown source address")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ip": ip.String()})
}

func (h *handler) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.cfg.Store.Ping(ctx); err != nil {
		h.log.Error("health check failed", "err", err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

// home sends visitors of the bare domain to the product site.
func (h *handler) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.cfg.SiteURL, http.StatusFound)
}

// shortLink redirects getplaid.direct/<label> to the server. The id alone
// identifies the server, so links with an old slug still work after a rename
// and land on the current hostname.
func (h *handler) shortLink(w http.ResponseWriter, r *http.Request) {
	_, id, ok := slug.Parse(strings.ToLower(r.PathValue("label")))
	if !ok {
		h.notFound(w)
		return
	}
	srv, err := h.cfg.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w)
		return
	}
	if err != nil {
		h.log.Error("short link lookup failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store") // the port can change
	// The target is always a host under our zone, built from stored values.
	http.Redirect(w, r, httpsURL(srv.Label()+"."+h.cfg.Zone, srv.Port)+"/", http.StatusFound) //nolint:gosec // see above
}

var notFoundPage = template.Must(template.New("404").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Server not found · PLAID</title>
<style>
body{margin:0;min-height:100vh;display:grid;place-items:center;font-family:system-ui,sans-serif;background:#14100f;color:#f4ece4;text-align:center;padding:16px}
a{color:#e8a25a}
</style>
</head>
<body>
<main>
<h1>This PLAID server doesn't exist</h1>
<p>The link may be mistyped, or the server stopped using getplaid.direct.</p>
<p><a href="{{.}}">What is PLAID?</a></p>
</main>
</body>
</html>
`))

func (h *handler) notFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = notFoundPage.Execute(w, h.cfg.SiteURL)
}

type authedHandler func(w http.ResponseWriter, r *http.Request, srv store.Server, ip netip.Addr)

// dummyHash is compared against when the id is unknown, so both failures
// take the same time and answer the same 401.
var dummyHash = sha256.Sum256([]byte("getplaid.direct"))

func (h *handler) auth(next authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, ok := h.clientIP(r)
		if !ok {
			writeError(w, http.StatusBadRequest, "bad_request", "unknown source address")
			return
		}
		token, hasToken := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		presented := sha256.Sum256([]byte(token))

		id := r.PathValue("id")
		var (
			srv store.Server
			err = store.ErrNotFound
		)
		if slug.ValidID(id) {
			srv, err = h.cfg.Store.Get(r.Context(), id)
		}
		want := dummyHash[:]
		switch {
		case err == nil:
			want = srv.TokenHash
		case !errors.Is(err, store.ErrNotFound):
			h.internal(w, r, err)
			return
		}
		match := subtle.ConstantTimeCompare(presented[:], want) == 1
		if !hasToken || token == "" || err != nil || !match {
			writeError(w, http.StatusUnauthorized, "unauthorized", "unknown server or wrong token")
			return
		}
		next(w, r, srv, ip)
	}
}

// newToken returns a 32-byte random token, base64url without padding (43
// characters), and its SHA-256: only the hash is stored.
func newToken() (string, []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never fails (Go 1.24+)
	token := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:]
}

// limitKey groups IPv6 sources by /64, the smallest block a home usually
// gets: otherwise one host could rotate addresses past the limit.
func limitKey(ip netip.Addr) string {
	if ip.Is6() {
		p, _ := ip.Prefix(64)
		return p.String()
	}
	return ip.String()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// decode reads a JSON body of at most maxBody bytes. With allowEmpty, an
// empty body counts as {}.
func decode(w http.ResponseWriter, r *http.Request, v any, allowEmpty bool) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	err := json.NewDecoder(r.Body).Decode(v)
	if err == nil || (allowEmpty && errors.Is(err, io.EOF)) {
		return true
	}
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
		return false
	}
	writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
	return false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func (h *handler) internal(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "internal error")
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs one line per request and turns panics into 500s. It never
// logs headers: the Authorization header carries the token.
func (h *handler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler { //nolint:errorlint // sentinel compared by identity, as net/http does
					panic(p)
				}
				h.log.Error("panic", "method", r.Method, "path", r.URL.Path, "panic", p)
				writeError(sw, http.StatusInternalServerError, "internal", "internal error")
			}
			if r.URL.Path == "/healthz" && sw.status == http.StatusOK {
				return // Coolify polls it every few seconds
			}
			ip, _ := h.clientIP(r)
			h.log.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status,
				"ms", time.Since(start).Milliseconds(), "ip", ip.String())
		}()
		next.ServeHTTP(sw, r)
	})
}
