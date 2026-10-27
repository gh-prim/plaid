package playback

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/playback/keyframes"
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
	// TranscodeDir holds HLS segments (R27); CacheDir holds converted
	// subtitles.
	TranscodeDir string
	CacheDir     string
	// Free reports a file system's free bytes; nil uses statfs.
	Free    func(dir string) (uint64, error)
	Timings *Timings // nil: DefaultTimings
	Log     *slog.Logger
	Now     func() time.Time
}

// subtitleCacheMax caps the converted subtitles (R27).
const subtitleCacheMax = 1 << 30

// Service runs playback sessions (F006). It's also the in-process API
// Plaid Party (F010) will use to start one session per participant (R38).
type Service struct {
	o         Options
	t         Timings
	manager   *Manager
	watch     *WatchStore
	keyframes *keyframes.Store
	storage   *Storage
	subtitles *fileCache

	cfgMu sync.Mutex
	cfg   Config

	mu      sync.Mutex
	streams map[string]*hlsStream // by session id
	slots   int                   // transcode slots in use (R26)
	full    bool                  // the transcode folder is over its cap (R27)
}

// New returns a service. Call Start before serving HLS.
func New(o Options) *Service {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	t := DefaultTimings
	if o.Timings != nil {
		t = *o.Timings
	}
	s := &Service{
		o: o, t: t, manager: NewManager(), watch: &WatchStore{DB: o.DB, Now: o.Now},
		keyframes: &keyframes.Store{DB: o.DB, Now: o.Now},
		storage:   &Storage{Dir: o.TranscodeDir, Free: o.Free},
		subtitles: &fileCache{Dir: filepath.Join(o.CacheDir, "subtitles"), Max: subtitleCacheMax},
		cfg:       Config{EncoderPreset: PresetFast},
		streams:   map[string]*hlsStream{},
	}
	s.manager.Now, s.manager.Log = o.Now, o.Log
	if o.Accounts != nil {
		s.manager.Alive = o.Accounts.SessionActive
	}
	s.manager.OnEnd = s.onEnd
	return s
}

// Start empties the transcode folder, reads the settings, and runs the
// session reaper and the stream housekeeping until ctx is done. Without a
// usable transcode folder, only Direct Play is served.
func (s *Service) Start(ctx context.Context) {
	if err := s.ReloadConfig(ctx); err != nil {
		s.o.Log.Error("reading playback settings failed; using defaults", "err", err)
	}
	if s.o.TranscodeDir != "" {
		purged, err := s.storage.Open()
		if err != nil {
			s.o.Log.Error("transcode folder unusable: only Direct Play is available", "dir", s.o.TranscodeDir, "err", err)
			s.o.TranscodeDir = ""
		} else {
			s.o.Log.Info("transcode folder ready", "dir", s.o.TranscodeDir, "purged", purged,
				"default_cap", formatBytes(s.storage.DefaultCap()), "max_transcodes", s.maxTranscodes())
		}
	}
	go s.manager.Run(ctx)
	go func() {
		tick := time.NewTicker(s.t.Tick)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s.housekeep()
			}
		}
	}()
}

// Close ends every session and stops every FFmpeg run.
func (s *Service) Close() { s.manager.CloseAll() }

// FFmpeg returns the FFmpeg found at startup, nil when missing.
func (s *Service) FFmpeg() *FFmpeg { return s.o.FFmpeg }

// Watch returns the watch state store.
func (s *Service) Watch() *WatchStore { return s.watch }

// hlsAvailable reports whether HLS sessions can be served.
func (s *Service) hlsAvailable() bool { return s.o.FFmpeg != nil && s.o.TranscodeDir != "" }

func (s *Service) config() Config {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	return s.cfg
}

// ReloadConfig reads the playback settings again (after a change).
func (s *Service) ReloadConfig(ctx context.Context) error {
	c, err := LoadConfig(ctx, s.o.Settings)
	if err != nil {
		return err
	}
	s.cfgMu.Lock()
	s.cfg = c
	s.cfgMu.Unlock()
	return nil
}

// SettingsView is the settings in effect with their defaults (R30).
type SettingsView struct {
	Config
	MaxTranscodesEffective        int
	MaxTranscodesDefault          int
	TranscodeDirMaxBytesEffective int64
	TranscodeDirMaxBytesDefault   int64
}

// Settings returns the settings in effect.
func (s *Service) Settings() SettingsView {
	c := s.config()
	v := SettingsView{Config: c, MaxTranscodesDefault: DefaultMaxTranscodes(cpuCount(), false),
		TranscodeDirMaxBytesDefault: s.storage.DefaultCap()}
	v.MaxTranscodesEffective, v.TranscodeDirMaxBytesEffective = s.maxTranscodes(), c.TranscodeDirMaxBytes
	if v.TranscodeDirMaxBytesEffective == 0 {
		v.TranscodeDirMaxBytesEffective = v.TranscodeDirMaxBytesDefault
	}
	return v
}

