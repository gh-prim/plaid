package metadata

import (
	"context"
	"strconv"
	"sync"
)

// --- Matched items (F007 R27) ------------------------------------------------

// Matched is a movie or show that just matched, for live views such as
// the onboarding's poster wall.
type Matched struct {
	ID       string
	Kind     string // movie or show
	Title    string
	Year     int
	Poster   *Art
	Backdrop *Art
}

// matchedBuffer is how many matched items wait for a slow subscriber; past
// it the oldest is dropped. The wall is a view: counters stay exact.
const matchedBuffer = 64

type matchedHub struct {
	mu   sync.Mutex
	subs map[string]map[chan Matched]struct{}
}

// SubscribeMatched returns a library's items as they match; cancel stops it.
func (s *Service) SubscribeMatched(libraryID string) (<-chan Matched, func()) {
	h := &s.matched
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = map[string]map[chan Matched]struct{}{}
	}
	if h.subs[libraryID] == nil {
		h.subs[libraryID] = map[chan Matched]struct{}{}
	}
	ch := make(chan Matched, matchedBuffer)
	h.subs[libraryID][ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[libraryID], ch)
	}
}

func (h *matchedHub) subscribed(libraryID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[libraryID]) > 0
}

func (h *matchedHub) publish(libraryID string, m Matched) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[libraryID] {
		select {
		case ch <- m:
		default:
			select {
			case <-ch: // drop the oldest
			default:
			}
			ch <- m
		}
	}
}

// publishMatched sends p to the library's subscribers when it ended
// matched. Nothing is read when nobody listens.
func (s *Service) publishMatched(ctx context.Context, p parsed) {
	if (p.Kind != "movie" && p.Kind != "show") || !s.matched.subscribed(p.LibraryID) {
		return
	}
	sums, err := s.store.Summaries(ctx, []string{p.ID})
	if err != nil {
		s.log.Warn("matched item not published", "item", p.ID, "err", err)
		return
	}
	sm, ok := sums[p.ID]
	if !ok || sm.State != "matched" {
		return
	}
	m := Matched{ID: p.ID, Kind: p.Kind, Title: sm.Title, Year: p.Year, Poster: sm.Poster, Backdrop: sm.Backdrop}
	if len(sm.ReleaseDate) >= 4 {
		if y, err := strconv.Atoi(sm.ReleaseDate[:4]); err == nil {
			m.Year = y
		}
	}
	if m.Title == "" {
		m.Title = p.Title
	}
	s.matched.publish(p.LibraryID, m)
}
