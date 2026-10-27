package handlers

import (
	"context"
	"errors"
	"slices"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/playback"
)

// StartPlayback starts playing a movie or an episode (F006 R1–R9).
func (h *Handlers) StartPlayback(ctx context.Context, req api.StartPlaybackRequestObject) (api.StartPlaybackResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.playback == nil {
		return nil, apierr.FFmpegMissing()
	}
	r, err := toStartRequest(*req.Body)
	if err != nil {
		return nil, err
	}
	sess, err := h.playback.StartSession(ctx, pr, clientinfo.FromContext(ctx), req.ItemId, r)
	if err != nil {
		return nil, playbackErr(err)
	}
	return api.StartPlayback201JSONResponse(toPlaybackSession(sess)), nil
}

// GetPlayback returns one of the caller's playback sessions.
func (h *Handlers) GetPlayback(ctx context.Context, req api.GetPlaybackRequestObject) (api.GetPlaybackResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := h.playbackSession(pr, req.SessionId)
	if err != nil {
		return nil, err
	}
	return api.GetPlayback200JSONResponse(toPlaybackSession(sess)), nil
}

// UpdatePlayback changes a session's tracks, quality or bitrate (R6).
func (h *Handlers) UpdatePlayback(ctx context.Context, req api.UpdatePlaybackRequestObject) (api.UpdatePlaybackResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.playback == nil {
		return nil, apierr.PlaybackSessionNotFound()
	}
	b := req.Body
	var p problems
	c := playback.Change{PositionMs: b.PositionMs, AudioIndex: b.AudioStreamIndex, BurnSubtitles: b.BurnSubtitles}
	if b.PositionMs < 0 {
		p.add("positionMs", "out_of_range")
	}
	if set, v := fromNullable(b.SubtitleStreamIndex); set {
		c.SubtitleSet, c.SubtitleIndex, c.SubtitlesOff = true, v, v == nil
	}
	if set, v := fromNullable(b.MaxBitrate); set {
		if v == nil {
			c.MaxBitrateClear = true
		} else if *v < 1 {
			p.add("maxBitrate", "out_of_range")
		} else {
			c.MaxBitrate = v
		}
	}
	if b.Quality != nil {
		q := string(*b.Quality)
		if !slices.Contains(qualityNames(), q) {
			p.add("quality", "invalid")
		}
		c.Quality = &q
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	sess, err := h.playback.ChangeSession(ctx, pr, clientinfo.FromContext(ctx), req.SessionId, c)
	if err != nil {
		return nil, playbackErr(err)
	}
	return api.UpdatePlayback200JSONResponse(toPlaybackSession(sess)), nil
}

// GetPlaybackSettings returns the playback settings in effect (admin, R30).
func (h *Handlers) GetPlaybackSettings(ctx context.Context, _ api.GetPlaybackSettingsRequestObject) (api.GetPlaybackSettingsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if h.playback == nil {
		return nil, apierr.FFmpegMissing()
	}
	return api.GetPlaybackSettings200JSONResponse(toPlaybackSettings(h.playback.Settings())), nil
}

// UpdatePlaybackSettings changes playback settings (admin, R30).
func (h *Handlers) UpdatePlaybackSettings(ctx context.Context, req api.UpdatePlaybackSettingsRequestObject) (api.UpdatePlaybackSettingsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if h.playback == nil {
		return nil, apierr.FFmpegMissing()
	}
	b := req.Body
	u := playback.ConfigUpdate{RemoteMaxBitrate: b.RemoteMaxBitrate, LocalNetworks: b.LocalNetworks}
	if set, v := fromNullable(b.MaxTranscodes); set {
		u.MaxTranscodes, u.ResetMaxTranscodes = v, v == nil
	}
	if set, v := fromNullable(b.TranscodeDirMaxBytes); set {
		u.TranscodeDirMaxBytes, u.ResetTranscodeDirMaxBytes = v, v == nil
	}
	if b.EncoderPreset != nil {
		preset := string(*b.EncoderPreset)
		u.EncoderPreset = &preset
	}
	if b.HwAccel != nil {
		hw := string(*b.HwAccel)
		u.HWAccel = &hw
	}
	if set, v := fromNullable(b.HwDevice); set {
		u.HWDevice, u.ResetHWDevice = v, v == nil
	}
	view, err := h.playback.SaveSettings(ctx, u)
	var fe *playback.FieldError
	if errors.As(err, &fe) {
		return nil, apierr.ValidationFailed(apierr.Detail{Field: fe.Field, Code: fe.Code})
	}
	if err != nil {
		return nil, playbackErr(err)
	}
	return api.UpdatePlaybackSettings200JSONResponse(toPlaybackSettings(view)), nil
}

func toPlaybackSettings(v playback.SettingsView) api.PlaybackSettings {
	nets := make([]string, 0, len(v.LocalNetworks))
	for _, n := range v.LocalNetworks {
		nets = append(nets, n.String())
	}
	return api.PlaybackSettings{
		MaxTranscodes: v.MaxTranscodesEffective, MaxTranscodesDefault: v.MaxTranscodesDefault,
		RemoteMaxBitrate: v.RemoteMaxBitrate, LocalNetworks: nets,
		TranscodeDirMaxBytes: v.TranscodeDirMaxBytesEffective, TranscodeDirMaxBytesDefault: v.TranscodeDirMaxBytesDefault,
		EncoderPreset: api.EncoderPreset(v.EncoderPreset),
		HwAccel:       api.HwAccelSetting(v.HWAccel), HwDevice: optional(v.HWDevice),
		Hardware: toHardwareStatus(v.Hardware),
	}
}

// toHardwareStatus is the hardware test result (R24, R31).
func toHardwareStatus(st playback.HWStatus) api.HardwareStatus {
	out := api.HardwareStatus{
		Configured: api.HwAccelSetting(st.Configured), Device: optional(st.Device), Tonemap: st.Tonemap,
		Encoders: st.Encoders, Decoders: st.Decoders,
	}
	if out.Encoders == nil {
		out.Encoders = []string{}
	}
	if out.Decoders == nil {
		out.Decoders = []string{}
	}
	if st.Active != "" {
		a := api.HardwareStatusActive(st.Active)
		out.Active = &a
	}
	if st.DisabledReason != "" {
		r := api.HardwareStatusDisabledReason(st.DisabledReason)
		out.DisabledReason = &r
	}
	return out
}

// StopPlayback ends a playback session and saves its last position (R8, R32).
func (h *Handlers) StopPlayback(ctx context.Context, req api.StopPlaybackRequestObject) (api.StopPlaybackResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.playback == nil {
		return nil, apierr.PlaybackSessionNotFound()
	}
	var pos *int64
	if req.Body != nil {
		if pos = req.Body.PositionMs; pos != nil && *pos < 0 {
			return nil, apierr.ValidationFailed(apierr.Detail{Field: "positionMs", Code: "out_of_range"})
		}
	}
	if err := h.playback.Stop(pr, req.SessionId, pos); err != nil {
		return nil, playbackErr(err)
	}
	return api.StopPlayback204Response{}, nil
}

// ReportPlaybackProgress records the position and state (R32).
func (h *Handlers) ReportPlaybackProgress(ctx context.Context, req api.ReportPlaybackProgressRequestObject) (api.ReportPlaybackProgressResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if h.playback == nil {
		return nil, apierr.PlaybackSessionNotFound()
	}
	var p problems
	if req.Body.PositionMs < 0 {
		p.add("positionMs", "out_of_range")
	}
	if !slices.Contains([]api.PlaybackState{api.Playing, api.Paused, api.Buffering, api.Stopped}, req.Body.State) {
		p.add("state", "invalid")
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	if err := h.playback.Progress(ctx, pr, req.SessionId, req.Body.PositionMs, string(req.Body.State)); err != nil {
		return nil, playbackErr(err)
	}
	return api.ReportPlaybackProgress204Response{}, nil
}

// ListPlaybackSessions lists every active playback session (admin, R29).
func (h *Handlers) ListPlaybackSessions(ctx context.Context, _ api.ListPlaybackSessionsRequestObject) (api.ListPlaybackSessionsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	out := api.ActivePlaybackList{Sessions: []api.ActivePlayback{}}
	if h.playback == nil {
		return api.ListPlaybackSessions200JSONResponse(out), nil
	}
	for _, s := range h.playback.Sessions() {
		a := api.ActivePlayback{
			Session: toPlaybackSession(s), UserId: s.UserID, Username: s.Owner.Username,
			ProfileId: s.ProfileID, ProfileName: s.Owner.ProfileName, DeviceName: s.Owner.DeviceName,
			ItemTitle: s.ItemTitle, PositionMs: s.PositionMs, Remote: s.Remote,
			StartedAt: s.CreatedAt, LastActivityAt: s.LastActivity,
		}
		if s.State != "" {
			st := api.PlaybackState(s.State)
			a.State = &st
		}
		if speed, ok := h.playback.Speed(s.ID); ok {
			a.Speed = &speed
		}
		out.Sessions = append(out.Sessions, a)
	}
	return api.ListPlaybackSessions200JSONResponse(out), nil
}

// MarkWatched marks an item watched for the active profile (R35).
func (h *Handlers) MarkWatched(ctx context.Context, req api.MarkWatchedRequestObject) (api.MarkWatchedResponseObject, error) {
	if err := h.mark(ctx, req.ItemId, true); err != nil {
		return nil, err
	}
	return api.MarkWatched204Response{}, nil
}

// MarkUnwatched marks an item unwatched for the active profile (R35).
func (h *Handlers) MarkUnwatched(ctx context.Context, req api.MarkUnwatchedRequestObject) (api.MarkUnwatchedResponseObject, error) {
	if err := h.mark(ctx, req.ItemId, false); err != nil {
		return nil, err
	}
	return api.MarkUnwatched204Response{}, nil
}

func (h *Handlers) mark(ctx context.Context, itemID string, watched bool) error {
	pr, err := principal(ctx)
	if err != nil {
		return err
	}
	if pr.ProfileID == "" {
		return apierr.ProfileRequired()
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return err
	}
	it, err := h.library.Store().GetItem(ctx, itemID)
	if errors.Is(err, library.ErrNotFound) || err == nil && !limit.Allows(it.AgeRating) {
		return apierr.NotFound("no such item")
	}
	if err != nil {
		return err
	}
	if h.playback == nil {
		return apierr.NotFound("no such item")
	}
	return h.playback.Watch().Mark(ctx, pr.ProfileID, itemID, watched)
}

// ListContinueWatching lists in-progress items and each show's next
// episode, most recent first (R37).
func (h *Handlers) ListContinueWatching(ctx context.Context, req api.ListContinueWatchingRequestObject) (api.ListContinueWatchingResponseObject, error) {
	pr, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	var p problems
	n := checkLimit(req.Params.Limit, &p)
	if err := p.err(); err != nil {
		return nil, err
	}
	out := api.ItemPage{Items: []api.ItemSummary{}}
	if pr.ProfileID == "" || h.playback == nil {
		return api.ListContinueWatching200JSONResponse(out), nil
	}
	limit, err := h.accounts.AccessLimit(ctx, pr)
	if err != nil {
		return nil, err
	}
	ids, next, err := h.playback.Watch().ContinueWatching(ctx, pr.ProfileID, limit, deref(req.Params.Cursor), n)
	if errors.Is(err, playback.ErrBadCursor) {
		return nil, apierr.ValidationFailed(apierr.Detail{Field: "cursor", Code: "invalid"})
	}
	if err != nil {
		return nil, err
	}
	if out.Items, err = h.summariesOf(ctx, ids, limit); err != nil {
		return nil, err
	}
	if err := h.withUserStates(ctx, pr, out.Items); err != nil {
		return nil, err
	}
	out.NextCursor = optional(next)
	return api.ListContinueWatching200JSONResponse(out), nil
}

// withUserStates fills the active profile's watch state into item
// summaries (R36). Without a profile, nothing is added.
func (h *Handlers) withUserStates(ctx context.Context, pr auth.Principal, items []api.ItemSummary) error {
	if h.playback == nil || pr.ProfileID == "" || len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.Id
	}
	states, err := h.playback.Watch().States(ctx, pr.ProfileID, ids)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].UserState = toUserState(states[items[i].Id], items[i].Kind)
	}
	return nil
}