// SaveSettings validates and stores a change, then applies it.
func (s *Service) SaveSettings(ctx context.Context, u ConfigUpdate) (SettingsView, error) {
	if err := SaveConfig(ctx, s.o.Settings, u); err != nil {
		return SettingsView{}, err
	}
	if err := s.ReloadConfig(ctx); err != nil {
		return SettingsView{}, err
	}
	s.o.Log.Info("playback settings changed")
	return s.Settings(), nil
}

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
	item, err := s.visibleItem(ctx, p, itemID)
	if err != nil {
		return Session{}, err
	}
	if item.Kind != library.KindMovie && item.Kind != library.KindEpisode {
		return Session{}, ErrItemNotPlayable
	}
	files, err := s.o.Library.PlayableFiles(ctx, itemID)
	if err != nil {
		return Session{}, err
	}
	r.Request = s.supported(r.Request)
	lim, err := s.limits(ctx, client)
	if err != nil {
		return Session{}, err
	}
	m, err := ChooseFile(files, r.FileID, func(m Media) Decision { d, _ := Decide(m, r.Request, r.Profile, lim); return d })
	if err != nil {
		return Session{}, err
	}
	d, err := s.decide(ctx, &m, r.Request, r.Profile, lim)
	if err != nil {
		return Session{}, err
	}
	slot, err := s.admit(d)
	if err != nil {
		return Session{}, err
	}
	owner, err := s.o.Accounts.SessionOwner(ctx, p)
	if err != nil {
		if slot {
			s.releaseSlot()
		}
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
	if d.Method != DirectPlay {
		s.newStream(sess, 0, slot)
	}
	s.o.Log.Info("playback started", "playback_id", sess.ID, "user_id", p.UserID, "item_id", item.ID,
		"file_id", m.FileID, "method", d.Method, "reasons", d.Reasons, "quality", d.Quality, "remote", lim.Remote)
	return sess, nil
}

// visibleItem returns an item p's profile may see (R9).
func (s *Service) visibleItem(ctx context.Context, p auth.Principal, itemID string) (library.Item, error) {
	item, err := s.o.Library.GetItem(ctx, itemID)
	if errors.Is(err, library.ErrNotFound) {
		return library.Item{}, ErrItemNotFound
	}
	if err != nil {
		return library.Item{}, err
	}
	limit, err := s.o.Accounts.AccessLimit(ctx, p)
	if err != nil {
		return library.Item{}, err
	}
	if !limit.Allows(item.AgeRating) {
		return library.Item{}, ErrItemNotFound // invisible items don't exist (R9)
	}
	return item, nil
}

// supported drops what this FFmpeg can't do: burning text subtitles needs
// the subtitles filter (libass), which some builds lack. The track is then
// delivered separately.
func (s *Service) supported(r Request) Request {
	if r.BurnSubtitles && (s.o.FFmpeg == nil || !s.o.FFmpeg.HasFilter("subtitles")) {
		r.BurnSubtitles = false
	}
	return r
}

func (s *Service) limits(ctx context.Context, client clientinfo.Info) (Limits, error) {
	cfg, err := LoadConfig(ctx, s.o.Settings)
	if err != nil {
		return Limits{}, err
	}
	return Limits{RemoteMaxBitrate: cfg.RemoteMaxBitrate, Remote: IsRemote(client.IP, cfg.LocalNetworks)}, nil
}

// decide runs the decider, reading the file's keyframe index only when a
// Direct Stream needs it (R13): a Direct Play never touches it.
func (s *Service) decide(ctx context.Context, m *Media, r Request, p Profile, lim Limits) (Decision, error) {
	d, err := Decide(*m, r, p, lim)
	if err != nil || !slices.Contains(d.Reasons, ReasonKeyframeIndex) || m.indexRead || !s.hlsAvailable() {
		return d, err
	}
	m.indexRead = true
	times, err := s.keyframes.Load(ctx, m.FileID, m.Path, m.Container, m.ModTime)
	if err != nil {
		s.o.Log.Warn("reading the keyframe index failed; transcoding instead", "file_id", m.FileID, "err", err)
	}
	if len(times) == 0 {
		return d, nil
	}
	m.Keyframes, m.HasKeyframeIndex = times, true
	return Decide(*m, r, p, lim)
}

// admit checks a decision can be served now, taking a transcode slot for
// a video transcode (R26, R27, R31).
func (s *Service) admit(d Decision) (slot bool, err error) {
	switch {
	case d.Method == DirectPlay:
		return false, nil
	case !s.hlsAvailable():
		return false, ErrFFmpegMissing
	case s.storageFull():
		return false, ErrTranscodeStorage
	case d.Method == Transcode:
		if !s.acquireSlot() {
			return false, ErrTranscodeLimit
		}
		return true, nil
	}
	return false, nil
}

