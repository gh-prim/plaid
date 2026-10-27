package library

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ErrFFprobeMissing means no ffprobe binary was found (R25). Files are then
// listed with probe_status = skipped.
var ErrFFprobeMissing = errors.New("ffprobe_missing")

// probeTimeout bounds the analysis of one file (R12).
const probeTimeout = 30 * time.Second

// HDR formats stored in media_files.hdr_format.
const (
	HDR10       = "hdr10"
	HDR10Plus   = "hdr10plus"
	DolbyVision = "dolby_vision"
	HLG         = "hlg"
)

// Prober analyzes a media file. The scanner calls it from several goroutines.
type Prober interface {
	Probe(ctx context.Context, path string) (*ProbeResult, error)
}

// ProbeResult is what ffprobe says about a file (R12).
type ProbeResult struct {
	Container  string
	DurationMs int64
	Bitrate    int64
	Streams    []Stream
	// Raw is ffprobe's JSON output, kept for later features.
	Raw []byte
}

// Stream is one video, audio or subtitle stream.
type Stream struct {
	Index    int
	Type     string // video, audio, subtitle
	Codec    string
	Profile  string
	Language string // ISO 639-2/B, "und" when unknown
	Title    string
	Default  bool
	Forced   bool
	SDH      bool

	// Video
	Width     int
	Height    int
	FrameRate float64
	BitDepth  int
	HDR       string // one of the HDR constants, "" for SDR
	DVProfile int    // Dolby Vision profile, with DVCompat: 8.1 = 8 and 1
	DVCompat  int

	// Audio
	Channels int
	Layout   string

	// Subtitle
	Image bool // bitmap subtitles (PGS, VobSub, DVB)
}

// Video returns the main video stream, or nil.
func (r *ProbeResult) Video() *Stream {
	for i := range r.Streams {
		if r.Streams[i].Type == "video" {
			return &r.Streams[i]
		}
	}
	return nil
}

// FFprobe runs the ffprobe binary.
type FFprobe struct {
	Path    string
	Version string
	Timeout time.Duration
	Log     *slog.Logger
	// run executes ffprobe with args; tests replace it with fixtures.
	run func(ctx context.Context, args ...string) ([]byte, error)
}

// FindFFprobe locates ffprobe: the configured path (PLAID_FFPROBE_PATH) when
// set, else PATH. It returns ErrFFprobeMissing when there is none.
func FindFFprobe(ctx context.Context, configured string, log *slog.Logger) (*FFprobe, error) {
	name := "ffprobe"
	if configured != "" {
		name = configured
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFFprobeMissing, err)
	}
	if log == nil {
		log = slog.Default()
	}
	p := &FFprobe{Path: path, Timeout: probeTimeout, Log: log}
	p.run = p.exec
	vctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := p.exec(vctx, "-version")
	if err != nil {
		return nil, fmt.Errorf("%w: %s -version: %w", ErrFFprobeMissing, path, err)
	}
	// "ffprobe version 7.1.1-1 Copyright (c) ..."
	if f := strings.Fields(string(out)); len(f) >= 3 && f[1] == "version" {
		p.Version = f[2]
	}
	return p, nil
}

func (p *FFprobe) exec(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, p.Path, args...) //nolint:gosec // admin-configured ffprobe binary
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	lowerPriority(cmd.Process.Pid, p.Log)
	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("ffprobe timed out: %w", ctx.Err())
		}
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return nil, fmt.Errorf("ffprobe: %w: %s", err, msg)
	}
	return stdout.Bytes(), nil
}

