package probe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"
)

func hostPort(t *testing.T, rawURL string) (netip.Addr, int) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	ap, err := netip.ParseAddrPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	return ap.Addr(), int(ap.Port())
}

func TestCheck(t *testing.T) {
	plaid := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"0.3.0"}`))
	}))
	defer plaid.Close()
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer other.Close()
	// Something that isn't TLS and talks first (a plain-HTTP error page).
	plain, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Close()
	go func() {
		for {
			c, err := plain.Accept()
			if err != nil {
				return
			}
			_, _ = c.Write([]byte("HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n"))
			c.Close()
		}
	}()

	// A port nothing listens on.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	p := &Prober{AllowPrivate: true, Timeout: 2 * time.Second}
	ctx := context.Background()

	ip, port := hostPort(t, plaid.URL)
	if r := p.Check(ctx, ip, port); !r.Reachable {
		t.Fatalf("plaid: %v", r)
	}
	ip, port = hostPort(t, other.URL)
	if r := p.Check(ctx, ip, port); r.Reachable || r.Error != ErrNotPlaid {
		t.Fatalf("other: %v", r)
	}
	ip, port = hostPort(t, "http://"+plain.Addr().String())
	if r := p.Check(ctx, ip, port); r.Reachable || r.Error != ErrTLS {
		t.Fatalf("plain http: %v", r)
	}
	if r := p.Check(ctx, netip.MustParseAddr("127.0.0.1"), closedPort); r.Error != ErrRefused {
		t.Fatalf("closed port: %v", r)
	}

	hang := &Prober{AllowPrivate: true, Timeout: 100 * time.Millisecond,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
	if r := hang.Check(ctx, netip.MustParseAddr("127.0.0.1"), 1); r.Error != ErrTimeout {
		t.Fatalf("hang: %v", r)
	}

	// Without AllowPrivate, private targets are refused before any dial.
	dialed := false
	guarded := &Prober{Dial: func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, net.ErrClosed
	}}
	for _, a := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.2", "100.64.0.1", "169.254.1.1", "::1", "fd00::1", "fe80::1"} {
		if r := guarded.Check(ctx, netip.MustParseAddr(a), 443); r.Error != ErrRefused {
			t.Fatalf("%s: %v", a, r)
		}
	}
	if dialed {
		t.Fatal("guard dialed a private address")
	}
}

func TestPublic(t *testing.T) {
	for a, want := range map[string]bool{
		"203.0.114.7": true, "8.8.8.8": true, "2a01:e0a::1": true,
		"10.1.2.3": false, "172.16.0.1": false, "192.168.0.1": false, "100.100.0.1": false,
		"127.0.0.1": false, "0.0.0.0": false, "::": false, "fc00::1": false, "::ffff:10.0.0.1": false,
	} {
		if got := Public(netip.MustParseAddr(a)); got != want {
			t.Errorf("Public(%s) = %v, want %v", a, got, want)
		}
	}
}
