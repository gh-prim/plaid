// Package playback decides how a client plays a file and runs the playback
// sessions of F006: Direct Play over HTTP ranges today, HLS Direct Stream
// and Transcode with FFmpeg next.
package playback

import (
	"slices"
	"strings"
)

// Protocols a client can receive.
const (
	ProtocolHTTP = "http"
	ProtocolHLS  = "hls"
)

// Profile is what a client can play (R2). It comes with every playback
// request and is never stored. The server trusts it for compatibility
// only, never for access.
type Profile struct {
	// External is the "external" preset: a player like VLC or mpv that
	// plays everything over HTTP. The other fields are ignored.
	External            bool
	Protocols           []string
	Containers          []string // short names: mp4, mkv, webm, mov, ts, avi…
	Video               []VideoCap
	Audio               []AudioCap
	TextSubtitles       []string // vtt, srt, ass
	ImageSubtitles      []string // pgs, vobsub
	CanSwitchAudioTrack bool
	MaxBitrate          int64 // bits per second, 0 for no limit
}

// VideoCap is one video codec a client decodes.
type VideoCap struct {
	Codec       string   // ffprobe name: h264, hevc, av1, vp9…
	Profiles    []string // lowercase ffprobe profile names; empty means any
	MaxLevel    int      // as ffprobe reports it; 0 means any
	MaxWidth    int      // 0 means any
	MaxHeight   int
	MaxBitDepth int // 0 means 8
	HDR         []string
}

// AudioCap is one audio codec a client decodes.
type AudioCap struct {
	Codec       string
	MaxChannels int // 0 means 2
}

// ExternalProfile is the "external" preset.
func ExternalProfile() Profile {
	return Profile{External: true, Protocols: []string{ProtocolHTTP, ProtocolHLS}, CanSwitchAudioTrack: true}
}

func (p Profile) hasProtocol(proto string) bool {
	return p.External || slices.Contains(p.Protocols, proto)
}

func (p Profile) hasContainer(c string) bool {
	return p.External || slices.Contains(p.Containers, c)
}

// video returns the client's capability for a codec, nil if it lacks it.
// The external preset decodes everything.
func (p Profile) video(codec string) *VideoCap {
	if p.External {
		return &VideoCap{Codec: codec, MaxBitDepth: 16, HDR: []string{"hdr10", "hdr10plus", "hlg", "dolby_vision"}}
	}
	for i := range p.Video {
		if strings.EqualFold(p.Video[i].Codec, codec) {
			return &p.Video[i]
		}
	}
	return nil
}

func (p Profile) audio(codec string) *AudioCap {
	if p.External {
		return &AudioCap{Codec: codec, MaxChannels: 64}
	}
	for i := range p.Audio {
		if strings.EqualFold(p.Audio[i].Codec, codec) {
			return &p.Audio[i]
		}
	}
	return nil
}

// rendersSubtitle reports whether the client draws a subtitle format
// itself, inside the file it plays directly.
func (p Profile) rendersSubtitle(format string, image bool) bool {
	if p.External {
		return true
	}
	if image {
		return slices.Contains(p.ImageSubtitles, format)
	}
	return slices.Contains(p.TextSubtitles, format)
}

func (c AudioCap) maxChannels() int {
	if c.MaxChannels <= 0 {
		return 2
	}
	return c.MaxChannels
}

func (c VideoCap) maxBitDepth() int {
	if c.MaxBitDepth <= 0 {
		return 8
	}
	return c.MaxBitDepth
}
