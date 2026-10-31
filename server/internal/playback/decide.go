package playback

import (
	"errors"
	"slices"
	"strings"

	"github.com/gh-prim/plaid/server/internal/library"
)

// Methods, from the lightest to the heaviest (R4).
const (
	DirectPlay   = "direct_play"
	DirectStream = "direct_stream"
	Transcode    = "transcode"
)

// Reasons a file isn't played directly (R5). Stable keys clients translate.
const (
	ReasonContainer        = "container_not_supported"
	ReasonVideoCodec       = "video_codec_not_supported"
	ReasonVideoProfile     = "video_profile_not_supported"
	ReasonResolution       = "resolution_too_high"
	ReasonBitDepth         = "bit_depth_not_supported"
	ReasonHDR              = "hdr_not_supported"
	ReasonAudioCodec       = "audio_codec_not_supported"
	ReasonAudioChannels    = "audio_channels_too_many"
	ReasonAudioTrackSwitch = "audio_track_switch_unsupported"
	ReasonSubtitleBurnIn   = "subtitle_burn_in"
	ReasonBitrate          = "bitrate_too_high"
	ReasonKeyframeIndex    = "keyframe_index_missing"
	ReasonForced           = "forced_by_client"
)

// WarningColors flags a transcode whose colors can't be mapped right
// (Dolby Vision without an HDR10 base layer, or an FFmpeg without
// zscale, R23).
const WarningColors = "colors_may_be_wrong"

// Subtitle deliveries (R16, R17).
const (
	DeliveryEmbedded = "embedded" // the client reads it from the file it plays
	DeliveryExternal = "external" // fetched separately as WebVTT (or ASS)
	DeliveryBurnIn   = "burn_in"  // drawn into the video
)

// Errors of Decide. The handlers turn them into API errors.
var (
	ErrFileNotLinked   = errors.New("file not linked to the item")
	ErrFileUnavailable = errors.New("file missing from disk")
	ErrFileNotAnalyzed = errors.New("file not analyzed yet")
	ErrBadStream       = errors.New("no such stream in the file")
	ErrNotPlayable     = errors.New("the client can't receive this file in any way")
)

// Quality is one rung of the transcode ladder (R21).
type Quality struct {
	Name    string
	Width   int
	Height  int
	Bitrate int64
}

// QualityOriginal is the source as is, for Direct Play and Direct Stream.
const QualityOriginal = "original"

// Ladder is R21's quality ladder, best first.
var Ladder = []Quality{
	{"2160p", 3840, 2160, 20_000_000},
	{"1080p", 1920, 1080, 8_000_000},
	{"720p", 1280, 720, 4_000_000},
	{"480p", 854, 480, 1_500_000},
	{"360p", 640, 360, 750_000},
}

// Request is what the client asks for (R1).
type Request struct {
	FileID        string
	AudioIndex    *int
	SubtitleIndex *int
	SubtitlesOff  bool
	MaxBitrate    int64
	Quality       string
	BurnSubtitles bool
	ForceMethod   string // "", direct_stream or transcode
}

// Limits are the server side of the decision.
type Limits struct {
	// RemoteMaxBitrate applies when Remote is set (R22); 0 for no limit.
	RemoteMaxBitrate int64
	Remote           bool
	// Tonemap: FFmpeg can tone map HDR to SDR (zscale and tonemap, R23).
	Tonemap bool
}

// Decision is how a file is played.
type Decision struct {
	Method  string
	Reasons []string
	Media   Media

	Audio *library.Stream // selected track, nil for a file without audio
	// AudioCodec is what the client receives: the source codec when copied.
	AudioCodec    string
	AudioChannels int
	AudioCopy     bool

	Subtitle         *Subtitle // selected track, nil for none
	SubtitleDelivery string

	VideoCopy bool
	// Quality is "original" without transcoding, else a Ladder name.
	Quality   string
	Qualities []string
	// Target of a transcode.
	Width, Height int
	VideoBitrate  int64
	VideoCodec    string
	// Tonemap maps an HDR source to SDR (R23).
	Tonemap bool
	// HWAccel is the accelerator a transcode runs on, "" in software. The
	// service sets it; it can drop to "" after a hardware failure (R24).
	HWAccel string

	MaxBitrate int64 // the effective limit, 0 for none
	Warnings   []string
}

