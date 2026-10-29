package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
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
	stream := scanEventStream{
		ctx: ctx, shutdown: h.streams, events: events, cancel: cancel, first: first,
		progress: func() (library.Progress, error) { return h.library.Progress(ctx, req.LibraryId) },
	}
	// F005 R35: the library's matching counters ride on the same stream.
	// F007 R27: so do the items as they match, for live views.
	if h.metadata != nil {
		counts, stop := h.metadata.Subscribe(req.LibraryId)
		matched, stopMatched := h.metadata.SubscribeMatched(req.LibraryId)
		stream.counts, stream.matched = counts, matched
		stream.cancel = func() { cancel(); stop(); stopMatched() }
	}
	return stream, nil
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
	counts   <-chan metadata.Counters // nil without F005
	matched  <-chan metadata.Matched  // nil without F005
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

	write := func(ev api.ScanEvent) bool {
		data, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	send := func(ev library.Event) bool {
		p := toScanProgress(ev.Progress)
		return write(api.ScanEvent{Type: api.ScanEventType(ev.Type), Progress: &p, Error: optional(ev.Error)})
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
		case c := <-s.counts:
			if !write(api.ScanEvent{Type: api.ScanEventTypeMetadata, Metadata: &api.MetadataCounts{
				Pending: c.Pending, Matched: c.Matched, NeedsReview: c.NeedsReview, Unmatched: c.Unmatched,
			}}) {
				return nil
			}
		case m := <-s.matched:
			if !write(api.ScanEvent{Type: api.ScanEventTypeMatched, Item: toMatchedItem(m)}) {
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

func toMatchedItem(m metadata.Matched) *api.MatchedItem {
	it := &api.MatchedItem{Id: m.ID, Kind: api.MatchedItemKind(m.Kind), Title: m.Title,
		Poster: toArtwork(m.Poster), Backdrop: toArtwork(m.Backdrop)}
	if m.Year > 0 {
		it.Year = &m.Year
	}
	return it
}
