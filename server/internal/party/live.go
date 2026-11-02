package party

import (
	"context"
	"slices"
	"time"
)

// What a live room does beyond the room itself: the transcode limit (R26),
// moving to the next episode (R30) and starting together once screens are
// ready (R29, R30).

// qualityOrder is F006's ladder, best first.
var qualityOrder = []string{"original", "2160p", "1080p", "720p", "480p", "360p"}

// lowest returns the lowest of the qualities, "720p" when none is known.
func lowest(qs []string) string {
	best := -1
	for _, q := range qs {
		if i := slices.Index(qualityOrder, q); i > best {
			best = i
		}
	}
	if best < 0 {
		return "720p"
	}
	return qualityOrder[best]
}

// blocked records that a person's screen can't get a transcode now and
// tells the host (R26). Callers hold l.mu.
func (s *Service) blocked(l *live, me, quality string) error {
	l.blocked[me] = quality
	type who struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Quality string `json:"quality,omitempty"`
	}
	waiting := []who{}
	qs := []string{}
	for _, id := range l.room.order {
		if q, ok := l.blocked[id]; ok {
			waiting = append(waiting, who{ID: id, Name: l.room.people[id].Name, Quality: q})
			qs = append(qs, q)
		}
	}
	shared := lowest(append(qs, "720p"))
	l.room.emit(Target{Host: true}, "party.transcode.limit", map[string]any{
		"waiting": waiting, "sharedQuality": shared, "options": []string{"shared", "order", "cancel"},
	})
	return nil
}

// chooseTranscode applies the host's choice (R26): one shared quality for
// everyone who converts, an order, or nothing. Callers hold l.mu.
func (s *Service) chooseTranscode(l *live, me, choice, first, quality string) error {
	if me != l.room.Host() {
		return ErrNotHost
	}
	waiting := make([]string, 0, len(l.blocked))
	for _, id := range l.room.order {
		if _, ok := l.blocked[id]; ok {
			waiting = append(waiting, id)
		}
	}
	switch choice {
	case "shared":
		if quality == "" {
			qs := []string{"720p"}
			for _, q := range l.blocked {
				qs = append(qs, q)
			}
			quality = lowest(qs)
		}
		// Every screen that converts moves to this quality, so one encode
		// serves them all (R25); the waiting ones start with it.
		l.room.emit(Target{All: true}, "party.transcode.apply", map[string]any{"quality": quality, "waiting": waiting})
	case "order":
		if !slices.Contains(waiting, first) {
			return ErrNoSuchPerson
		}
		l.room.emit(Target{Person: first}, "party.transcode.apply", map[string]any{"retry": true})
		for _, id := range waiting {
			if id != first {
				l.room.emit(Target{Person: id}, "party.transcode.wait", map[string]any{"after": first})
			}
		}
	case "cancel":
		for _, id := range waiting {
			l.room.emit(Target{Person: id}, "party.transcode.cancelled", nil)
		}
	default:
		return ErrBadChoice
	}
	l.blocked = map[string]string{}
	return nil
}

// ErrBadChoice is an unknown transcode choice.
var ErrBadChoice = ErrNotAllowed

// nextCountdown is how long the next episode waits (R30).
const nextCountdown = 10 * time.Second

// startTimeout starts a new item for the ready screens even if some never
// get ready.
const startTimeout = 20 * time.Second

// itemEnded runs when a screen reaches the end credits or the end (R30).
// Callers hold l.mu.
func (s *Service) itemEnded(ctx context.Context, l *live, now time.Time) {
	if !l.nextAt.IsZero() || l.nextItem != "" {
		return
	}
	next, err := s.store.NextEpisode(ctx, l.room.ItemID)
	if err != nil || next == "" {
		return
	}
	l.nextItem, l.nextAt = next, now.Add(nextCountdown)
	ready, total := l.room.Screens()
	it, _ := s.store.Item(ctx, next)
	l.room.emit(Target{All: true}, "party.next", map[string]any{
		"itemId": next, "title": it.Title, "season": it.Season, "episode": it.Episode, "runtimeMs": it.RuntimeMs,
		"startsAtServerMs": l.nextAt.UnixMilli(), "ready": ready, "of": total,
	})
}

// tickNext moves to the next episode when its countdown ends, then starts
// it once every screen loaded it (or after startTimeout). Callers hold l.mu.
func (s *Service) tickNext(ctx context.Context, l *live, now time.Time) {
	if !l.nextAt.IsZero() && !now.Before(l.nextAt) {
		next := l.nextItem
		l.nextAt = time.Time{}
		l.room.ChangeItem(now, next)
		l.party.ItemID = next
		l.startBy = now.Add(startTimeout)
		if err := s.store.Update(context.WithoutCancel(ctx), l.party); err != nil {
			s.o.Log.Error("party: storing the next episode", "party_id", l.party.ID, "err", err)
		}
		s.o.Log.Info("party: next episode", "party_id", l.party.ID, "item_id", next)
	}
	if !l.startBy.IsZero() {
		ready, total := l.room.Screens()
		if (total > 0 && ready == total) || !now.Before(l.startBy) {
			l.startBy = time.Time{}
			l.nextItem = ""
			l.room.Start(now, 0)
		}
	}
}