// ChooseFile picks the file to play among an item's files (R3): the one
// asked for, else the first available file the client plays directly,
// else the highest resolution.
func ChooseFile(files []library.PlayableFile, fileID string, decide func(Media) Decision) (Media, error) {
	usable := func(f library.PlayableFile) error {
		switch {
		case f.MissingSince != nil:
			return ErrFileUnavailable
		case f.Probe == nil:
			return ErrFileNotAnalyzed
		}
		return nil
	}
	if fileID != "" {
		for _, f := range files {
			if f.ID == fileID {
				if err := usable(f); err != nil {
					return Media{}, err
				}
				return mediaFrom(f), nil
			}
		}
		return Media{}, ErrFileNotLinked
	}
	var candidates []Media
	missing := false
	for _, f := range files {
		if err := usable(f); err != nil {
			missing = missing || errors.Is(err, ErrFileUnavailable)
			continue
		}
		candidates = append(candidates, mediaFrom(f))
	}
	if len(candidates) == 0 {
		if missing {
			return Media{}, ErrFileUnavailable
		}
		return Media{}, ErrFileNotAnalyzed
	}
	for _, m := range candidates {
		if decide(m).Method == DirectPlay {
			return m, nil
		}
	}
	best := candidates[0]
	for _, m := range candidates[1:] {
		if pixels(m) > pixels(best) || (pixels(m) == pixels(best) && m.Bitrate > best.Bitrate) {
			best = m
		}
	}
	return best, nil
}

func pixels(m Media) int {
	if m.Video == nil {
		return 0
	}
	return m.Video.Width * m.Video.Height
}

// Decide picks the lightest method that works (R4) for a file, a request
// and a client. It's a pure function: no I/O.
func Decide(m Media, req Request, p Profile, lim Limits) (Decision, error) {
	d := Decision{Media: m, MaxBitrate: effectiveBitrate(req, p, lim)}

	// Tracks (R18).
	if req.AudioIndex != nil {
		if d.Audio = m.audio(*req.AudioIndex); d.Audio == nil {
			return Decision{}, ErrBadStream
		}
	} else {
		d.Audio = m.defaultAudio()
	}
	switch {
	case req.SubtitlesOff:
	case req.SubtitleIndex != nil:
		if d.Subtitle = m.subtitle(*req.SubtitleIndex); d.Subtitle == nil {
			return Decision{}, ErrBadStream
		}
	case d.Audio != nil:
		for i := range m.Subtitles {
			if s := &m.Subtitles[i]; s.Forced && s.Language == d.Audio.Language {
				d.Subtitle = s
				break
			}
		}
	}

	videoReasons := videoProblems(m, p)
	audioReasons := d.audioProblems(p)
	var reasons []string
	if !p.hasProtocol(ProtocolHTTP) || !p.hasContainer(m.Container) {
		reasons = append(reasons, ReasonContainer)
	}
	reasons = append(reasons, videoReasons...)
	reasons = append(reasons, audioReasons...)
	burn := d.Subtitle != nil && (req.BurnSubtitles || (d.Subtitle.Image &&
		!p.rendersSubtitle(d.Subtitle.Format(), true) && !p.drawsSeparately(d.Subtitle.Format())))
	if burn {
		reasons = append(reasons, ReasonSubtitleBurnIn)
	}
	tooFast := d.MaxBitrate > 0 && m.Bitrate > d.MaxBitrate
	if tooFast {
		reasons = append(reasons, ReasonBitrate)
	}
	// A lower quality than the source is the client's choice too: without
	// it, a file that plays directly would ignore the quality menu.
	lowered := req.Quality != "" && req.Quality != QualityOriginal
	if req.ForceMethod != "" || (lowered && len(reasons) == 0) {
		reasons = append(reasons, ReasonForced)
	}

	if len(reasons) == 0 {
		d.Method, d.Reasons = DirectPlay, []string{}
		d.VideoCopy, d.AudioCopy = true, true
		d.setAudioOut(true, p)
		d.SubtitleDelivery = subtitleDelivery(d.Subtitle, p, true)
		d.Quality, d.Qualities = QualityOriginal, qualities(m, true)
		return d, nil
	}
	d.Reasons = reasons

	if !p.hasProtocol(ProtocolHLS) {
		return Decision{}, ErrNotPlayable
	}
	// HLS segments can't carry image subtitles: over HLS they're always
	// burned in, even for a client that renders them in a file (R17).
	if d.Subtitle != nil && d.Subtitle.Image && !burn && !p.drawsSeparately(d.Subtitle.Format()) {
		burn = true
		d.Reasons = append(d.Reasons, ReasonSubtitleBurnIn)
	}
	streamable := len(videoReasons) == 0 && !burn && !tooFast && req.ForceMethod != Transcode &&
		m.Video != nil && slices.Contains([]string{"h264", "hevc", "av1"}, m.Video.Codec)
	if streamable && !m.HasKeyframeIndex {
		d.Reasons = append(d.Reasons, ReasonKeyframeIndex)
		streamable = false
	}
	if streamable && (req.Quality == "" || req.Quality == QualityOriginal) {
		d.Method, d.VideoCopy = DirectStream, true
		d.setAudioOut(len(audioReasons) == 0 || onlyTrackSwitch(audioReasons), p)
		d.SubtitleDelivery = subtitleDelivery(d.Subtitle, p, false)
		d.Quality, d.Qualities = QualityOriginal, qualities(m, true)
		return d, nil
	}

	d.Method, d.VideoCodec = Transcode, "h264"
	d.setAudioOut(len(audioReasons) == 0 || onlyTrackSwitch(audioReasons), p)
	if burn {
		d.SubtitleDelivery = DeliveryBurnIn
	} else {
		d.SubtitleDelivery = subtitleDelivery(d.Subtitle, p, false)
	}
	d.Qualities = qualities(m, streamable)
	q := pickQuality(m, p, req.Quality, d.MaxBitrate)
	d.Quality = q.Name
	d.Width, d.Height = fit(m, q)
	d.VideoBitrate = q.Bitrate
	if d.MaxBitrate > 0 && d.VideoBitrate > d.MaxBitrate {
		d.VideoBitrate = d.MaxBitrate
	}
	if m.Bitrate > 0 && d.VideoBitrate > m.Bitrate {
		d.VideoBitrate = m.Bitrate
	}
	// The output is SDR (R20): HDR10, HDR10+, HLG and Dolby Vision with
	// an HDR10 or HLG base layer are tone mapped; Dolby Vision 5 has no
	// base layer to map and 8.2 is SDR already (R23).
	switch {
	case m.HDR == "" || (m.HDR == library.DolbyVision && m.DVCompat == 2):
	case m.HDR == library.DolbyVision && m.DVCompat == 0, !lim.Tonemap:
		d.Warnings = append(d.Warnings, WarningColors)
	default:
		d.Tonemap = true
	}
	if d.Warnings == nil {
		d.Warnings = []string{}
	}
	return d, nil
}

