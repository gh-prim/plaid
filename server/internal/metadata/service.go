package metadata

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/metadata/artwork"
	"github.com/gh-prim/plaid/server/internal/metadata/fanart"
	"github.com/gh-prim/plaid/server/internal/metadata/httpx"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
	"github.com/gh-prim/plaid/server/internal/metadata/tmdb"
	"github.com/gh-prim/plaid/server/internal/settings"
)

// Request rates and concurrency (R32).
const (
	tmdbRPS        = 20
	fanartRPS      = 5
	imageDownloads = 8
)

// cleanupEvery is how often unused originals are deleted (R18).
const cleanupEvery = 24 * time.Hour

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
	// DataDir holds artwork (PLAID_DATA_DIR); empty disables artwork.
	DataDir string
	// FanartKey is PLAID_FANART_API_KEY.
	FanartKey string
	// ArtworkCacheBytes caps resized variants (R20); 0 means 2 GiB.
	ArtworkCacheBytes int64
	// Fanart and ImageHTTP replace fanart.tv and image downloads in tests,
	// with ImageHosts pointing the image hosts at a local server.
	Fanart     *fanart.Client
	ImageHTTP  *httpx.Client
	ImageHosts map[string]string
	// CheckKey validates a key ("tmdb" or "fanart") before it's saved;
	// nil asks the provider. Tests replace it.
	CheckKey func(ctx context.Context, provider, key string) error
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
	fan       *fanart.Client
	fanHTTP   *httpx.Client
	art       *artwork.Store
	hub       counterHub

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
	if o.DataDir != "" {
		h := o.ImageHTTP
		if h == nil {
			h = httpx.New(httpx.Options{Name: "images", Hosts: append(slices.Clone(tmdb.Hosts), fanart.Hosts...),
				MaxConcurrent: imageDownloads, Log: o.Log})
		}
		s.art = artwork.New(artwork.Options{DataDir: o.DataDir, HTTP: h, Hosts: o.ImageHosts,
			MaxVariantBytes: o.ArtworkCacheBytes, Log: o.Log})
	}
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
	if s.art != nil {
		s.wg.Add(1)
		go s.cleanupLoop(ctx)
	}
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
		s.prov, s.keySource, s.fan = s.o.Provider, KeyCustom, s.o.Fanart
		s.mu.Unlock()
		return nil
	}
	fanKey, _, err := resolveKey(ctx, s.o.Settings, settings.FanartAPIKey, s.o.FanartKey, builtinFanartKey)
	if err != nil {
		return err
	}
	personal := ""
	if s.o.Settings != nil {
		personal, _, _ = s.o.Settings.Get(ctx, settings.FanartPersonalKey)
	}
	s.mu.Lock()
	s.fan, s.fanHTTP = nil, nil
	if fanKey != "" {
		s.fanHTTP = httpx.New(httpx.Options{Name: "fanart", Hosts: fanart.Hosts, RPS: fanartRPS, Burst: 1, Log: s.log})
		s.fan = fanart.New(s.fanHTTP, fanart.BaseURL, fanKey, personal)
	}
	s.mu.Unlock()
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
	// Burst 1: a smooth 20 requests/s, never more in any second (R32).
	s.http = httpx.New(httpx.Options{Name: "tmdb", Hosts: tmdb.Hosts, RPS: tmdbRPS, Burst: 1, Log: s.log})
	s.prov = tmdb.New(s.http, tmdb.BaseURL, key)
	return nil
}

func (s *Service) cleanupLoop(ctx context.Context) {
	defer s.wg.Done()
	t := time.NewTicker(cleanupEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		n, err := s.art.Cleanup(ctx, func(sha string) (bool, error) {
			var used int
			err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artwork WHERE sha256 = ? AND source <> 'local'`, sha).Scan(&used)
			return used > 0, err
		})
		if err != nil && ctx.Err() == nil {
			s.log.Error("artwork cleanup failed", "err", err)
		} else if n > 0 {
			s.log.Info("unused artwork removed", "count", n)
		}
	}
}

func (s *Service) fanart() *fanart.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fan
}

// Artwork is the image store; nil without a data directory.
func (s *Service) Artwork() *artwork.Store { return s.art }

func (s *Service) provider() provider.Provider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prov
}

// ItemsCreated is library.Hooks.ItemsCreated: new items get a pending row,
// and a matched show with new seasons or episodes is refreshed now so they
// get their metadata (R1, R31).
func (s *Service) ItemsCreated(libraryID string, itemIDs []string) {
	defer s.changed(libraryID)
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
	s.changed(p.LibraryID)
	if errors.Is(perr, httpx.ErrInvalidKey) {
		return errors.Join(ErrPaused, perr)
	}
	return perr
}
