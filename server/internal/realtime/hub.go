// Package realtime is PLAID's WebSocket (F010-R44): one connection per
// client at /api/v1/ws, carrying JSON envelopes. The hub authenticates (the
// auth middleware already ran), keeps connections alive, answers clock pings
// and routes every other message to a Handler (Plaid Party). Protocol:
// api/events.md.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/google/uuid"
)

// Envelope is every message, both ways (api/events.md conventions).
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
	// TS is the server's clock in Unix milliseconds on server messages.
	TS int64 `json:"ts,omitempty"`
	// ID is set by a client that wants an ack or an error back for this
	// message; the server echoes it.
	ID string `json:"id,omitempty"`
}

// Error is a refusal sent back as {"type":"error","id":…,"data":{code,message}}.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func (e *Error) Error() string { return e.Code }

// Errors handlers return.
var (
	ErrBadMessage  = &Error{Code: "bad_message"}
	ErrRateLimited = &Error{Code: "rate_limited"}
	ErrNotAllowed  = &Error{Code: "not_allowed"}
)

// Device is what a connection runs on, for presence and routing (F010-R20, R22).
type Device struct {
	Name string `json:"name"`
	// Type is the F003 session's device type (web, ios, ipados, tvos…);
	// "browser" for a guest.
	Type string `json:"type"`
}

// Handler receives what the hub doesn't answer itself.
type Handler interface {
	Connected(c *Conn)
	// Message handles one client message; a returned *Error goes back to
	// the client, any other error is logged and sent as "internal".
	Message(c *Conn, env Envelope) error
	Disconnected(c *Conn)
}

// Options configures a Hub.
type Options struct {
	Handler Handler
	// Describe returns the device behind a connection; nil leaves it empty.
	Describe func(ctx context.Context, p auth.Principal) Device
	// OriginPatterns are other origins allowed to open a socket with the
	// cookie (the app hosted on getplaid.direct); the server's own origin
	// always is.
	OriginPatterns []string
	Log            *slog.Logger
	Now            func() time.Time
	// PingEvery keeps connections alive through proxies (25 s).
	PingEvery time.Duration
	// SlowAfter drops a client whose writes are stuck that long (5 s).
	SlowAfter time.Duration
	// Streams ends every connection when cancelled (server shutdown).
	Streams context.Context
}

// Hub holds every live connection.
type Hub struct {
	opts Options

	mu    sync.RWMutex
	conns map[string]*Conn
}

// sendQueue is how many messages wait for a slow client before it's dropped.
const sendQueue = 256

// maxMessage caps a client message (a chat line is 500 characters).
const maxMessage = 8 << 10

// New returns a hub.
func New(opts Options) *Hub {
	if opts.Log == nil {
		opts.Log = slog.New(slog.DiscardHandler)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.PingEvery == 0 {
		opts.PingEvery = 25 * time.Second
	}
	if opts.SlowAfter == 0 {
		opts.SlowAfter = 5 * time.Second
	}
	if opts.Streams == nil {
		opts.Streams = context.Background()
	}
	return &Hub{opts: opts, conns: map[string]*Conn{}}
}

// Conn is one client connection.
type Conn struct {
	ID        string
	Principal auth.Principal
	Device    Device

	hub    *Hub
	send   chan []byte
	cancel context.CancelFunc
	limit  Limiter

	mu   sync.Mutex
	vals map[string]any
}

// Set and Get keep per-connection state for the handler (the party joined).
func (c *Conn) Set(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vals == nil {
		c.vals = map[string]any{}
	}
	c.vals[key] = v
}

// Get returns what Set stored.
func (c *Conn) Get(key string) any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.vals[key]
}

// Send queues a server message. A client too slow to drain its queue is
// disconnected (it reconnects and gets a fresh snapshot).
func (c *Conn) Send(typ string, data any) {
	b, err := c.hub.encode(typ, "", data)
	if err != nil {
		c.hub.opts.Log.Error("realtime: encode", "type", typ, "err", err)
		return
	}
	select {
	case c.send <- b:
	default:
		c.hub.opts.Log.Info("realtime: dropping a slow client", "conn", c.ID)
		c.Close()
	}
}

// Close ends the connection.
func (c *Conn) Close() { c.cancel() }

func (h *Hub) encode(typ, id string, data any) ([]byte, error) {
	env := Envelope{Type: typ, TS: h.opts.Now().UnixMilli(), ID: id}
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		env.Data = raw
	}
	return json.Marshal(env)
}

