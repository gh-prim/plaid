package playback

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/settings"
)

// Errors of the service, on top of Decide's. The handlers turn them into
// API errors.
var (
	ErrProfileRequired = errors.New("profile_required")
	ErrItemNotFound    = errors.New("no such item")
	ErrItemNotPlayable = errors.New("only movies and episodes play")
	// ErrFFmpegMissing: the file needs HLS, which needs FFmpeg (R31).
	ErrFFmpegMissing = errors.New("ffmpeg_missing")
)

// Playback states clients report (R32).
const (
	StatePlaying   = "playing"
	StatePaused    = "paused"
	StateBuffering = "buffering"
	StateStopped   = "stopped"
)

// Accounts is what playback needs from F003.
type Accounts interface {
	AccessLimit(ctx context.Context, p auth.Principal) (access.Limit, error)
	SessionActive(ctx context.Context, sessionID string) (bool, error)
	SessionOwner(ctx context.Context, p auth.Principal) (accounts.SessionOwner, error)
}

// Options configures a Service.
type Options struct {
	DB       *sql.DB
	Library  *library.Store
	Accounts Accounts
	Settings *settings.Store
	// FFmpeg is nil when it isn't installed: Direct Play still works.
	FFmpeg *FFmpeg
	Log    *slog.Logger
	Now    func() time.Time
}

// Service runs playback sessions (F006). It's also the in-process API
// Plaid Party (F010) will use to start one session per participant (R38).
type Service struct {
	o       Options
	manager *Manager
	watch   *WatchStore
	// hls is set once HLS sessions are served (plan PR B). Until then a
	// file that needs HLS answers ffmpeg_missing.
	hls bool
}

// New returns a service. Call Start to run its reaper.
func New(o Options) *Service {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	s := &Service{o: o, manager: NewManager(), watch: &WatchStore{DB: o.DB, Now: o.Now}}
	s.manager.Now, s.manager.Log = o.Now, o.Log
	if o.Accounts != nil {
		s.manager.Alive = o.Accounts.SessionActive
	}
	s.manager.OnEnd = s.saveFinal
	return s
}

// Start runs the session reaper until ctx is done.
func (s *Service) Start(ctx context.Context) { go s.manager.Run(ctx) }

// Close ends every session.
func (s *Service) Close() { s.manager.CloseAll() }

// FFmpeg returns the FFmpeg found at startup, nil when missing.
func (s *Service) FFmpeg() *FFmpeg { return s.o.FFmpeg }

// Watch returns the watch state store.
func (s *Service) Watch() *WatchStore { return s.watch }

// StartRequest is a playback request with the client's profile.
type StartRequest struct {
	Request
	Profile         Profile
	StartPositionMs int64
}

// StartSession starts playing an item for p (R1–R9).
func (s *Service) StartSession(ctx context.Context, p auth.Principal, client clientinfo.Info, itemID string, r StartRequest) (Session, error) {
	if p.ProfileID == "" {
		return Session{}, ErrProfileRequired
	}
	item, err := s.o.Library.GetItem(ctx, itemID)
	if errors.Is(err, library.ErrNotFound) {
		return Session{}, ErrItemNotFound
	}
	if err != nil {
		return Session{}, err
	}
	limit, err := s.o.Accounts.AccessLimit(ctx, p)
	if err != nil {
		return Session{}, err
	}
	if !limit.Allows(item.AgeRating) {
		return Session{}, ErrItemNotFound // invisible items don't exist (R9)
	}
	if item.Kind != library.KindMovie && item.Kind != library.KindEpisode {
		return Session{}, ErrItemNotPlayable
	}
	files, err := s.o.Library.PlayableFiles(ctx, itemID)
	if err != nil {
		return Session{}, err
	}
	cfg, err := LoadConfig(ctx, s.o.Settings)
	if err != nil {
		return Session{}, err
	}
	lim := Limits{RemoteMaxBitrate: cfg.RemoteMaxBitrate, Remote: IsRemote(client.IP, cfg.LocalNetworks)}
	decide := func(m Media) (Decision, error) { return Decide(m, r.Request, r.Profile, lim) }
	m, err := ChooseFile(files, r.FileID, func(m Media) Decision { d, _ := decide(m); return d })
	if err != nil {
		return Session{}, err
	}
	d, err := decide(m)
	if err != nil {
		return Session{}, err
	}
	if d.Method != DirectPlay && (!s.hls || s.o.FFmpeg == nil) {
		return Session{}, ErrFFmpegMissing
	}
	owner, err := s.o.Accounts.SessionOwner(ctx, p)
	if err != nil {
		return Session{}, err
	}
	start := min(max(r.StartPositionMs, 0), m.DurationMs)
	sess := s.manager.Add(Session{
		UserID: p.UserID, AuthSessionID: p.SessionID, ProfileID: p.ProfileID,
		ItemID: item.ID, ItemTitle: item.Title,
		Owner:    Owner{Username: owner.Username, ProfileName: owner.ProfileName, DeviceName: owner.DeviceName},
		Decision: d, Request: r.Request, Profile: r.Profile,
		ClientIP: client.IP, Remote: lim.Remote, StartPositionMs: start, PositionMs: start,
	})
	s.o.Log.Info("playback started", "playback_id", sess.ID, "user_id", p.UserID, "item_id", item.ID,
		"file_id", m.FileID, "method", d.Method, "reasons", d.Reasons, "quality", d.Quality, "remote", lim.Remote)
	return sess, nil
}