func effectiveBitrate(req Request, p Profile, lim Limits) int64 {
	var limit int64
	for _, b := range []int64{req.MaxBitrate, p.MaxBitrate} {
		if b > 0 && (limit == 0 || b < limit) {
			limit = b
		}
	}
	if lim.Remote && lim.RemoteMaxBitrate > 0 && (limit == 0 || lim.RemoteMaxBitrate < limit) {
		limit = lim.RemoteMaxBitrate
	}
	return limit
}

// videoProblems lists why the client can't decode the video stream as is.
func videoProblems(m Media, p Profile) []string {
	v := m.Video
	if v == nil {
		return nil
	}
	c := p.video(v.Codec)
	if c == nil {
		return []string{ReasonVideoCodec}
	}
	var out []string
	profileOK := len(c.Profiles) == 0 || slices.ContainsFunc(c.Profiles, func(s string) bool { return strings.EqualFold(s, v.Profile) })
	if !profileOK || (c.MaxLevel > 0 && v.Level > c.MaxLevel) {
		out = append(out, ReasonVideoProfile)
	}
	if (c.MaxWidth > 0 && v.Width > c.MaxWidth) || (c.MaxHeight > 0 && v.Height > c.MaxHeight) {
		out = append(out, ReasonResolution)
	}
	if v.BitDepth > c.maxBitDepth() {
		out = append(out, ReasonBitDepth)
	}
	if m.HDR != "" && !hdrShown(m, c) {
		out = append(out, ReasonHDR)
	}
	return out
}

// hdrShown reports whether a client shows the file's HDR format, directly
// or through the base layer it carries: HDR10+ plays as HDR10, Dolby
// Vision 8.1 as HDR10, 8.4 as HLG, 8.2 as SDR.
func hdrShown(m Media, c *VideoCap) bool {
	if c == nil {
		return false
	}
	has := func(f string) bool { return slices.Contains(c.HDR, f) }
	switch m.HDR {
	case "":
		return true
	case library.HDR10Plus:
		return has(library.HDR10Plus) || has(library.HDR10)
	case library.DolbyVision:
		switch m.DVCompat {
		case 1, 6:
			return has(library.DolbyVision) || has(library.HDR10)
		case 4:
			return has(library.DolbyVision) || has(library.HLG)
		case 2:
			return true
		}
		return has(library.DolbyVision)
	}
	return has(m.HDR)
}