// ServeHTTP upgrades GET /api/v1/ws. The auth middleware put the caller in
// the context; the library checks Origin against the Host and the extra
// patterns, so another site can't ride a browser's cookie.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.FromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.opts.OriginPatterns})
	if err != nil {
		return
	}
	ws.SetReadLimit(maxMessage)
	ctx, cancel := context.WithCancel(h.opts.Streams)
	c := &Conn{
		ID: uuid.Must(uuid.NewV7()).String(), Principal: p, hub: h,
		send: make(chan []byte, sendQueue), cancel: cancel,
		limit: Limiter{Rate: 20, Burst: 40},
	}
	if h.opts.Describe != nil {
		c.Device = h.opts.Describe(r.Context(), p)
	}
	h.mu.Lock()
	h.conns[c.ID] = c
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.conns, c.ID)
		h.mu.Unlock()
		if h.opts.Handler != nil {
			h.opts.Handler.Disconnected(c)
		}
		cancel()
		ws.CloseNow()
	}()
	if h.opts.Handler != nil {
		h.opts.Handler.Connected(c)
	}
	go h.writeLoop(ctx, ws, c)
	h.readLoop(ctx, ws, c)
}

func (h *Hub) writeLoop(ctx context.Context, ws *websocket.Conn, c *Conn) {
	ping := time.NewTicker(h.opts.PingEvery)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			ws.Close(websocket.StatusGoingAway, "")
			return
		case b := <-c.send:
			wctx, done := context.WithTimeout(ctx, h.opts.SlowAfter)
			err := ws.Write(wctx, websocket.MessageText, b)
			done()
			if err != nil {
				c.Close()
				return
			}
		case <-ping.C:
			pctx, done := context.WithTimeout(ctx, 2*h.opts.SlowAfter)
			err := ws.Ping(pctx)
			done()
			if err != nil {
				c.Close()
				return
			}
		}
	}
}

func (h *Hub) readLoop(ctx context.Context, ws *websocket.Conn, c *Conn) {
	for {
		typ, b, err := ws.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		var env Envelope
		if json.Unmarshal(b, &env) != nil || env.Type == "" {
			h.reply(c, "", ErrBadMessage)
			continue
		}
		if !c.limit.Allow(h.opts.Now()) {
			h.reply(c, env.ID, ErrRateLimited)
			continue
		}
		h.dispatch(c, env)
	}
}

// pong answers a clock ping (F010-R9).
type pong struct {
	T0 int64 `json:"t0"`
}

func (h *Hub) dispatch(c *Conn, env Envelope) {
	if env.Type == "sync.ping" {
		var in pong
		if json.Unmarshal(env.Data, &in) != nil {
			h.reply(c, env.ID, ErrBadMessage)
			return
		}
		b, _ := h.encode("sync.pong", env.ID, in)
		select {
		case c.send <- b:
		default:
		}
		return
	}
	if h.opts.Handler == nil {
		h.reply(c, env.ID, &Error{Code: "unknown_type"})
		return
	}
	h.reply(c, env.ID, h.opts.Handler.Message(c, env))
}

// reply sends the ack or error for a client message carrying an id; errors
// without an id still go back, so a client sees why it was ignored.
func (h *Hub) reply(c *Conn, id string, err error) {
	if err == nil {
		if id != "" {
			b, _ := h.encode("ack", id, nil)
			c.trySend(b)
		}
		return
	}
	var e *Error
	if !errors.As(err, &e) {
		h.opts.Log.Error("realtime: handler", "conn", c.ID, "err", err)
		e = &Error{Code: "internal"}
	}
	b, _ := h.encode("error", id, e)
	c.trySend(b)
}

func (c *Conn) trySend(b []byte) {
	select {
	case c.send <- b:
	default:
	}
}

// Find returns the live connections match accepts.
func (h *Hub) Find(match func(*Conn) bool) []*Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var out []*Conn
	for _, c := range h.conns {
		if match(c) {
			out = append(out, c)
		}
	}
	return out
}

// SendToUser sends a message to every connection of an account.
func (h *Hub) SendToUser(userID, typ string, data any) {
	for _, c := range h.Find(func(c *Conn) bool { return c.Principal.UserID == userID && userID != "" }) {
		c.Send(typ, data)
	}
}

// Limiter is a token bucket: Rate per second, up to Burst. Not safe for
// concurrent use.
type Limiter struct {
	Rate, Burst float64
	tokens      float64
	last        time.Time
}

// Allow takes a token if there is one.
func (b *Limiter) Allow(now time.Time) bool {
	if b.last.IsZero() {
		b.tokens = b.Burst
	} else {
		b.tokens = min(b.Burst, b.tokens+now.Sub(b.last).Seconds()*b.Rate)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
