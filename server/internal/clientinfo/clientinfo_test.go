package clientinfo

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

var trusted = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
}

func TestResolve(t *testing.T) {
	for _, tc := range []struct {
		name      string
		remote    string
		headers   map[string][]string
		tls       bool
		wantIP    string
		wantHTTPS bool
		wantUntr  bool
	}{
		{
			name:   "direct client",
			remote: "192.168.1.20:51000",
			wantIP: "192.168.1.20",
		},
		{
			name:      "direct TLS",
			remote:    "192.168.1.20:51000",
			tls:       true,
			wantIP:    "192.168.1.20",
			wantHTTPS: true,
		},
		{
			name:   "IPv4-mapped IPv6 is unmapped",
			remote: "[::ffff:192.168.1.20]:51000",
			wantIP: "192.168.1.20",
		},
		{
			name:     "forwarded headers ignored from an untrusted peer",
			remote:   "127.0.0.1:40000",
			headers:  map[string][]string{"X-Forwarded-For": {"203.0.113.9"}, "X-Forwarded-Proto": {"https"}},
			wantIP:   "127.0.0.1",
			wantUntr: true,
		},
		{
			name:     "Forwarded header from an untrusted peer is flagged",
			remote:   "127.0.0.1:40000",
			headers:  map[string][]string{"Forwarded": {"for=203.0.113.9"}},
			wantIP:   "127.0.0.1",
			wantUntr: true,
		},
		{
			name:     "X-Real-IP from an untrusted peer is flagged",
			remote:   "127.0.0.1:40000",
			headers:  map[string][]string{"X-Real-Ip": {"203.0.113.9"}},
			wantIP:   "127.0.0.1",
			wantUntr: true,
		},
		{
			name:      "trusted proxy",
			remote:    "10.0.0.2:40000",
			headers:   map[string][]string{"X-Forwarded-For": {"203.0.113.9"}, "X-Forwarded-Proto": {"https"}},
			wantIP:    "203.0.113.9",
			wantHTTPS: true,
		},
		{
			name:   "chain: rightmost untrusted hop wins, spoofed left part ignored",
			remote: "10.0.0.2:40000",
			headers: map[string][]string{
				"X-Forwarded-For": {"1.1.1.1, 203.0.113.9", "10.0.0.7"},
			},
			wantIP: "203.0.113.9",
		},
		{
			name:    "chain of trusted proxies only: leftmost",
			remote:  "[::1]:40000",
			headers: map[string][]string{"X-Forwarded-For": {"10.0.0.8, 10.0.0.7"}},
			wantIP:  "10.0.0.8",
		},
		{
			name:    "garbage entry stops the walk",
			remote:  "10.0.0.2:40000",
			headers: map[string][]string{"X-Forwarded-For": {"203.0.113.9, nonsense"}},
			wantIP:  "10.0.0.2",
		},
		{
			name:    "trusted proxy says http",
			remote:  "10.0.0.2:40000",
			headers: map[string][]string{"X-Forwarded-Proto": {"http"}},
			tls:     true,
			wantIP:  "10.0.0.2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote
			for k, vs := range tc.headers {
				for _, v := range vs {
					r.Header.Add(k, v)
				}
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			got := Resolve(r, trusted)
			if got.IP.String() != tc.wantIP || got.HTTPS != tc.wantHTTPS || got.UntrustedForward != tc.wantUntr {
				t.Errorf("got {%s https=%v untrusted=%v}, want {%s https=%v untrusted=%v}",
					got.IP, got.HTTPS, got.UntrustedForward, tc.wantIP, tc.wantHTTPS, tc.wantUntr)
			}
		})
	}
}

func TestMiddlewareStoresInfo(t *testing.T) {
	var got Info
	h := Middleware(nil, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = FromContext(r.Context())
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.20:51000"
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got.IP.String() != "192.168.1.20" {
		t.Errorf("IP = %s", got.IP)
	}
}

func TestFromContextWithoutInfo(t *testing.T) {
	if info := FromContext(t.Context()); info.IP.IsValid() || info.HTTPS {
		t.Errorf("got %+v, want zero Info", info)
	}
}
