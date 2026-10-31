// Package segments finds the intro, recap, credits and preview of movies
// and episodes, for skip buttons and the next-episode card: from
// TheIntroDB's community timestamps, and for an episode's end credits from
// the file itself (detect.go), which wins over TheIntroDB. Answers are kept
// in item_segments: TheIntroDB is asked again after a month, or after a few
// days when it knew nothing; a file is looked at once.
package segments

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
)

// Hosts TheIntroDB requests may go to.
var Hosts = []string{"api.theintrodb.org"}

// BaseURL is TheIntroDB's API root.
const BaseURL = "https://api.theintrodb.org/v4"

// Kinds of segment, as TheIntroDB names them.
const (
	Intro   = "intro"
	Recap   = "recap"
	Credits = "credits"
	Preview = "preview"
)

const (
	// keepFound and keepNone are how long an answer is used before asking again.
	keepFound = 30 * 24 * time.Hour
	keepNone  = 3 * 24 * time.Hour
	// Without a key TheIntroDB allows 30 requests per 10 s and 5,000 a day.
	rps = 2
)

// Segment is one part of a title; a nil bound is the start or the end.
type Segment struct {
	Kind    string `json:"kind"`
	StartMs *int64 `json:"startMs,omitempty"`
	EndMs   *int64 `json:"endMs,omitempty"`
}

// Service looks segments up and keeps them.
type Service struct {
	db   *sql.DB
	http *httpx.Client
	base string
	log  *slog.Logger
	now  func() time.Time
	// ffmpeg finds credits in files; "" turns that off.
	ffmpeg string
	// detecting runs one detection at a time; queued holds the items
	// waiting or running, so each is looked at once.
	detecting chan struct{}
	mu        sync.Mutex
	queued    map[string]bool
	// background is where detections run; tests wait on it.
	background sync.WaitGroup
}

// Options configure a Service.
type Options struct {
	DB  *sql.DB
	Log *slog.Logger
	// Base and HTTP are for tests; BaseURL and a rate-limited client otherwise.
	Base string
	HTTP *httpx.Client
	Now  func() time.Time
	// FFmpeg is the ffmpeg binary that finds end credits; "" turns it off.
	FFmpeg string
}

