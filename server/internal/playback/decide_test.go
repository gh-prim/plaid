package playback

import (
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
)

// fixture builds an analyzed file from one of F004's recorded ffprobe
// outputs, at path (whose extension sets the container), then lets the
// case adjust it.
func fixture(t *testing.T, name, path string, adjust ...func(*library.PlayableFile)) library.PlayableFile {
	t.Helper()
	raw, err := os.ReadFile("../library/testdata/probe/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	probe, err := library.ParseProbe(raw)
	if err != nil {
		t.Fatal(err)
	}
	f := library.PlayableFile{Probe: probe}
	f.ID, f.Path, f.ProbeStatus = name, path, "ok"
	f.DurationMs, f.Bitrate = probe.DurationMs, probe.Bitrate
	if v := probe.Video(); v != nil {
		f.HDRFormat, f.DVProfile, f.DVCompat = v.HDR, v.DVProfile, v.DVCompat
	}
	for _, a := range adjust {
		a(&f)
	}
	return f
}

func media(t *testing.T, name, path string, adjust ...func(*library.PlayableFile)) Media {
	t.Helper()
	m := mediaFrom(fixture(t, name, path, adjust...))
	m.HasKeyframeIndex = true
	return m
}

// audioTo replaces the first audio stream's codec and channels.
func audioTo(codec string, channels int) func(*library.PlayableFile) {
	return func(f *library.PlayableFile) {
		for i := range f.Probe.Streams {
			if f.Probe.Streams[i].Type == "audio" {
				f.Probe.Streams[i].Codec, f.Probe.Streams[i].Channels = codec, channels
				return
			}
		}
	}
}

func externalSRT(index int, lang string) func(*library.PlayableFile) {
	return func(f *library.PlayableFile) {
		f.Streams = append(f.Streams, library.MediaStream{
			Index: index, Type: "subtitle", Codec: "subrip", Language: lang, External: true, ExternalPath: "/media/x." + lang + ".srt",
		})
	}
}

// chrome is the spec's example profile: a Chromium browser.
var chrome = Profile{
	Protocols:  []string{ProtocolHTTP, ProtocolHLS},
	Containers: []string{"mp4", "webm"},
	Video: []VideoCap{
		{Codec: "h264", Profiles: []string{"baseline", "main", "high"}, MaxLevel: 52, MaxWidth: 3840, MaxHeight: 2160},
		{Codec: "vp9", MaxBitDepth: 10},
		{Codec: "av1", MaxBitDepth: 10},
	},
	Audio:         []AudioCap{{Codec: "aac", MaxChannels: 6}, {Codec: "opus", MaxChannels: 8}, {Codec: "mp3"}},
	TextSubtitles: []string{"vtt"},
}

// pgsChrome is chrome drawing PGS itself from a stream of its own (V2).
var pgsChrome = func() Profile {
	p := chrome
	p.SeparateSubtitles = []string{"pgs"}
	return p
}()

// appleTV plays HEVC 10-bit with HDR10 but not Dolby Vision, and switches
// audio tracks itself.
var appleTV = Profile{
	Protocols:  []string{ProtocolHTTP, ProtocolHLS},
	Containers: []string{"mp4", "mov"},
	Video: []VideoCap{
		{Codec: "h264", MaxLevel: 52},
		{Codec: "hevc", Profiles: []string{"main", "main 10"}, MaxLevel: 153, MaxBitDepth: 10, HDR: []string{"hdr10", "hlg"}},
	},
	Audio:               []AudioCap{{Codec: "aac", MaxChannels: 6}, {Codec: "ac3", MaxChannels: 6}, {Codec: "eac3", MaxChannels: 8}},
	TextSubtitles:       []string{"vtt"},
	CanSwitchAudioTrack: true,
}

func intp(n int) *int { return &n }

func TestDecide(t *testing.T) {
	cases := []struct {
		name    string
		media   Media
		req     Request
		profile Profile
		limits  Limits
		method  string
		reasons []string // exactly, in order
		check   func(*testing.T, Decision)
	}{
		{
			name:  "H.264/AAC MP4 in a browser plays directly",
			media: media(t, "sdr_h264_stereo", "/media/Movie.mp4"), profile: chrome,
			method: DirectPlay, reasons: []string{},
			check: func(t *testing.T, d Decision) {
				if d.Quality != QualityOriginal || !d.VideoCopy || !d.AudioCopy || d.AudioCodec != "aac" {
					t.Errorf("decision = %+v", d)
				}
			},
		},
		{
			name:  "H.264/DTS MKV in a browser is direct-streamed with AAC",
			media: media(t, "sdr_h264_stereo", "/media/Movie.mkv", audioTo("dts", 6)), profile: chrome,
			method: DirectStream, reasons: []string{ReasonContainer, ReasonAudioCodec},
			check: func(t *testing.T, d Decision) {
				if !d.VideoCopy || d.AudioCopy || d.AudioCodec != "aac" || d.AudioChannels != 6 {
					t.Errorf("audio out = %s %dch copy=%v", d.AudioCodec, d.AudioChannels, d.AudioCopy)
				}
			},
		},
		{
			name: "the same MKV without a keyframe index is transcoded",
			media: func() Media {
				m := media(t, "sdr_h264_stereo", "/media/Movie.mkv")
				m.HasKeyframeIndex = false
				return m
			}(), profile: chrome,
			method: Transcode, reasons: []string{ReasonContainer, ReasonKeyframeIndex},
		},
		{
			name:  "HEVC 10-bit HDR10 in a browser without HEVC is transcoded to H.264",
			media: media(t, "hevc_hdr10_multiaudio", "/media/Dune.mkv"), profile: chrome,
			method: Transcode, reasons: []string{ReasonContainer, ReasonVideoCodec, ReasonAudioCodec},
			check: func(t *testing.T, d Decision) {
				if d.VideoCodec != "h264" || d.Quality != "2160p" || d.Width != 3840 || d.Height != 2160 {
					t.Errorf("target = %s %s %dx%d", d.VideoCodec, d.Quality, d.Width, d.Height)
				}
				if d.VideoBitrate > d.Media.Bitrate {
					t.Errorf("video bitrate %d above the source's %d", d.VideoBitrate, d.Media.Bitrate)
				}
			},
		},
		{
			name:  "HDR10 transcoded for an SDR client is tone mapped (R23)",
			media: media(t, "hevc_hdr10_multiaudio", "/media/Dune.mkv"), profile: chrome, limits: Limits{Tonemap: true},
			method: Transcode, reasons: []string{ReasonContainer, ReasonVideoCodec, ReasonAudioCodec},
			check: func(t *testing.T, d Decision) {
				if !d.Tonemap || len(d.Warnings) != 0 {
					t.Errorf("tonemap = %v, warnings = %v", d.Tonemap, d.Warnings)
				}
			},
		},
		{
			name:  "without zscale the same file is flagged instead (R23)",
			media: media(t, "hevc_hdr10_multiaudio", "/media/Dune.mkv"), profile: chrome,
			method: Transcode, reasons: []string{ReasonContainer, ReasonVideoCodec, ReasonAudioCodec},
			check: func(t *testing.T, d Decision) {
				if d.Tonemap || !slices.Contains(d.Warnings, WarningColors) {
					t.Errorf("tonemap = %v, warnings = %v", d.Tonemap, d.Warnings)
				}
			},
		},
		{
			name:  "HLG is tone mapped too",
			media: media(t, "hlg", "/media/HLG.mkv"), profile: chrome, limits: Limits{Tonemap: true},
			method: Transcode, reasons: []string{ReasonContainer, ReasonVideoCodec},
			check: func(t *testing.T, d Decision) {
				if !d.Tonemap {
					t.Errorf("tonemap = %v", d.Tonemap)
				}
			},
		},
		{
			name:  "SDR isn't tone mapped",
			media: media(t, "sdr_h264_stereo", "/media/Movie.mp4"), profile: chrome, req: Request{ForceMethod: Transcode}, limits: Limits{Tonemap: true},
			method: Transcode, reasons: []string{ReasonForced},
			check: func(t *testing.T, d Decision) {
				if d.Tonemap || len(d.Warnings) != 0 {
					t.Errorf("tonemap = %v, warnings = %v", d.Tonemap, d.Warnings)
				}
			},
		},
		{
			name:  "the same file in VLC plays directly",
			media: media(t, "hevc_hdr10_multiaudio", "/media/Dune.mkv"), profile: ExternalProfile(),
			method: DirectPlay, reasons: []string{},
		},
		{
			name:  "a PGS track selected forces a transcode with burn-in",
			media: media(t, "pgs_subtitles", "/media/Film.mkv"), profile: chrome, req: Request{SubtitleIndex: intp(2)},
			method: Transcode, reasons: []string{ReasonContainer, ReasonSubtitleBurnIn},
			check: func(t *testing.T, d Decision) {
				if d.SubtitleDelivery != DeliveryBurnIn {
					t.Errorf("delivery = %s", d.SubtitleDelivery)
				}
			},
		},
		{
			name:  "a web player that draws PGS gets it on its own stream: no burn-in, no transcode",
			media: media(t, "pgs_subtitles", "/media/Film.mkv"), profile: pgsChrome, req: Request{SubtitleIndex: intp(2)},
			method: DirectStream, reasons: []string{ReasonContainer},
			check: func(t *testing.T, d Decision) {
				if d.SubtitleDelivery != DeliveryExternal {
					t.Errorf("delivery = %s", d.SubtitleDelivery)
				}
			},
		},
		{
			name:  "an SRT track selected is sent separately, no transcode",
			media: media(t, "pgs_subtitles", "/media/Film.mkv", externalSRT(1000, "fre")), profile: chrome, req: Request{SubtitleIndex: intp(1000)},
			method: DirectStream, reasons: []string{ReasonContainer},
			check: func(t *testing.T, d Decision) {
				if d.SubtitleDelivery != DeliveryExternal {
					t.Errorf("delivery = %s", d.SubtitleDelivery)
				}
			},
		},
		{
			name:  "VLC renders an embedded PGS track itself",
			media: media(t, "pgs_subtitles", "/media/Film.mkv"), profile: ExternalProfile(), req: Request{SubtitleIndex: intp(2)},
			method: DirectPlay, reasons: []string{},
			check: func(t *testing.T, d Decision) {
				if d.SubtitleDelivery != DeliveryEmbedded {
					t.Errorf("delivery = %s", d.SubtitleDelivery)
				}
			},
		},
		{
			name:    "a 40 Mbps file sent outside the local network is transcoded under the limit",
			media:   media(t, "sdr_h264_stereo", "/media/Movie.mp4", func(f *library.PlayableFile) { f.Bitrate = 40_000_000 }),
			profile: chrome, limits: Limits{Remote: true, RemoteMaxBitrate: 8_000_000},
			method: Transcode, reasons: []string{ReasonBitrate},
			check: func(t *testing.T, d Decision) {
				if d.MaxBitrate != 8_000_000 || d.VideoBitrate > 8_000_000 || d.Quality != "720p" {
					t.Errorf("limit %d, video %d at %s", d.MaxBitrate, d.VideoBitrate, d.Quality)
				}
			},
		},
		{
			name:    "the same file at home plays directly",
			media:   media(t, "sdr_h264_stereo", "/media/Movie.mp4", func(f *library.PlayableFile) { f.Bitrate = 40_000_000 }),
			profile: chrome, limits: Limits{Remote: false, RemoteMaxBitrate: 8_000_000},
			method: DirectPlay, reasons: []string{},
		},
		{
			name:  "Dolby Vision 8.1 plays as HDR10 on a client without Dolby Vision",
			media: media(t, "dolby_vision_p81", "/media/DV.mkv"), profile: appleTV,
			method: DirectStream, reasons: []string{ReasonContainer},
		},
		{
			name:    "Dolby Vision 5 is transcoded with a color warning",
			media:   media(t, "dolby_vision_p81", "/media/DV.mkv", func(f *library.PlayableFile) { f.DVProfile, f.DVCompat = 5, 0 }),
			profile: appleTV, limits: Limits{Tonemap: true},
			method: Transcode, reasons: []string{ReasonContainer, ReasonHDR},
			check: func(t *testing.T, d Decision) {
				if d.Tonemap || !slices.Contains(d.Warnings, WarningColors) {
					t.Errorf("warnings = %v", d.Warnings)
				}
			},
		},
		{
			name:    "7.1 TrueHD is converted for a client that takes AAC 5.1",
			media:   media(t, "sdr_h264_stereo", "/media/Movie.mp4", audioTo("truehd", 8)),
			profile: chrome,
			method:  DirectStream, reasons: []string{ReasonAudioCodec},
			check: func(t *testing.T, d Decision) {
				if d.AudioCodec != "aac" || d.AudioChannels != 6 {
					t.Errorf("audio out = %s %dch", d.AudioCodec, d.AudioChannels)
				}
			},
		},
		{
			name:    "a non-default audio track needs a remux when the player can't switch",
			media:   media(t, "hevc_hdr10_multiaudio", "/media/Dune.mp4"),
			profile: func() Profile { p := appleTV; p.CanSwitchAudioTrack = false; return p }(),
			req:     Request{AudioIndex: intp(2)},
			method:  DirectStream, reasons: []string{ReasonAudioTrackSwitch},
			check: func(t *testing.T, d Decision) {
				if !d.AudioCopy || d.Audio.Index != 2 {
					t.Errorf("audio = %+v copy=%v", d.Audio, d.AudioCopy)
				}
			},
		},
		{
			name:    "the French audio track brings its forced French subtitles",
			media:   media(t, "hevc_hdr10_multiaudio", "/media/Dune.mp4"),
			profile: appleTV, req: Request{AudioIndex: intp(2)},
			method: DirectPlay, reasons: []string{},
			check: func(t *testing.T, d Decision) {
				if d.Subtitle == nil || d.Subtitle.Index != 3 || !d.Subtitle.Forced || d.SubtitleDelivery != DeliveryExternal {
					t.Errorf("subtitle = %+v via %s", d.Subtitle, d.SubtitleDelivery)
				}
			},
		},
		{
			name:    "no forced subtitles with the English track, and none when turned off",
			media:   media(t, "hevc_hdr10_multiaudio", "/media/Dune.mp4"),
			profile: appleTV, req: Request{AudioIndex: intp(2), SubtitlesOff: true},
			method: DirectPlay, reasons: []string{},
			check: func(t *testing.T, d Decision) {
				if d.Subtitle != nil {
					t.Errorf("subtitle = %+v, want none", d.Subtitle)
				}
			},
		},
		{
			name:  "forceMethod transcode is honored",
			media: media(t, "sdr_h264_stereo", "/media/Movie.mp4"), profile: chrome, req: Request{ForceMethod: Transcode},
			method: Transcode, reasons: []string{ReasonForced},
		},
		{
			name:  "asking for 480p transcodes a file that would play directly",
			media: media(t, "sdr_h264_stereo", "/media/Movie.mp4"), profile: chrome, req: Request{Quality: "480p"},
			method: Transcode, reasons: []string{ReasonForced},
			check: func(t *testing.T, d Decision) {
				if d.Quality != "480p" || d.Height != 480 {
					t.Errorf("quality %s %dx%d", d.Quality, d.Width, d.Height)
				}
			},
		},
		{
			name:  "asking for 480p transcodes a 720p source to 854×480",
			media: media(t, "sdr_h264_stereo", "/media/Movie.mkv"), profile: chrome, req: Request{Quality: "480p"},
			method: Transcode, reasons: []string{ReasonContainer},
			check: func(t *testing.T, d Decision) {
				if d.Quality != "480p" || d.Width != 854 || d.Height != 480 {
					t.Errorf("quality %s %dx%d", d.Quality, d.Width, d.Height)
				}
				if !slices.Equal(d.Qualities, []string{QualityOriginal, "720p", "480p", "360p"}) {
					t.Errorf("qualities = %v", d.Qualities)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := Decide(c.media, c.req, c.profile, c.limits)
			if err != nil {
				t.Fatal(err)
			}
			if d.Method != c.method || !slices.Equal(d.Reasons, c.reasons) {
				t.Errorf("got %s %v, want %s %v", d.Method, d.Reasons, c.method, c.reasons)
			}
			if c.check != nil {
				c.check(t, d)
			}
		})
	}
}

func TestDecideErrors(t *testing.T) {
	m := media(t, "sdr_h264_stereo", "/media/Movie.mkv")
	if _, err := Decide(m, Request{AudioIndex: intp(99)}, chrome, Limits{}); !errors.Is(err, ErrBadStream) {
		t.Errorf("unknown audio track: %v", err)
	}
	if _, err := Decide(m, Request{SubtitleIndex: intp(42)}, chrome, Limits{}); !errors.Is(err, ErrBadStream) {
		t.Errorf("unknown subtitle track: %v", err)
	}
	httpOnly := chrome
	httpOnly.Protocols = []string{ProtocolHTTP}
	if _, err := Decide(m, Request{}, httpOnly, Limits{}); !errors.Is(err, ErrNotPlayable) {
		t.Errorf("an MKV for an HTTP-only browser: %v", err)
	}
}

func TestChooseFile(t *testing.T) {
	decide := func(p Profile) func(Media) Decision {
		return func(m Media) Decision {
			d, _ := Decide(m, Request{}, p, Limits{})
			return d
		}
	}
	gone := time.Now()
	hd := fixture(t, "sdr_h264_stereo", "/media/Movie - 720p.mp4")
	uhd := fixture(t, "hevc_hdr10_multiaudio", "/media/Movie - 2160p.mkv", func(f *library.PlayableFile) { f.ID = "uhd" })
	missing := fixture(t, "sdr_h264_stereo", "/media/Movie - old.mp4", func(f *library.PlayableFile) { f.ID, f.MissingSince = "old", &gone })
	pending := fixture(t, "sdr_h264_stereo", "/media/Movie - new.mp4", func(f *library.PlayableFile) { f.ID, f.Probe = "new", nil })

	if m, err := ChooseFile([]library.PlayableFile{uhd, hd}, "", decide(chrome)); err != nil || m.FileID != hd.ID {
		t.Errorf("browser: got %s (%v), want the directly playable 720p file", m.FileID, err)
	}
	if m, err := ChooseFile([]library.PlayableFile{hd, uhd}, "", decide(Profile{Protocols: []string{ProtocolHLS}})); err != nil || m.FileID != "uhd" {
		t.Errorf("HLS-only client: got %s (%v), want the highest resolution", m.FileID, err)
	}
	if m, err := ChooseFile([]library.PlayableFile{missing, hd}, "", decide(chrome)); err != nil || m.FileID != hd.ID {
		t.Errorf("with a missing file: got %s (%v)", m.FileID, err)
	}
	for name, c := range map[string]struct {
		files  []library.PlayableFile
		fileID string
		want   error
	}{
		"unknown file id":       {[]library.PlayableFile{hd}, "nope", ErrFileNotLinked},
		"asked file missing":    {[]library.PlayableFile{hd, missing}, "old", ErrFileUnavailable},
		"asked file unanalyzed": {[]library.PlayableFile{hd, pending}, "new", ErrFileNotAnalyzed},
		"every file missing":    {[]library.PlayableFile{missing, pending}, "", ErrFileUnavailable},
		"nothing analyzed":      {[]library.PlayableFile{pending}, "", ErrFileNotAnalyzed},
	} {
		if _, err := ChooseFile(c.files, c.fileID, decide(chrome)); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}
