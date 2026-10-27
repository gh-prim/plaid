package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/library"
)

// Server-sent events timing (F004-R21, watch-out SSE). Variables so tests
// can shorten them.
var (
	sseProgressEvery = 2 * time.Second
	ssePingEvery     = 15 * time.Second
)

// GetLibraryScanEvents streams the library's scan events.
func (h *Handlers) GetLibraryScanEvents(ctx context.Context, req api.GetLibraryScanEventsRequestObject) (api.GetLibraryScanEventsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	first, err := h.library.Progress(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	events, cancel, err := h.library.Subscribe(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	return scanEventStream{
		ctx: ctx, shutdown: h.streams, events: events, cancel: cancel, first: first,
		progress: func() (library.Progress, error) { return h.library.Progress(ctx, req.LibraryId) },
	}, nil
}

// scanEventStream writes a text/event-stream response. It implements the
// generated response interface so the route keeps the OpenAPI contract
// and the auth middleware.
type scanEventStream struct {
	ctx      context.Context // the request's: done when the client leaves
	shutdown context.Context // done when the server shuts down
	events   <-chan library.Event
	cancel   func()
	first    library.Progress
	progress func() (library.Progress, error)
}

// VisitGetLibraryScanEventsResponse runs until the client disconnects, the
// server shuts down or the library is deleted. Write errors mean the
// client is gone: headers are sent by then, so they end the stream quietly.
func (s scanEventStream) VisitGetLibraryScanEventsResponse(w http.ResponseWriter) error {
	defer s.cancel()
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)

	send := func(ev library.Event) bool {
		data, err := json.Marshal(api.ScanEvent{
			Type: api.ScanEventType(ev.Type), Progress: toScanProgress(ev.Progress), Error: optional(ev.Error),
		})
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send(library.Event{Type: library.EventProgress, Progress: s.first}) {
		return nil
	}

	progress := time.NewTicker(sseProgressEvery)
	defer progress.Stop()
	ping := time.NewTicker(ssePingEvery)
	defer ping.Stop()
	shutdown := s.shutdown
	if shutdown == nil {
		shutdown = context.Background()
	}
	for {
		select {
		case <-s.ctx.Done():
			return nil
		case <-shutdown.Done():
			return nil
		case ev, ok := <-s.events:
			if !ok || !send(ev) {
				return nil
			}
		case <-progress.C:
			// A heartbeat with counters while scanning, even when nothing
			// changed, so clients see the scan is alive.
			p, err := s.progress()
			if err != nil {
				return nil
			}
			if p.State != library.StateIdle && !send(library.Event{Type: library.EventProgress, Progress: p}) {
				return nil
			}
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return nil
			}
		}
	}
}
