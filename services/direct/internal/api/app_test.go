package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type connectionsView struct {
	ID          string       `json:"id"`
	Hostname    string       `json:"hostname"`
	Connections []connection `json:"connections"`
}

func (e *env) registerLAN(lanIP string, lanPort int) serverView {
	e.t.Helper()
	body := `{"name":"Living room","port":32501,"lanIp":"` + lanIP + `","lanPort":` + strconv.Itoa(lanPort) + `}`
	rec := e.do(call{method: "POST", path: "/api/v1/servers", body: body})
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("register: %d %s", rec.Code, rec.Body)
	}
	return decodeJSON[serverView](e.t, rec)
}

func TestLANAddressIsValidated(t *testing.T) {
	e := newEnv(t)
	for _, ip := range []string{"8.8.8.8", "fd00::1", "nope"} {
		rec := e.do(call{method: "POST", path: "/api/v1/servers", body: `{"port":32501,"lanIp":"` + ip + `"}`})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("lanIp %s: %d", ip, rec.Code)
		}
	}
}

func TestConnectionsListLANFirst(t *testing.T) {
	e := newEnv(t)
	v := e.registerLAN("192.168.1.20", 32501)
	rec := e.do(call{method: "GET", path: "/api/v1/servers/" + v.ID + "/connections", remote: "198.51.100.9:4000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("connections: %d %s", rec.Code, rec.Body)
	}
	got := decodeJSON[connectionsView](t, rec)
	host := "living-room-" + v.ID + ".getplaid.direct"
	want := []connection{{"https://192-168-1-20." + host + ":32501", true}, {"https://" + host + ":32501", false}}
	if got.Hostname != host || len(got.Connections) != 2 || got.Connections[0] != want[0] || got.Connections[1] != want[1] {
		t.Fatalf("connections %+v", got)
	}

	// Clearing the LAN address leaves the public one.
	if rec := e.do(call{method: "PUT", path: "/api/v1/servers/" + v.ID, token: v.Token, body: `{"lanIp":""}`}); rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	got = decodeJSON[connectionsView](t, e.do(call{method: "GET", path: "/api/v1/servers/" + v.ID + "/connections"}))
	if len(got.Connections) != 1 || got.Connections[0].Local {
		t.Fatalf("after clearing: %+v", got)
	}
	if rec := e.do(call{method: "GET", path: "/api/v1/servers/zzzzzz/connections"}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown server: %d", rec.Code)
	}
}

// Without the app built in, a visitor on the server's own network is sent to
// the LAN name: many routers can't loop back to their public address.
func TestShortLinkSendsHomeVisitorsToTheLANName(t *testing.T) {
	e := newEnv(t)
	v := e.registerLAN("192.168.1.20", 32511)
	host := "living-room-" + v.ID + ".getplaid.direct"
	home := e.do(call{method: "GET", path: "/" + v.ID}) // same source as the server
	if loc := home.Header().Get("Location"); loc != "https://192-168-1-20."+host+":32511/" {
		t.Errorf("at home: %q", loc)
	}
	away := e.do(call{method: "GET", path: "/" + v.ID, remote: "198.51.100.9:4000"})
	if loc := away.Header().Get("Location"); loc != "https://"+host+":32501/" {
		t.Errorf("away: %q", loc)
	}
}

func TestShortLinkServesTheApp(t *testing.T) {
	e := newEnv(t)
	app := fstest.MapFS{
		"index.html":           {Data: []byte(`<!doctype html><div id="root"></div>`)},
		"assets/app-1.js":      {Data: []byte("console.log(1)")},
		"manifest.webmanifest": {Data: []byte("{}")},
	}
	e.h = New(Config{Store: e.store, Prober: e.prober, Zone: "getplaid.direct", SiteURL: "https://getplaid.tv",
		TrustedProxies: DefaultTrustedProxies, Now: func() time.Time { return e.now }, App: app})
	v := e.register("Living room", 32501)

	for _, p := range []string{"/" + v.ID, "/" + v.ID + "/movies/abc"} {
		rec := e.do(call{method: "GET", path: p})
		csp := rec.Header().Get("Content-Security-Policy")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="root"`) || !strings.Contains(csp, "connect-src 'self' https://*.getplaid.direct:*") {
			t.Fatalf("%s: %d csp=%q %s", p, rec.Code, csp, rec.Body)
		}
	}
	// The label form lands on the id, where the app lives.
	if rec := e.do(call{method: "GET", path: "/living-room-" + v.ID}); rec.Code != http.StatusFound || rec.Header().Get("Location") != "/"+v.ID {
		t.Fatalf("label: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := e.do(call{method: "GET", path: "/app/assets/app-1.js"}); rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %v", rec.Code, rec.Header())
	}
	for _, p := range []string{"/zzzzzz/movies", "/" + v.ID + "x/y", "/nope/deeper"} {
		if rec := e.do(call{method: "GET", path: p}); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	// The API still answers with JSON next to the catch-all.
	if rec := e.do(call{method: "DELETE", path: "/api/v1/nope"}); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not_found") {
		t.Fatalf("api 404: %d %s", rec.Code, rec.Body)
	}
}
