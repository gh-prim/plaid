package playback

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/library"
)

// Media is a file as the decider sees it: its container, what ffprobe said
// about its streams, and its external subtitles.
type Media struct {
	FileID     string
	Path       string
	Size       int64
	ModTime    time.Time
	Container  string // short name from the extension (mkv, mp4…)
	DurationMs int64
	Bitrate    int64
	Video      *library.Stream
	Audio      []library.Stream
	Subtitles  []Subtitle
	HDR        string // stored hdr_format: knows HDR10+, which needs a frame read
	DVProfile  int
	DVCompat   int
	// HasKeyframeIndex is set when the container index lists the video
	// keyframes, which Direct Stream needs to cut segments (R13).
	HasKeyframeIndex bool
	// Keyframes are the video keyframe times in ms, from the index.
	Keyframes []int64
	indexRead bool // the index was looked for
}

// Subtitle is an embedded or external subtitle track.
type Subtitle struct {
	Index        int
	Codec        string
	Language     string
	Title        string
	Default      bool
	Forced       bool
	SDH          bool
	Image        bool
	ExternalPath string // set for subtitle files next to the video
}

// Format is the subtitle's short name as clients list it: vtt, srt, ass,
// pgs, vobsub; "" for formats no client renders (DVB, xsub).
func (s Subtitle) Format() string {
	switch s.Codec {
	case "subrip", "srt":
		return "srt"
	case "ass", "ssa":
		return "ass"
	case "webvtt":
		return "vtt"
	case "mov_text":
		return "tx3g"
	case "hdmv_pgs_subtitle":
		return "pgs"
	case "dvd_subtitle":
		return "vobsub"
	}
	return ""
}

// containerNames maps file extensions to the container names clients list.
var containerNames = map[string]string{
	".mkv": "mkv", ".webm": "webm", ".mp4": "mp4", ".m4v": "mp4", ".mov": "mov",
	".ts": "ts", ".m2ts": "ts", ".mts": "ts", ".avi": "avi", ".wmv": "wmv",
	".mpg": "mpeg", ".mpeg": "mpeg", ".flv": "flv", ".ogv": "ogg", ".3gp": "3gp",
}

// ContentType is the MIME type a file is served with for Direct Play.
func (m Media) ContentType() string {
	switch m.Container {
	case "mp4":
		return "video/mp4"
	case "mkv":
		return "video/x-matroska"
	case "webm":
		return "video/webm"
	case "mov":
		return "video/quicktime"
	case "ts":
		return "video/mp2t"
	case "avi":
		return "video/x-msvideo"
	}
	return "application/octet-stream"
}

// mediaFrom builds a Media from an analyzed file. Embedded streams come
// from the analysis, external subtitles from the stored streams.
func mediaFrom(f library.PlayableFile) Media {
	m := Media{
		FileID: f.ID, Path: f.Path, Size: f.Size, ModTime: f.ModTime,
		Container:  containerNames[strings.ToLower(filepath.Ext(f.Path))],
		DurationMs: f.DurationMs, Bitrate: f.Bitrate,
		HDR: f.HDRFormat, DVProfile: f.DVProfile, DVCompat: f.DVCompat,
	}
	if f.Probe != nil {
		if m.DurationMs == 0 {
			m.DurationMs = f.Probe.DurationMs
		}
		if m.Bitrate == 0 {
			m.Bitrate = f.Probe.Bitrate
		}
		for _, s := range f.Probe.Streams {
			switch s.Type {
			case "video":
				if m.Video == nil {
					v := s
					m.Video = &v
				}
			case "audio":
				m.Audio = append(m.Audio, s)
			case "subtitle":
				m.Subtitles = append(m.Subtitles, Subtitle{
					Index: s.Index, Codec: s.Codec, Language: s.Language, Title: s.Title,
					Default: s.Default, Forced: s.Forced, SDH: s.SDH, Image: s.Image,
				})
			}
		}
	}
	for _, s := range f.Streams {
		if s.External && s.Type == "subtitle" {
			m.Subtitles = append(m.Subtitles, Subtitle{
				Index: s.Index, Codec: s.Codec, Language: s.Language, Title: s.Title,
				Default: s.Default, Forced: s.Forced, SDH: s.SDH, Image: s.Image, ExternalPath: s.ExternalPath,
			})
		}
	}
	return m
}

func (m Media) audio(index int) *library.Stream {
	for i := range m.Audio {
		if m.Audio[i].Index == index {
			return &m.Audio[i]
		}
	}
	return nil
}

func (m Media) subtitle(index int) *Subtitle {
	for i := range m.Subtitles {
		if m.Subtitles[i].Index == index {
			return &m.Subtitles[i]
		}
	}
	return nil
}

// defaultAudio is the file's default audio track, else the first one.
func (m Media) defaultAudio() *library.Stream {
	for i := range m.Audio {
		if m.Audio[i].Default {
			return &m.Audio[i]
		}
	}
	if len(m.Audio) > 0 {
		return &m.Audio[0]
	}
	return nil
}