func toUserState(st playback.UserState, kind api.ItemKind) *api.UserState {
	out := &api.UserState{PositionMs: st.PositionMs, Watched: st.Watched, PlayCount: st.PlayCount, LastPlayedAt: st.LastPlayedAt}
	if kind == api.ItemKindShow || kind == api.ItemKindSeason {
		w, t := st.WatchedEpisodes, st.TotalEpisodes
		out.WatchedEpisodes, out.TotalEpisodes = &w, &t
	}
	return out
}

func (h *Handlers) playbackSession(pr auth.Principal, id string) (playback.Session, error) {
	if h.playback == nil {
		return playback.Session{}, apierr.PlaybackSessionNotFound()
	}
	s, err := h.playback.Session(pr, id)
	if err != nil {
		return playback.Session{}, playbackErr(err)
	}
	return s, nil
}

// playbackErr turns playback errors into API errors.
func playbackErr(err error) error {
	switch {
	case errors.Is(err, playback.ErrProfileRequired):
		return apierr.ProfileRequired()
	case errors.Is(err, playback.ErrItemNotFound):
		return apierr.NotFound("no such item")
	case errors.Is(err, playback.ErrItemNotPlayable):
		return apierr.NotPlayable("only movies and episodes can be played")
	case errors.Is(err, playback.ErrNotPlayable):
		return apierr.NotPlayable("this client can't receive this file in any way it supports")
	case errors.Is(err, playback.ErrFileNotLinked):
		return apierr.ValidationFailed(apierr.Detail{Field: "fileId", Code: "not_found"})
	case errors.Is(err, playback.ErrBadStream):
		return apierr.ValidationFailed(apierr.Detail{Field: "streamIndex", Code: "not_found"})
	case errors.Is(err, playback.ErrFileUnavailable):
		return apierr.FileUnavailable()
	case errors.Is(err, playback.ErrFileNotAnalyzed):
		return apierr.FileNotAnalyzed()
	case errors.Is(err, playback.ErrFFmpegMissing):
		return apierr.FFmpegMissing()
	case errors.Is(err, playback.ErrHWAccelUnavailable):
		return apierr.HWAccelUnavailable()
	case errors.Is(err, playback.ErrSessionNotFound):
		return apierr.PlaybackSessionNotFound()
	case errors.Is(err, playback.ErrTranscodeLimit):
		return apierr.TranscodeLimitReached()
	case errors.Is(err, playback.ErrTranscodeStorage):
		return apierr.TranscodeStorageFull()
	case errors.Is(err, playback.ErrSegmentNotReady):
		return apierr.SegmentNotReady()
	case errors.Is(err, playback.ErrNoSuchSegment), errors.Is(err, playback.ErrNoSuchSubtitle):
		return apierr.NotFound("no such stream resource")
	}
	return err
}