// Change is a PATCH of a session (R6). Nil fields keep their value.
type Change struct {
	PositionMs      int64
	AudioIndex      *int
	SubtitleIndex   *int
	SubtitlesOff    bool // with SubtitleSet: subtitles off
	SubtitleSet     bool
	Quality         *string
	MaxBitrate      *int64
	MaxBitrateClear bool
	BurnSubtitles   *bool
}

// ChangeSession decides again with the changed request (R6). An unchanged
// decision keeps the session's URL; a changed one gets a new token and a
// new stream that starts at the client's position.
func (s *Service) ChangeSession(ctx context.Context, p auth.Principal, client clientinfo.Info, id string, c Change) (Session, error) {
	sess, err := s.manager.Get(p, id)
	if err != nil || sess.UserID != p.UserID { // admins end others' sessions, they don't change them
		return Session{}, ErrSessionNotFound
	}
	if _, err := s.visibleItem(ctx, p, sess.ItemID); err != nil {
		return Session{}, err
	}
	r := sess.Request
	if c.AudioIndex != nil {
		r.AudioIndex = c.AudioIndex
	}
	if c.SubtitleSet {
		r.SubtitleIndex, r.SubtitlesOff = c.SubtitleIndex, c.SubtitlesOff
	}
	if c.Quality != nil {
		r.Quality = *c.Quality
	}
	switch {
	case c.MaxBitrateClear:
		r.MaxBitrate = 0
	case c.MaxBitrate != nil:
		r.MaxBitrate = *c.MaxBitrate
	}
	if c.BurnSubtitles != nil {
		r.BurnSubtitles = *c.BurnSubtitles
	}
	r = s.supported(r)
	lim, err := s.limits(ctx, client)
	if err != nil {
		return Session{}, err
	}
	m := sess.Decision.Media
	d, err := s.decide(ctx, &m, r, sess.Profile, lim)
	if err != nil {
		return Session{}, err
	}
	pos := min(max(c.PositionMs, 0), m.DurationMs)
	if sameDecision(d, sess.Decision) {
		return s.manager.Update(p, id, func(x *Session) {
			x.Request, x.PositionMs, x.Decision.Media = r, pos, m
		})
	}

	// A new stream: take a slot first, so a refusal leaves the session as
	// it was (R26).
	st := s.stream(id)
	hadSlot := false
	if st != nil {
		st.mu.Lock()
		hadSlot = st.slot
		st.mu.Unlock()
	}
	keepSlot := hadSlot && d.Method == Transcode
	if d.Method != DirectPlay && !keepSlot {
		slot, err := s.admit(d)
		if err != nil {
			return Session{}, err
		}
		keepSlot = slot
	}
	s.closeStream(id, keepSlot)
	gen := 0
	if st != nil {
		gen = genOf(st.dir) + 1
	}
	sess, err = s.manager.Rekey(id, func(x *Session) {
		x.Decision, x.Request, x.StartPositionMs, x.PositionMs, x.Remote = d, r, pos, pos, lim.Remote
	})
	if err != nil {
		if keepSlot {
			s.releaseSlot()
		}
		return Session{}, err
	}
	if d.Method != DirectPlay {
		s.newStream(sess, gen, keepSlot)
	}
	s.o.Log.Info("playback changed", "playback_id", sess.ID, "method", d.Method, "reasons", d.Reasons,
		"quality", d.Quality, "position_ms", pos)
	return sess, nil
}

// sameDecision reports whether two decisions produce the same stream.
func sameDecision(a, b Decision) bool {
	idx := func(s *Subtitle) int {
		if s == nil {
			return -1
		}
		return s.Index
	}
	aud := func(s *library.Stream) int {
		if s == nil {
			return -1
		}
		return s.Index
	}
	return a.Method == b.Method && aud(a.Audio) == aud(b.Audio) && a.AudioCodec == b.AudioCodec &&
		a.AudioChannels == b.AudioChannels && a.AudioCopy == b.AudioCopy &&
		idx(a.Subtitle) == idx(b.Subtitle) && a.SubtitleDelivery == b.SubtitleDelivery &&
		a.Quality == b.Quality && a.Width == b.Width && a.Height == b.Height && a.VideoBitrate == b.VideoBitrate
}

// genOf reads the generation at the end of a stream folder's name.
func genOf(dir string) int {
	base := filepath.Base(dir)
	n, _ := strconv.Atoi(base[strings.LastIndexByte(base, '-')+1:])
	return n
}

// onEnd runs when a session ends: its stream stops and its last position
// is saved.
func (s *Service) onEnd(sess Session) {
	s.closeStream(sess.ID, false)
	s.saveFinal(sess)
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
// never reported one (VLC on a direct link) leave watch state alone; HLS
// sessions use the last segment they asked for (R34).
func (s *Service) saveFinal(sess Session) {
	if !sess.reported && !sess.fromSegments {
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