// Probe analyzes one file within the timeout.
func (p *FFprobe) Probe(ctx context.Context, path string) (*ProbeResult, error) {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = probeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	raw, err := p.run(ctx, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	if err != nil {
		return nil, err
	}
	res, err := parseProbe(raw)
	if err != nil {
		return nil, err
	}
	// HDR10+ lives in per-frame side data: read one frame, only for PQ video.
	if v := res.Video(); v != nil && v.HDR == HDR10 {
		frames, err := p.run(ctx, "-v", "error", "-print_format", "json", "-select_streams", "v:0",
			"-read_intervals", "%+#1", "-show_frames", path)
		if err != nil {
			p.Log.Debug("ffprobe frame read failed, keeping HDR10", "path", path, "err", err)
		} else if hasHDR10Plus(frames) {
			v.HDR = HDR10Plus
		}
	}
	return res, nil
}

type ffStream struct {
	Index            int               `json:"index"`
	CodecName        string            `json:"codec_name"`
	CodecType        string            `json:"codec_type"`
	Profile          string            `json:"profile"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	PixFmt           string            `json:"pix_fmt"`
	BitsPerRawSample string            `json:"bits_per_raw_sample"`
	RFrameRate       string            `json:"r_frame_rate"`
	AvgFrameRate     string            `json:"avg_frame_rate"`
	ColorTransfer    string            `json:"color_transfer"`
	ColorPrimaries   string            `json:"color_primaries"`
	Channels         int               `json:"channels"`
	ChannelLayout    string            `json:"channel_layout"`
	Disposition      map[string]int    `json:"disposition"`
	Tags             map[string]string `json:"tags"`
	SideDataList     []map[string]any  `json:"side_data_list"`
}

type ffOutput struct {
	Streams []ffStream `json:"streams"`
	Format  struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
}

var imageSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true, "dvd_subtitle": true, "dvb_subtitle": true, "xsub": true,
}

func parseProbe(raw []byte) (*ProbeResult, error) {
	var out ffOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("ffprobe output: %w", err)
	}
	res := &ProbeResult{Container: out.Format.FormatName, Raw: raw}
	if d, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
		res.DurationMs = int64(math.Round(d * 1000))
	}
	res.Bitrate, _ = strconv.ParseInt(out.Format.BitRate, 10, 64)

	for _, s := range out.Streams {
		if s.Disposition["attached_pic"] == 1 {
			continue // cover art
		}
		st := Stream{
			Index:    s.Index,
			Type:     s.CodecType,
			Codec:    s.CodecName,
			Profile:  s.Profile,
			Language: streamLanguage(tag(s.Tags, "language")),
			Title:    tag(s.Tags, "title"),
			Default:  s.Disposition["default"] == 1,
			Forced:   s.Disposition["forced"] == 1,
			SDH:      s.Disposition["hearing_impaired"] == 1,
		}
		switch s.CodecType {
		case "video":
			st.Width, st.Height = s.Width, s.Height
			st.FrameRate = frameRate(s.AvgFrameRate)
			if st.FrameRate == 0 {
				st.FrameRate = frameRate(s.RFrameRate)
			}
			st.BitDepth = bitDepth(s)
			st.HDR, st.DVProfile, st.DVCompat = hdrFormat(s)
		case "audio":
			st.Channels, st.Layout = s.Channels, s.ChannelLayout
		case "subtitle":
			st.Image = imageSubtitleCodecs[s.CodecName]
		default:
			continue // data, attachments
		}
		res.Streams = append(res.Streams, st)
	}
	if len(res.Streams) == 0 {
		return nil, errors.New("ffprobe found no audio or video streams")
	}
	return res, nil
}

func tag(tags map[string]string, key string) string {
	for k, v := range tags {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// frameRate parses "24000/1001".
func frameRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return 0
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

func bitDepth(s ffStream) int {
	if n, err := strconv.Atoi(s.BitsPerRawSample); err == nil && n > 0 {
		return n
	}
	switch {
	case strings.Contains(s.PixFmt, "12"):
		return 12
	case strings.Contains(s.PixFmt, "10"):
		return 10
	case s.PixFmt != "":
		return 8
	}
	return 0
}

// hdrFormat reads the stream metadata. Dolby Vision wins over the HDR10 base
// layer it may carry; HDR10+ needs a frame and is decided by Probe.
func hdrFormat(s ffStream) (format string, dvProfile, dvCompat int) {
	for _, sd := range s.SideDataList {
		if sd["side_data_type"] == "DOVI configuration record" {
			return DolbyVision, intField(sd["dv_profile"]), intField(sd["dv_bl_signal_compatibility_id"])
		}
	}
	switch {
	case s.ColorTransfer == "smpte2084" && strings.HasPrefix(s.ColorPrimaries, "bt2020"):
		return HDR10, 0, 0
	case s.ColorTransfer == "arib-std-b67":
		return HLG, 0, 0
	}
	return "", 0, 0
}

func intField(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

func hasHDR10Plus(frames []byte) bool {
	var out struct {
		Frames []struct {
			SideDataList []map[string]any `json:"side_data_list"`
		} `json:"frames"`
	}
	if json.Unmarshal(frames, &out) != nil {
		return false
	}
	for _, f := range out.Frames {
		for _, sd := range f.SideDataList {
			if t, _ := sd["side_data_type"].(string); strings.Contains(t, "SMPTE2094-40") {
				return true
			}
		}
	}
	return false
}