// Session returns a session p may see.
func (s *Service) Session(p auth.Principal, id string) (Session, error) { return s.manager.Get(p, id) }

// Sessions lists every live session (admin, R29).
func (s *Service) Sessions() []Session { return s.manager.List() }

// Progress records a client's position and state (R32), writing watch
// state at most every 10 s per session, and always on stopped.
func (s *Service) Progress(ctx context.Context, p auth.Principal, id string, positionMs int64, state string) error {
	var (
		write, watched, count bool
	)
	sess, err := s.manager.Update(p, id, func(sess *Session) {
		sess.PositionMs, sess.State, sess.reported = positionMs, state, true
		now := s.o.Now()
		if state == StateStopped || now.Sub(sess.lastWrite) >= progressWrites {
			write, sess.lastWrite = true, now
			watched = reachedEnd(positionMs, sess.Decision.Media.DurationMs)
			count = watched && !sess.counted
			sess.counted = sess.counted || watched
		}
	})
	if err != nil || !write {
		return err
	}
	return s.watch.Save(ctx, sess.ProfileID, sess.ItemID, sess.Decision.Media.FileID,
		positionMs, sess.Decision.Media.DurationMs, watched, count)
}

// Stop ends a session p may see, with its last position when given.
func (s *Service) Stop(p auth.Principal, id string, positionMs *int64) error {
	return s.manager.End(p, id, func(sess *Session) {
		if positionMs != nil {
			sess.PositionMs, sess.reported = *positionMs, true
		}
	})
}

// saveFinal writes the last position of an ending session. Sessions that
// never reported one (VLC on a direct link) leave watch state alone (R34).
func (s *Service) saveFinal(sess Session) {
	if !sess.reported {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	watched := reachedEnd(sess.PositionMs, sess.Decision.Media.DurationMs)
	if err := s.watch.Save(ctx, sess.ProfileID, sess.ItemID, sess.Decision.Media.FileID,
		sess.PositionMs, sess.Decision.Media.DurationMs, watched, watched && !sess.counted); err != nil {
		s.o.Log.Warn("saving the last position failed", "playback_id", sess.ID, "err", err)
	}
}

// StreamSession resolves a stream request's token (R25). When checkAuth
// is set (playlists and files, not segments), it also checks the F003
// session is still usable, and ends its playback at once if it isn't.
func (s *Service) StreamSession(ctx context.Context, token string, checkAuth bool) (Session, error) {
	sess, err := s.manager.ByToken(token)
	if err != nil || !checkAuth || s.o.Accounts == nil {
		return sess, err
	}
	ok, err := s.o.Accounts.SessionActive(ctx, sess.AuthSessionID)
	if err != nil {
		return Session{}, err
	}
	if !ok {
		s.manager.EndAuthSession(sess.AuthSessionID)
		return Session{}, ErrSessionNotFound
	}
	return sess, nil
}