// New returns a Service.
func New(o Options) *Service {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Base == "" {
		o.Base = BaseURL
	}
	if o.HTTP == nil {
		o.HTTP = httpx.New(httpx.Options{Name: "theintrodb", Hosts: Hosts, RPS: rps, Burst: 2, Timeout: 5 * time.Second, Log: o.Log})
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{db: o.DB, http: o.HTTP, base: strings.TrimSuffix(o.Base, "/"), log: o.Log, now: o.Now,
		ffmpeg: o.FFmpeg, detecting: make(chan struct{}, 1), queued: map[string]bool{}}
}

// For returns the segments of a movie or an episode, in time order; none
// for other items, unmatched ones, or when nothing is known. A TheIntroDB
// lookup that fails is not kept: the next call asks again. An episode
// whose file wasn't looked at yet is queued; its credits come with a later
// call.
func (s *Service) For(ctx context.Context, itemID string) ([]Segment, error) {
	t, ok, err := s.target(ctx, itemID)
	if err != nil || !ok {
		return nil, err
	}
	var (
		raw, fetched sql.NullString
		creditsMs    sql.NullInt64
		detectedAt   sql.NullString
	)
	err = s.db.QueryRowContext(ctx, `SELECT segments, fetched_at, credits_ms, detected_at FROM item_segments WHERE item_id = ?`,
		itemID).Scan(&raw, &fetched, &creditsMs, &detectedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var segs []Segment
	fresh := false
	if raw.Valid && json.Unmarshal([]byte(raw.String), &segs) == nil {
		at, _ := time.Parse(time.RFC3339Nano, fetched.String)
		keep := keepNone
		if len(segs) > 0 {
			keep = keepFound
		}
		fresh = s.now().Sub(at) < keep
	}
	if !fresh && t.query != nil {
		found, err := s.fetch(ctx, t)
		if err != nil {
			s.log.Warn("theintrodb: lookup failed", "item", itemID, "err", err)
		} else {
			segs = found
			body, _ := json.Marshal(segs)
			if _, err := s.db.ExecContext(ctx, `INSERT INTO item_segments (item_id, segments, fetched_at) VALUES (?, ?, ?)
				ON CONFLICT (item_id) DO UPDATE SET segments = excluded.segments, fetched_at = excluded.fetched_at`,
				itemID, string(body), s.now().UTC().Format(time.RFC3339Nano)); err != nil {
				return nil, err
			}
		}
	}
	if t.kind == "episode" && !detectedAt.Valid {
		s.queue(itemID, t)
	}
	if creditsMs.Valid {
		// Found in the file: these credits win.
		segs = slices.DeleteFunc(slices.Clone(segs), func(x Segment) bool { return x.Kind == Credits })
		at := creditsMs.Int64
		segs = append(segs, Segment{Kind: Credits, StartMs: &at})
		sort.SliceStable(segs, func(i, j int) bool { return start(segs[i]) < start(segs[j]) })
	}
	if segs == nil {
		segs = []Segment{}
	}
	return segs, nil
}

// target is what is known of an item to look it up: how to ask
// TheIntroDB (nil when it can't be asked), and its longest file.
type target struct {
	kind       string
	query      url.Values
	path       string
	durationMs int64
}

// target reads an item: a movie by its TMDB id, an episode by its show's
// with its season and number, and the length of its longest file to pick
// the right cut. ok is false for anything but a movie or an episode.
func (s *Service) target(ctx context.Context, itemID string) (target, bool, error) {
	var (
		t               target
		tmdb            sql.NullInt64
		imdb            sql.NullString
		season, episode sql.NullInt64
		path            sql.NullString
		duration        sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx, `SELECT i.kind, m.tmdb_id, m.imdb_id, p.season_number, i.episode_number, f.path, f.duration_ms
		FROM items i
		LEFT JOIN item_metadata m ON m.item_id = i.id
		LEFT JOIN items p ON p.id = i.parent_id
		LEFT JOIN media_files f ON f.id = (SELECT x.file_id FROM item_files x JOIN media_files y ON y.id = x.file_id
			WHERE x.item_id = i.id AND y.missing_since IS NULL ORDER BY y.duration_ms DESC LIMIT 1)
		WHERE i.id = ?`, itemID).Scan(&t.kind, &tmdb, &imdb, &season, &episode, &path, &duration)
	if errors.Is(err, sql.ErrNoRows) {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	if t.kind != "movie" && t.kind != "episode" {
		return t, false, nil
	}
	t.path, t.durationMs = path.String, duration.Int64
	q := url.Values{"details": {"true"}}
	switch {
	case tmdb.Valid && tmdb.Int64 > 0:
		q.Set("tmdb_id", strconv.FormatInt(tmdb.Int64, 10))
	case t.kind == "movie" && imdb.Valid && imdb.String != "":
		q.Set("imdb_id", imdb.String)
	default:
		q = nil
	}
	if q != nil && t.kind == "episode" {
		if season.Valid && episode.Valid {
			q.Set("season", strconv.FormatInt(season.Int64, 10))
			q.Set("episode", strconv.FormatInt(episode.Int64, 10))
		} else {
			q = nil
		}
	}
	if q != nil && t.durationMs > 0 {
		q.Set("duration_ms", strconv.FormatInt(t.durationMs, 10))
	}
	t.query = q
	return t, true, nil
}

type bound struct {
	StartMs     *int64  `json:"start_ms"`
	EndMs       *int64  `json:"end_ms"`
	Confidence  float64 `json:"confidence"`
	Submissions int     `json:"submission_count"`
}

const (
	// Credits from TheIntroDB are believed with this many submissions, or
	// this confidence, and when they last at least minTheirCredits.
	trustedSubmissions = 2
	trustedConfidence  = 0.5
	minTheirCredits    = 45_000
)

// fetch asks TheIntroDB. A 404 (nothing submitted, unknown title) is no
// segments, which is kept like any answer. End credits from a single
// unsure submission, or too short to be credits, are left out; credits
// measured on a cut of another length move with the end of this one.
func (s *Service) fetch(ctx context.Context, t target) ([]Segment, error) {
	body, err := s.http.Get(ctx, s.base+"/media?"+t.query.Encode(), nil)
	if errors.Is(err, httpx.ErrNotFound) {
		return []Segment{}, nil
	}
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var group int64
	if v, ok := raw["group_duration_ms"]; ok {
		_ = json.Unmarshal(v, &group)
	}
	length := cmp.Or(t.durationMs, group)
	segs := []Segment{}
	for _, kind := range []string{Intro, Recap, Credits, Preview} {
		var bounds []bound
		if v, ok := raw[kind]; !ok || json.Unmarshal(v, &bounds) != nil {
			continue
		}
		for _, b := range bounds {
			if b.StartMs == nil && b.EndMs == nil {
				continue
			}
			if b.StartMs != nil && b.EndMs != nil && *b.EndMs <= *b.StartMs {
				continue
			}
			seg := Segment{Kind: kind, StartMs: b.StartMs, EndMs: b.EndMs}
			if kind == Credits && b.StartMs != nil && b.EndMs == nil {
				if b.Submissions < trustedSubmissions && b.Confidence < trustedConfidence {
					continue
				}
				at := *b.StartMs
				if group > 0 && t.durationMs > 0 {
					at += t.durationMs - group
				}
				if length > 0 && length-at < minTheirCredits {
					continue
				}
				seg.StartMs = &at
			}
			segs = append(segs, seg)
		}
	}
	sort.SliceStable(segs, func(i, j int) bool { return start(segs[i]) < start(segs[j]) })
	return segs, nil
}

// queue looks for an episode's credits in its file, in the background,
// one file at a time.
func (s *Service) queue(itemID string, t target) {
	if s.ffmpeg == "" || t.path == "" || t.durationMs <= 0 {
		return
	}
	s.mu.Lock()
	if s.queued[itemID] {
		s.mu.Unlock()
		return
	}
	s.queued[itemID] = true
	s.mu.Unlock()
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		defer func() {
			s.mu.Lock()
			delete(s.queued, itemID)
			s.mu.Unlock()
		}()
		s.detecting <- struct{}{}
		defer func() { <-s.detecting }()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		began := s.now()
		at, ok, err := detectCredits(ctx, s.ffmpeg, t.path, t.durationMs)
		if err != nil {
			// Not kept: tried again on the next play.
			s.log.Warn("segments: credits detection failed", "item", itemID, "err", err)
			return
		}
		var credits any
		if ok {
			credits = at
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO item_segments (item_id, segments, fetched_at, credits_ms, detected_at)
			VALUES (?, '[]', '', ?, ?)
			ON CONFLICT (item_id) DO UPDATE SET credits_ms = excluded.credits_ms, detected_at = excluded.detected_at`,
			itemID, credits, s.now().UTC().Format(time.RFC3339Nano)); err != nil {
			s.log.Warn("segments: keeping detected credits", "item", itemID, "err", err)
			return
		}
		s.log.Info("segments: end credits looked for", "item", itemID, "found", ok, "start_ms", at, "took", s.now().Sub(began).Round(time.Second))
	}()
}

func start(s Segment) int64 {
	if s.StartMs == nil {
		return 0
	}
	return *s.StartMs
}
