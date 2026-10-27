package metadata

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdb"
	"github.com/gh-prim/plaid/server/internal/settings"
)

// TMDB request rate (R32).
const tmdbRPS = 20

// sweepEvery is how often items without a metadata row are looked for.
const sweepEvery = time.Hour

// Options configure the service.
type Options struct {
	DB       *sql.DB
	Settings *settings.Store
	// TMDBKey is PLAID_TMDB_API_KEY.
	TMDBKey string
	Log     *slog.Logger
	Workers int
	// Provider replaces TMDB in tests.
	Provider provider.Provider
	// Now is the clock; tests replace it.
	Now func() time.Time
	// QueueIdle is how long idle workers wait; tests shorten it.
	QueueIdle time.Duration
}

// Service matches items and keeps their metadata up to date (F005).
type Service struct {
	o     Options
	store *Store
	queue *Queue
	log   *slog.Logger

	mu        sync.Mutex
	prov      provider.Provider
	http      *httpx.Client
	keySource string

	stop context.CancelFunc
	wg   sync.WaitGroup
}

// New builds the service; Start runs it.
func New(o Options) *Service {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	st := NewStore(o.DB)
	st.Now = o.Now
	s := &Service{o: o, store: st, log: o.Log}
	s.queue = &Queue{Store: st, Process: s.Process, Workers: o.Workers, Idle: o.QueueIdle, Log: o.Log}
	return s
}

// Store gives the handlers read access.
func (s *Service) Store() *Store { return s.store }

// Start resolves the TMDB key, sweeps for items without metadata and runs
// the queue, in the background: startup never waits on the network (R1).
func (s *Service) Start(ctx context.Context) {
	ctx, s.stop = context.WithCancel(ctx)
	if err := s.ReloadKeys(ctx); err != nil {
		s.log.Error("metadata keys not loaded", "err", err)
	}
	s.queue.Start(ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		t := time.NewTicker(sweepEvery)
		defer t.Stop()
		for {
			if n, err := s.store.Sweep(ctx); err != nil && ctx.Err() == nil {
				s.log.Error("metadata sweep failed", "err", err)
			} else if n > 0 {
				s.log.Info("items queued for metadata", "count", n)
				s.queue.Wake()
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// Close stops the queue and the sweeper.
func (s *Service) Close() {
	if s.stop != nil {
		s.stop()
	}
	s.queue.Close()
	s.wg.Wait()
}

// Wake tells the queue there may be work now.
func (s *Service) Wake() { s.queue.Wake() }

// ReloadKeys picks the TMDB key again (ADR 0007 order) and rebuilds the
// client. With no key, metadata stays local-only (R34).
func (s *Service) ReloadKeys(ctx context.Context) error {
	if s.o.Provider != nil {
		s.mu.Lock()
		s.prov, s.keySource = s.o.Provider, KeyCustom
		s.mu.Unlock()
		return nil
	}
	key, source, err := resolveKey(ctx, s.o.Settings, settings.TMDBAPIKey, s.o.TMDBKey, builtinTMDBKey)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keySource = source
	if key == "" {
		s.prov, s.http = nil, nil
		s.log.Warn("no TMDB API key: metadata comes from .nfo files only; set PLAID_TMDB_API_KEY", "code", "no_api_key")
		return nil
	}
	s.http = httpx.New(httpx.Options{Name: "tmdb", Hosts: tmdb.Hosts, RPS: tmdbRPS, Burst: tmdbRPS, Log: s.log})
	s.prov = tmdb.New(s.http, tmdb.BaseURL, key)
	return nil
}

func (s *Service) provider() provider.Provider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prov
}

// ItemsCreated is library.Hooks.ItemsCreated: new items get a pending row,
// and a matched show with new seasons or episodes is refreshed now so they
// get their metadata (R1, R31).
func (s *Service) ItemsCreated(_ string, itemIDs []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.store.AddPending(ctx, itemIDs); err != nil {
		s.log.Error("metadata: queueing new items failed", "err", err)
		return
	}
	if err := s.store.refreshShowsOf(ctx, itemIDs); err != nil {
		s.log.Error("metadata: refreshing shows of new episodes failed", "err", err)
	}
	s.queue.Wake()
}

// Process does one job: match (or re-match) a movie or a show and store
// its metadata. Provider errors leave stored data untouched (R33).
func (s *Service) Process(ctx context.Context, j Job) error {
	p, err := s.store.loadParsed(ctx, j.ItemID)
	if errors.Is(err, ErrNotFound) {
		return nil // deleted meanwhile
	}
	if err != nil {
		return err
	}
	var perr error
	switch p.Kind {
	case "movie":
		perr = s.processMovie(ctx, p)
	case "show":
		perr = s.processShow(ctx, p)
	default:
		return nil
	}
	if errors.Is(perr, httpx.ErrInvalidKey) {
		return errors.Join(ErrPaused, perr)
	}
	return perr
}