// toStartRequest reads a playback request, validating what the schema
// can't (R1, R2).
func toStartRequest(b api.PlaybackRequest) (playback.StartRequest, error) {
	var p problems
	r := playback.StartRequest{Profile: toCapabilityProfile(b.Profile)}
	r.FileID = deref(b.FileId)
	r.AudioIndex = b.AudioStreamIndex
	if set, v := fromNullable(b.SubtitleStreamIndex); set {
		if v == nil {
			r.SubtitlesOff = true
		} else {
			r.SubtitleIndex = v
		}
	}
	if b.StartPositionMs != nil {
		if *b.StartPositionMs < 0 {
			p.add("startPositionMs", "out_of_range")
		}
		r.StartPositionMs = *b.StartPositionMs
	}
	if b.MaxBitrate != nil {
		if *b.MaxBitrate < 1 {
			p.add("maxBitrate", "out_of_range")
		}
		r.MaxBitrate = *b.MaxBitrate
	}
	if b.Quality != nil {
		if !slices.Contains(qualityNames(), string(*b.Quality)) {
			p.add("quality", "invalid")
		}
		r.Quality = string(*b.Quality)
	}
	if b.BurnSubtitles != nil {
		r.BurnSubtitles = *b.BurnSubtitles
	}
	if b.ForceMethod != nil {
		switch m := string(*b.ForceMethod); m {
		case playback.DirectStream, playback.Transcode:
			r.ForceMethod = m
		default:
			p.add("forceMethod", "invalid")
		}
	}
	return r, p.err()
}

