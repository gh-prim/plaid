package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gh-prim/plaid/server/internal/auth"
)

// recorder is a Handler that echoes "echo" messages and refuses "nope".
type recorder struct {
	mu        sync.Mutex
	connected []*Conn
	gone      int
}

func (r *recorder) Connected(c *Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connected = append(r.connected, c)
}

func (r *recorder) Message(c *Conn, env Envelope) error {
	switch env.Type {
	case "echo":
		c.Send("echoed", env.Data)
		return nil
	case "nope":
		return ErrNotAllowed
	}
	return &Error{Code: "unknown_type"}
}

func (r *recorder) Disconnected(*Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gone++
}

// server serves the hub behind a fake auth middleware: "Bearer alex" is
// user alex, anything else is refused.
func server(t *testing.T, h *Hub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer alex" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: "alex", SessionID: "s1"})
		h.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func dial(t *testing.T, srv *httptest.Server, token string) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/ws",
		&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + token}}})
	if err == nil {
		t.Cleanup(func() { ws.CloseNow() })
	}
	return ws, err
}

func roundTrip(t *testing.T, ws *websocket.Conn, out string) Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out != "" {
		if err := ws.Write(ctx, websocket.MessageText, []byte(out)); err != nil {
			t.Fatal(err)
		}
	}
	_, b, err := ws.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestHub(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	rec := &recorder{}
	h := New(Options{Handler: rec, Now: func() time.Time { return now },
		Describe: func(context.Context, auth.Principal) Device { return Device{Name: "Salon", Type: "tvos"} }})
	srv := server(t, h)

	if _, err := dial(t, srv, "mia"); err == nil {
		t.Fatal("unauthenticated socket accepted")
	}
	ws, err := dial(t, srv, "alex")
	if err != nil {
		t.Fatal(err)
	}

	// Clock ping: the server echoes t0 with its time (R9).
	env := roundTrip(t, ws, `{"type":"sync.ping","id":"1","data":{"t0":42}}`)
	if env.Type != "sync.pong" || env.ID != "1" || env.TS != now.UnixMilli() || string(env.Data) != `{"t0":42}` {
		t.Errorf("pong = %+v %s", env, env.Data)
	}
	// Routed messages, then their ack.
	env = roundTrip(t, ws, `{"type":"echo","id":"2","data":{"x":1}}`)
	if env.Type != "echoed" || string(env.Data) != `{"x":1}` {
		t.Errorf("echo = %+v", env)
	}
	if env = roundTrip(t, ws, ""); env.Type != "ack" || env.ID != "2" {
		t.Errorf("ack = %+v", env)
	}
	// Refusals carry their code.
	env = roundTrip(t, ws, `{"type":"nope","id":"3"}`)
	var e Error
	json.Unmarshal(env.Data, &e)
	if env.Type != "error" || env.ID != "3" || e.Code != "not_allowed" {
		t.Errorf("error = %+v %s", env, env.Data)
	}
	env = roundTrip(t, ws, `not json`)
	json.Unmarshal(env.Data, &e)
	if env.Type != "error" || e.Code != "bad_message" {
		t.Errorf("bad message = %+v", env)
	}

	conns := h.Find(func(c *Conn) bool { return c.Principal.UserID == "alex" })
	if len(conns) != 1 || conns[0].Device.Type != "tvos" {
		t.Fatalf("Find = %+v", conns)
	}
	h.SendToUser("alex", "notify", map[string]string{"kind": "invite"})
	if env = roundTrip(t, ws, ""); env.Type != "notify" {
		t.Errorf("SendToUser = %+v", env)
	}

	ws.Close(websocket.StatusNormalClosure, "")
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec.mu.Lock()
		gone := rec.gone
		rec.mu.Unlock()
		if gone == 1 && len(h.Find(func(*Conn) bool { return true })) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("connection not cleaned up")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestHubRefusesOtherOrigins(t *testing.T) {
	srv := server(t, New(Options{}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer alex"}, "Origin": {"https://evil.example"}}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin socket: %v %v", resp, err)
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := Limiter{Rate: 1, Burst: 5}
	for i := 0; i < 5; i++ {
		if !l.Allow(now) {
			t.Fatalf("burst refused at %d", i)
		}
	}
	if l.Allow(now) {
		t.Fatal("over burst allowed")
	}
	if !l.Allow(now.Add(time.Second)) || l.Allow(now.Add(time.Second)) {
		t.Fatal("refill wrong")
	}
}
