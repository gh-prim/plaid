package playback

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// historyLen is a day of samples, one a minute (step 21).
const historyLen = 24 * 60

// Sample is what playback was doing at a minute (step 21 dashboard).
type Sample struct {
	At         time.Time
	Streams    int
	Transcodes int
	UploadBps  int64
}

// history keeps the last day of samples in memory.
type history struct {
	mu      sync.Mutex
	samples []Sample
}

func (h *history) add(s Sample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.samples = append(h.samples, s)
	if len(h.samples) > historyLen {
		h.samples = h.samples[len(h.samples)-historyLen:]
	}
}

// History returns the last day of samples, oldest first.
func (s *Service) History() []Sample {
	s.hist.mu.Lock()
	defer s.hist.mu.Unlock()
	return append([]Sample(nil), s.hist.samples...)
}

// sample records what playback does now.
func (s *Service) sample() {
	transcodes, _, upload := s.Load()
	s.hist.add(Sample{At: s.o.Now().UTC(), Streams: len(s.manager.List()), Transcodes: transcodes, UploadBps: upload})
}

// runHistory samples once a minute until ctx is done.
func (s *Service) runHistory(ctx context.Context) {
	s.sample()
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.sample()
		}
	}
}

// logPlay writes an ended session to play_log. Best effort: a database
// error only loses a line of Activity.
func (s *Service) logPlay(sess Session) {
	if s.o.DB == nil {
		return
	}
	end := s.o.Now().UTC()
	watched := max(sess.PositionMs-sess.StartPositionMs, 0)
	// Never more than the time it lasted, whatever the client reported.
	watched = min(watched, end.Sub(sess.CreatedAt).Milliseconds())
	if watched < 1000 {
		return
	}
	method := sess.Decision.Method
	if method == "" {
		method = DirectPlay
	}
	// Sessions end outside any request: the write gets its own deadline.
	ctx, cancel := context.WithTimeout(context.Background(), finalSaveTimeout)
	defer cancel()
	var display string
	_ = s.o.DB.QueryRowContext(ctx, `SELECT display_name FROM users WHERE id = ?`, sess.UserID).Scan(&display)
	if display == "" {
		display = sess.Owner.Username
	}
	const layout = "2006-01-02T15:04:05.000Z"
	_, err := s.o.DB.ExecContext(ctx, `INSERT INTO play_log (id, user_id, display_name, profile_name, item_id, title, device_name,
		method, remote, started_at, ended_at, watched_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.Must(uuid.NewV7()).String(), sess.UserID, display, sess.Owner.ProfileName, sess.ItemID, sess.ItemTitle,
		sess.Owner.DeviceName, method, sess.Remote, sess.CreatedAt.UTC().Format(layout), end.Format(layout), watched)
	if err != nil {
		s.o.Log.Warn("play not logged", "playback_id", sess.ID, "err", err)
	}
}

// Play is one line of play_log.
type Play struct {
	ItemID, Title, UserID, DisplayName, ProfileName, DeviceName, Method string
	Remote                                                              bool
	StartedAt                                                           time.Time
	WatchedMs                                                           int64
}

// Day is the hours watched one local day.
type Day struct {
	Date  string // YYYY-MM-DD, local
	Hours float64
	Plays int
}

// Activity returns the hours watched per local day over the last days
// (today last) and the last plays, newest first. offset is the browser's
// getTimezoneOffset, in minutes.
func (s *Service) Activity(ctx context.Context, days, offset int) ([]Day, []Play, error) {
	loc := time.FixedZone("", -offset*60)
	now := s.o.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	from := today.AddDate(0, 0, -(days - 1))
	out := make([]Day, days)
	index := map[string]int{}
	for i := range out {
		d := from.AddDate(0, 0, i).Format("2006-01-02")
		out[i] = Day{Date: d}
		index[d] = i
	}
	const layout = "2006-01-02T15:04:05.000Z"
	rows, err := s.o.DB.QueryContext(ctx, `SELECT item_id, title, COALESCE(user_id, ''), display_name, profile_name,
		device_name, method, remote, started_at, watched_ms FROM play_log WHERE started_at >= ? ORDER BY started_at DESC`,
		from.UTC().Format(layout))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var plays []Play
	for rows.Next() {
		var (
			p       Play
			started string
		)
		if err := rows.Scan(&p.ItemID, &p.Title, &p.UserID, &p.DisplayName, &p.ProfileName, &p.DeviceName, &p.Method,
			&p.Remote, &started, &p.WatchedMs); err != nil {
			return nil, nil, err
		}
		p.StartedAt, _ = time.Parse(layout, started)
		if i, ok := index[p.StartedAt.In(loc).Format("2006-01-02")]; ok {
			out[i].Hours += float64(p.WatchedMs) / 3.6e6
			out[i].Plays++
		}
		if len(plays) < 50 {
			plays = append(plays, p)
		}
	}
	return out, plays, rows.Err()
}