// audioProblems lists why the client can't play the selected audio as is.
func (d *Decision) audioProblems(p Profile) []string {
	a := d.Audio
	if a == nil {
		return nil
	}
	var out []string
	switch c := p.audio(a.Codec); {
	case c == nil:
		out = append(out, ReasonAudioCodec)
	case a.Channels > c.maxChannels():
		out = append(out, ReasonAudioChannels)
	}
	if !a.Default && len(d.Media.Audio) > 1 && !p.CanSwitchAudioTrack {
		out = append(out, ReasonAudioTrackSwitch)
	}
	return out
}

// onlyTrackSwitch: picking a non-default track needs a remux, not an
// audio conversion.
func onlyTrackSwitch(reasons []string) bool {
	return len(reasons) == 1 && reasons[0] == ReasonAudioTrackSwitch
}

// setAudioOut fills what the client receives (R19): the source as is, else
// AAC (5.1 when the client takes it), or AC-3/E-AC-3 kept when supported.
func (d *Decision) setAudioOut(copyOK bool, p Profile) {
	a := d.Audio
	if a == nil {
		return
	}
	if copyOK {
		d.AudioCopy, d.AudioCodec, d.AudioChannels = true, a.Codec, a.Channels
		return
	}
	d.AudioCopy = false
	if c := p.audio("eac3"); c != nil && a.Codec == "eac3" && a.Channels <= c.maxChannels() {
		d.AudioCopy, d.AudioCodec, d.AudioChannels = true, "eac3", a.Channels
		return
	}
	d.AudioCodec, d.AudioChannels = "aac", 2
	if c := p.audio("aac"); c != nil && c.maxChannels() >= 6 && a.Channels >= 6 {
		d.AudioChannels = 6
	}
}

// subtitleDelivery says how the selected subtitle reaches the client when
// it isn't burned in.
func subtitleDelivery(s *Subtitle, p Profile, direct bool) string {
	if s == nil {
		return ""
	}
	// A web player can't read image tracks from the file: it draws them
	// from a stream of their own.
	if s.Image && p.drawsSeparately(s.Format()) {
		return DeliveryExternal
	}
	if direct && s.ExternalPath == "" && p.rendersSubtitle(s.Format(), s.Image) {
		return DeliveryEmbedded
	}
	if s.Image {
		return DeliveryBurnIn
	}
	return DeliveryExternal
}

// qualities lists what a session can switch to: "original" when the
// source can be sent without re-encoding, then every rung the source fills.
func qualities(m Media, original bool) []string {
	var out []string
	if original {
		out = append(out, QualityOriginal)
	}
	for _, q := range Ladder {
		if fills(m, q) {
			out = append(out, q.Name)
		}
	}
	if len(out) == 0 {
		out = append(out, Ladder[len(Ladder)-1].Name)
	}
	return out
}

// fills reports whether the source is at least as big as a rung, counting
// wide films (1920×800) as 1080p.
func fills(m Media, q Quality) bool {
	if m.Video == nil {
		return false
	}
	return m.Video.Height >= q.Height*9/10 || m.Video.Width >= q.Width*9/10
}

// pickQuality is the asked rung when the source fills it, else the best
// one under the bitrate limit and the client's H.264 size limits.
func pickQuality(m Media, p Profile, want string, maxBitrate int64) Quality {
	c := p.video("h264")
	allowed := func(q Quality) bool {
		if !fills(m, q) && q != Ladder[len(Ladder)-1] {
			return false
		}
		if c != nil && ((c.MaxWidth > 0 && q.Width > c.MaxWidth) || (c.MaxHeight > 0 && q.Height > c.MaxHeight)) {
			return false
		}
		return true
	}
	for _, q := range Ladder {
		if q.Name == want && allowed(q) {
			return q
		}
	}
	for _, q := range Ladder {
		if allowed(q) && (maxBitrate == 0 || q.Bitrate <= maxBitrate) {
			return q
		}
	}
	return Ladder[len(Ladder)-1]
}

// fit scales the source into a rung's box, keeping its aspect ratio, with
// even dimensions; never upscaled.
func fit(m Media, q Quality) (w, h int) {
	if m.Video == nil || m.Video.Width == 0 || m.Video.Height == 0 {
		return q.Width, q.Height
	}
	sw, sh := m.Video.Width, m.Video.Height
	w, h = sw, sh
	if w > q.Width {
		w, h = q.Width, sh*q.Width/sw
	}
	if h > q.Height {
		w, h = sw*q.Height/sh, q.Height
	}
	return w &^ 1, h &^ 1
}