func qualityNames() []string {
	names := []string{playback.QualityOriginal}
	for _, q := range playback.Ladder {
		names = append(names, q.Name)
	}
	return names
}

// toCapabilityProfile reads a capability profile. Unknown codec names stay in it
// and simply never match, so newer clients work with older servers.
func toCapabilityProfile(c api.CapabilityProfile) playback.Profile {
	if c.Preset != nil && *c.Preset == api.CapabilityProfilePresetExternal {
		return playback.ExternalProfile()
	}
	var p playback.Profile
	for _, proto := range derefSlice(c.Protocols) {
		p.Protocols = append(p.Protocols, string(proto))
	}
	p.Containers = derefSlice(c.Containers)
	for _, v := range derefSlice(c.Video) {
		vc := playback.VideoCap{Codec: v.Codec, Profiles: derefSlice(v.Profiles)}
		vc.MaxLevel, vc.MaxWidth, vc.MaxHeight, vc.MaxBitDepth = derefInt(v.MaxLevel), derefInt(v.MaxWidth), derefInt(v.MaxHeight), derefInt(v.MaxBitDepth)
		for _, hdr := range derefSlice(v.Hdr) {
			vc.HDR = append(vc.HDR, string(hdr))
		}
		p.Video = append(p.Video, vc)
	}
	for _, a := range derefSlice(c.Audio) {
		p.Audio = append(p.Audio, playback.AudioCap{Codec: a.Codec, MaxChannels: derefInt(a.MaxChannels)})
	}
	if s := c.Subtitles; s != nil {
		for _, f := range derefSlice(s.Text) {
			p.TextSubtitles = append(p.TextSubtitles, string(f))
		}
		for _, f := range derefSlice(s.Image) {
			p.ImageSubtitles = append(p.ImageSubtitles, string(f))
		}
	}
	if c.CanSwitchAudioTrack != nil {
		p.CanSwitchAudioTrack = *c.CanSwitchAudioTrack
	}
	if set, v := fromNullable(c.MaxBitrate); set && v != nil && *v > 0 {
		p.MaxBitrate = *v
	}
	return p
}

