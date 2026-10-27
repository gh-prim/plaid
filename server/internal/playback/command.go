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
	// HW is the accelerator of a transcode run, nil in software (R24).
	HW *HWConfig
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
	if hw := r.hw(); hw != nil {
		a = append(a, hwDeviceArgs(hw)...)
		if m.Video != nil && hw.decodes(m.Video.Codec) {
			a = append(a, hwDecodeArgs(hw, m.Video.Codec, r.gpuChain())...)
		}
	}
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

// hw is the run's accelerator: transcodes only.
func (r RunSpec) hw() *HWConfig {
	if r.Decision.Method != Transcode {
		return nil
	}
	return r.HW
}

// gpuChain reports whether decoded frames stay on the GPU for scaling
// (and tone mapping) before the encoder: the GPU decodes the codec, and
// tone mapping, when needed, has a GPU filter (R24).
func (r RunSpec) gpuChain() bool {
	hw, m := r.hw(), r.Decision.Media
	return hw != nil && hw.spec().gpuFrames && m.Video != nil && hw.decodes(m.Video.Codec) &&
		(!r.Decision.Tonemap || hw.Tonemap)
}

// swTonemap maps HDR10 and HLG to SDR BT.709 (R23).
const swTonemap = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv"

func (r RunSpec) videoTranscode() []string {
	d := r.Decision
	var a []string
	switch hw := r.hw(); {
	case hw == nil:
		a = r.swFilters(",format=yuv420p")
	case !r.gpuChain():
		// Software decoding or filters, GPU encoding.
		if up := hw.spec().upload; up != "" {
			a = r.swFilters(",format=nv12," + up)
		} else {
			a = r.swFilters(",format=yuv420p")
		}
	default:
		a = r.gpuFilters(hw)
	}
	a = append(a, r.encoderArgs()...)
	if d.Tonemap {
		a = append(a, "-color_primaries", "bt709", "-color_trc", "bt709", "-colorspace", "bt709")
	}
	return a
}

// swFilters draws burned-in subtitles at the source size, then scales,
// tone maps when needed, and ends with tail (pixel format, upload).
func (r RunSpec) swFilters(tail string) []string {
	d := r.Decision
	m := d.Media
	scale := fmt.Sprintf("scale=%d:%d", d.Width, d.Height)
	if d.Tonemap {
		scale += "," + swTonemap
	}
	scale += tail
	switch sub := d.Subtitle; {
	case d.SubtitleDelivery == DeliveryBurnIn && sub.Image:
		return []string{"-filter_complex", fmt.Sprintf("[%s][%s]overlay=eof_action=pass,%s[v]", stream(m.Video.Index), stream(sub.Index), scale), "-map", "[v]"}
	case d.SubtitleDelivery == DeliveryBurnIn:
		return []string{"-map", stream(m.Video.Index), "-vf", subtitlesFilter(m, *sub) + "," + scale}
	}
	return []string{"-map", stream(m.Video.Index), "-vf", scale}
}

// gpuFilters scale (and tone map) on the GPU. Burned-in subtitles go
// through system memory: download, overlay in software, upload (R24).
func (r RunSpec) gpuFilters(hw *HWConfig) []string {
	d := r.Decision
	m := d.Media
	a := hw.spec()
	pixfmt := "nv12"
	var scale string
	switch hw.Accel {
	case HWVAAPI:
		scale = fmt.Sprintf("scale_vaapi=w=%d:h=%d:format=nv12", d.Width, d.Height)
		if d.Tonemap {
			scale = fmt.Sprintf("scale_vaapi=w=%d:h=%d,tonemap_vaapi=format=nv12:t=bt709:m=bt709:p=bt709", d.Width, d.Height)
		}
	case HWQSV:
		scale = fmt.Sprintf("scale_qsv=w=%d:h=%d:format=nv12", d.Width, d.Height)
		if d.Tonemap {
			scale = fmt.Sprintf("vpp_qsv=w=%d:h=%d:format=nv12:tonemap=1", d.Width, d.Height)
		}
	default:
		pixfmt = "yuv420p"
		scale = fmt.Sprintf("%s=w=%d:h=%d:format=yuv420p", a.scale, d.Width, d.Height)
	}
	upload := ""
	if a.upload != "" {
		upload = ",format=nv12," + a.upload
	}
	down := ",hwdownload,format=" + pixfmt
	switch sub := d.Subtitle; {
	case d.SubtitleDelivery == DeliveryBurnIn && sub.Image:
		// The subtitle canvas is the source's size: scaled to the output's.
		return []string{"-filter_complex", fmt.Sprintf("[%s]%s%s[v0];[%s]scale=%d:%d[s0];[v0][s0]overlay=eof_action=pass%s[v]",
			stream(m.Video.Index), scale, down, stream(sub.Index), d.Width, d.Height, upload), "-map", "[v]"}
	case d.SubtitleDelivery == DeliveryBurnIn:
		return []string{"-map", stream(m.Video.Index), "-vf", scale + down + "," + subtitlesFilter(m, *sub) + upload}
	}
	return []string{"-map", stream(m.Video.Index), "-vf", scale}
}

// encoderArgs are the H.264 encoder's options (R20): libx264 High in
// software, the accelerator's encoder otherwise, keyframes forced on
// segment boundaries either way.
func (r RunSpec) encoderArgs() []string {
	d := r.Decision
	rate := strconv.FormatInt(d.VideoBitrate, 10)
	rates := []string{"-b:v", rate, "-maxrate", rate, "-bufsize", strconv.FormatInt(2*d.VideoBitrate, 10),
		"-force_key_frames", "expr:gte(t,n_forced*6)"}
	hw := r.hw()
	if hw == nil {
		level := "4.1"
		if d.Width > 1920 || d.Height > 1080 {
			level = "5.1"
		}
		preset := x264Presets[r.Preset]
		if preset == "" {
			preset = x264Presets[PresetFast]
		}
		a := []string{"-c:v", "libx264", "-preset", preset, "-profile:v", "high", "-level:v", level}
		return append(append(a, rates...), "-sc_threshold", "0")
	}
	sp := hw.spec()
	a := []string{"-c:v", encoderFor(hw.Accel, "h264")}
	if sp.presets != nil {
		preset := sp.presets[r.Preset]
		if preset == "" {
			preset = sp.presets[PresetFast]
		}
		a = append(a, "-preset", preset)
	}
	if hw.Accel != HWV4L2M2M {
		a = append(a, "-profile:v", "high")
	}
	a = append(a, sp.extra...)
	return append(a, rates...)
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
