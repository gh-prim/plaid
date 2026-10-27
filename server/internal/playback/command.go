package playback

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Encoder presets (R30) and the libx264 presets they map to.
const (
	PresetFast     = "fast"
	PresetBalanced = "balanced"
	PresetQuality  = "quality"
)

var x264Presets = map[string]string{PresetFast: "veryfast", PresetBalanced: "faster", PresetQuality: "medium"}

// segmentOptions make every run of a session write the same init segment
// and keep the source timestamps, so segments of different runs join up
// after a seek (plan step 16). frag_discont keeps each fragment's real
// decode time; the edit list is forced so it always holds the B-frame
// delay: left to "auto", a run starting at 0 writes one and a run
// starting later doesn't, and without one the MP4 muxer shifts video
// later than audio by that delay.
const segmentOptions = "use_editlist=1:movflags=+frag_discont"

// RunSpec is one FFmpeg run of a session: from a piece onwards.
type RunSpec struct {
	Decision   Decision
	Layout     Layout
	StartPiece int
	Dir        string // the session folder
	RunID      int    // names the run's init segment and playlist
	Preset     string // encoderPreset; "" is fast
}

// InitName is the init segment a run writes. The first one written
// becomes the session's init.mp4; later runs write identical ones.
func (r RunSpec) InitName() string { return fmt.Sprintf("init-%d.mp4", r.RunID) }

// Args builds a run's FFmpeg arguments: always a list, never a shell
// string, and with no playback token in it (R39).
func (r RunSpec) Args() []string {
	d := r.Decision
	m := d.Media
	a := []string{"-hide_banner", "-nostdin", "-nostats", "-loglevel", "error", "-progress", "pipe:1", "-stats_period", "0.5"}
	if start := r.startMs(); start > 0 {
		// -seek_timestamp: -ss is a timestamp, not offset by the file's
		// start time. FFmpeg 7 adds a negative start (AAC priming) to it,
		// which would land a copied video on the previous keyframe.
		a = append(a, "-seek_timestamp", "1", "-ss", seconds(start))
	}
	a = append(a, "-i", m.Path, "-copyts", "-avoid_negative_ts", "disabled", "-map_metadata", "-1", "-map_chapters", "-1")

	hlsTime := "6"
	if d.Method == DirectStream {
		a = append(a, "-map", stream(m.Video.Index), "-c:v", "copy")
		if m.Video.Codec == "hevc" {
			a = append(a, "-tag:v", "hvc1") // what Apple players expect
		}
		hlsTime = "0.001" // one piece per source keyframe
	} else {
		a = append(a, r.videoTranscode()...)
	}
	a = append(a, audioArgs(d)...)
	a = append(a, "-f", "hls", "-hls_time", hlsTime, "-hls_segment_type", "fmp4",
		"-hls_segment_options", segmentOptions, "-hls_playlist_type", "vod", "-hls_flags", "temp_file",
		"-start_number", strconv.Itoa(r.StartPiece), "-hls_fmp4_init_filename", r.InitName(),
		"-hls_segment_filename", filepath.Join(r.Dir, "%d.m4s"), filepath.Join(r.Dir, fmt.Sprintf("ffmpeg-%d.m3u8", r.RunID)))
	return a
}

// startMs is the run's -ss. A Direct Stream run seeks 1 ms past its
// keyframe, so millisecond rounding never lands it on the previous one.
func (r RunSpec) startMs() int64 {
	if r.StartPiece == 0 {
		return 0
	}
	start := r.Layout.PieceStartMs[r.StartPiece]
	if r.Decision.Method == DirectStream {
		start++
	}
	return start
}

func (r RunSpec) videoTranscode() []string {
	d := r.Decision
	m := d.Media
	scale := fmt.Sprintf("scale=%d:%d,format=yuv420p", d.Width, d.Height)
	var a []string
	switch sub := d.Subtitle; {
	case d.SubtitleDelivery == DeliveryBurnIn && sub.Image:
		a = append(a, "-filter_complex", fmt.Sprintf("[%s][%s]overlay=eof_action=pass,%s[v]", stream(m.Video.Index), stream(sub.Index), scale), "-map", "[v]")
	case d.SubtitleDelivery == DeliveryBurnIn:
		a = append(a, "-map", stream(m.Video.Index), "-vf", subtitlesFilter(m, *sub)+","+scale)
	default:
		a = append(a, "-map", stream(m.Video.Index), "-vf", scale)
	}
	level := "4.1"
	if d.Width > 1920 || d.Height > 1080 {
		level = "5.1"
	}
	preset := x264Presets[r.Preset]
	if preset == "" {
		preset = x264Presets[PresetFast]
	}
	rate := strconv.FormatInt(d.VideoBitrate, 10)
	return append(a, "-c:v", "libx264", "-preset", preset, "-profile:v", "high", "-level:v", level,
		"-b:v", rate, "-maxrate", rate, "-bufsize", strconv.FormatInt(2*d.VideoBitrate, 10),
		"-force_key_frames", "expr:gte(t,n_forced*6)", "-sc_threshold", "0")
}

func audioArgs(d Decision) []string {
	if d.Audio == nil {
		return []string{"-an"}
	}
	a := []string{"-map", stream(d.Audio.Index)}
	if d.AudioCopy {
		return append(a, "-c:a", "copy")
	}
	ch := strconv.Itoa(d.AudioChannels)
	switch d.AudioCodec {
	case "ac3", "eac3":
		return append(a, "-c:a", d.AudioCodec, "-ac", ch, "-b:a", "448k")
	}
	rate := "192k"
	if d.AudioChannels > 2 {
		rate = "384k"
	}
	return append(a, "-c:a", "aac", "-ac", ch, "-b:a", rate)
}

// subtitlesFilter draws a text subtitle with libass: the external file,
// or the embedded track by its position among the file's subtitle tracks.
func subtitlesFilter(m Media, sub Subtitle) string {
	if sub.ExternalPath != "" {
		return "subtitles=filename=" + escapeFilterValue(sub.ExternalPath)
	}
	si := 0
	for _, s := range m.Subtitles {
		if s.ExternalPath == "" && s.Index < sub.Index {
			si++
		}
	}
	return fmt.Sprintf("subtitles=filename=%s:si=%d", escapeFilterValue(m.Path), si)
}

// escapeFilterValue escapes an option value for a filter graph: once for
// the filter's option parser (\ ' :), then once for the graph parser
// (\ ' [ ] , ;).
func escapeFilterValue(v string) string {
	opt := strings.NewReplacer(`\`, `\\`, `'`, `\'`, `:`, `\:`).Replace(v)
	return strings.NewReplacer(`\`, `\\`, `'`, `\'`, `[`, `\[`, `]`, `\]`, `,`, `\,`, `;`, `\;`).Replace(opt)
}

func stream(index int) string { return "0:" + strconv.Itoa(index) }

func seconds(ms int64) string { return fmt.Sprintf("%d.%03d", ms/1000, ms%1000) }