func derefSlice[T any](s *[]T) []T {
	if s == nil {
		return nil
	}
	return *s
}

func derefInt(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// toPlaybackSession is a session as clients see it (R5). Its URL carries
// the playback token: it's only ever sent to the session's owner and to
// admins.
func toPlaybackSession(s playback.Session) api.PlaybackSession {
	d := s.Decision
	out := api.PlaybackSession{
		Id: s.ID, ItemId: s.ItemID, FileId: d.Media.FileID,
		Method: api.PlaybackMethod(d.Method), Reasons: d.Reasons,
		Url:                     playback.StreamURL(s),
		ExpiresAfterIdleSeconds: int(playback.IdleTimeout.Seconds()),
		DurationMs:              d.Media.DurationMs, StartPositionMs: s.StartPositionMs,
		Quality: api.Quality(d.Quality), Qualities: make([]api.Quality, 0, len(d.Qualities)),
		Warnings: d.Warnings, AudioTracks: []api.PlaybackAudioTrack{}, Subtitles: []api.PlaybackSubtitle{},
	}
	if out.Reasons == nil {
		out.Reasons = []string{}
	}
	if d.HWAccel != "" {
		hw := api.PlaybackSessionHardwareAcceleration(d.HWAccel)
		out.HardwareAcceleration = &hw
	}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	for _, q := range d.Qualities {
		out.Qualities = append(out.Qualities, api.Quality(q))
	}
	for _, a := range d.Media.Audio {
		t := api.PlaybackAudioTrack{
			Index: a.Index, Codec: a.Codec, Channels: a.Channels, Language: a.Language,
			Title: optional(a.Title), Default: a.Default, Selected: d.Audio != nil && d.Audio.Index == a.Index,
		}
		if t.Selected && !d.AudioCopy {
			t.Delivered = optional(d.AudioCodec)
		}
		out.AudioTracks = append(out.AudioTracks, t)
	}
	for _, sub := range d.Media.Subtitles {
		out.Subtitles = append(out.Subtitles, api.PlaybackSubtitle{
			Index: sub.Index, Codec: sub.Codec, Language: sub.Language, Title: optional(sub.Title),
			Forced: sub.Forced, Selected: d.Subtitle != nil && d.Subtitle.Index == sub.Index,
			Delivery: api.PlaybackSubtitleDelivery(playback.DeliveryOf(s, sub)),
			Url:      optional(playback.SubtitleURL(s, sub)),
		})
	}
	return out
}
