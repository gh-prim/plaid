package playback

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Hardware acceleration settings (R24): an accelerator, auto or none.
const (
	HWAuto         = "auto"
	HWNone         = "none"
	HWVAAPI        = "vaapi"
	HWQSV          = "qsv"
	HWNVENC        = "nvenc"
	HWV4L2M2M      = "v4l2m2m"
	HWVideoToolbox = "videotoolbox"
)

// HWChoices are the valid hwAccel values.
var HWChoices = []string{HWAuto, HWNone, HWVAAPI, HWQSV, HWNVENC, HWV4L2M2M, HWVideoToolbox}

// hwAutoOrder is the order auto tries accelerators in (R24).
var hwAutoOrder = []string{HWVAAPI, HWQSV, HWNVENC, HWV4L2M2M, HWVideoToolbox}

// Why hardware isn't in use, in HWStatus.DisabledReason.
const (
	HWReasonNotFound         = "not_found"         // the test encode failed (or auto found nothing)
	HWReasonRepeatedFailures = "repeated_failures" // 3 failed runs in 10 minutes
)

// ErrHWAccelUnavailable means an explicitly chosen accelerator failed its
// test (R24).
var ErrHWAccelUnavailable = errors.New("hw_accel_unavailable")

// Runtime fallback (R24): a hardware run failing this early restarts in
// software; hwMaxFailures within hwFailWindow disable hardware.
const (
	hwEarlyFailure = 10 * time.Second
	hwFailWindow   = 10 * time.Minute
	hwMaxFailures  = 3
)

// accel is how FFmpeg uses one accelerator (plan step 25).
type accel struct {
	name    string
	hwaccel string // -hwaccel for decoding; "" decodes in software
	// gpuFrames keeps decoded frames on the GPU (-hwaccel_output_format),
	// so scaling happens there too.
	gpuFrames bool
	scale     string // GPU scaler, with gpuFrames
	upload    string // puts software frames on the GPU; "" when the encoder takes them as they are
	tonemap   string // GPU tone mapping, "" for none
	device    bool   // opens a render node (/dev/dri/renderD*)
	presets   map[string]string
	extra     []string // encoder options
}

var accels = map[string]accel{
	HWVAAPI: {name: HWVAAPI, hwaccel: "vaapi", gpuFrames: true, scale: "scale_vaapi", upload: "hwupload",
		tonemap: "tonemap_vaapi", device: true},
	HWQSV: {name: HWQSV, hwaccel: "qsv", gpuFrames: true, scale: "scale_qsv", upload: "hwupload=extra_hw_frames=64",
		tonemap: "vpp_qsv", device: true, presets: x264Presets, extra: []string{"-forced_idr", "1"}},
	HWNVENC: {name: HWNVENC, hwaccel: "cuda", gpuFrames: true, scale: "scale_cuda",
		presets: map[string]string{PresetFast: "p2", PresetBalanced: "p4", PresetQuality: "p6"}, extra: []string{"-forced-idr", "1"}},
	HWV4L2M2M:      {name: HWV4L2M2M},
	HWVideoToolbox: {name: HWVideoToolbox, hwaccel: "videotoolbox"},
}

// encoderFor names an accelerator's encoder for a codec (h264, hevc).
func encoderFor(name, codec string) string { return codec + "_" + name }

// HWStatus is the result of the hardware test (R24, R31).
type HWStatus struct {
	Configured string   // the setting: auto, none or an accelerator
	Active     string   // the accelerator in use, "" for software
	Device     string   // the render node, for VAAPI and QSV
	Encoders   []string // working encoders, e.g. h264_vaapi
	Decoders   []string // codecs decoded on the GPU, e.g. h264, hevc
	// Tonemap: the accelerator's tone mapping filter works.
	Tonemap        bool
	DisabledReason string
	Tested         time.Time
}

// HWConfig is the hardware a run uses; nil runs in software.
type HWConfig struct {
	Accel    string
	Device   string
	Decoders []string
	Tonemap  bool
}

func (h *HWConfig) spec() accel { return accels[h.Accel] }

// decodes reports whether the GPU decodes a codec in this chain.
func (h *HWConfig) decodes(codec string) bool {
	return h.spec().hwaccel != "" && slices.Contains(h.Decoders, codec)
}

// config is the HWConfig of an active status, nil in software.
func (st HWStatus) config() *HWConfig {
	if st.Active == "" {
		return nil
	}
	return &HWConfig{Accel: st.Active, Device: st.Device, Decoders: st.Decoders, Tonemap: st.Tonemap}
}

// renderNodes lists /dev/dri/renderD*; tests replace it.
var renderNodes = func() []string {
	nodes, _ := filepath.Glob("/dev/dri/renderD*")
	slices.Sort(nodes)
	return nodes
}

// ProbeHardware runs R24's test: list FFmpeg's accelerators and encoders,
// then per candidate a 1-second test encode to H.264 (and HEVC) and a
// decode of what it encoded. The first one whose H.264 encode works is
// kept. An explicit choice that fails returns ErrHWAccelUnavailable with
// the software status.
func ProbeHardware(ctx context.Context, ff *FFmpeg, choice, device string) (HWStatus, error) {
	st := HWStatus{Configured: choice, Tested: time.Now()}
	if choice == "" {
		st.Configured = HWAuto
	}
	switch {
	case st.Configured == HWNone:
		return st, nil
	case ff == nil && st.Configured == HWAuto:
		st.DisabledReason = HWReasonNotFound
		return st, nil
	case ff == nil:
		st.DisabledReason = HWReasonNotFound
		return st, fmt.Errorf("%w: no ffmpeg", ErrHWAccelUnavailable)
	}
	candidates := hwAutoOrder
	if st.Configured != HWAuto {
		candidates = []string{st.Configured}
	}
	hwaccels := listColumn(ctx, ff.Path, "-hwaccels", 0)
	encoders := listColumn(ctx, ff.Path, "-encoders", 1)
	var failures []string
	for _, name := range candidates {
		got, err := probeOne(ctx, ff, accels[name], device, hwaccels, encoders)
		if err == nil {
			got.Configured, got.Tested = st.Configured, st.Tested
			return got, nil
		}
		failures = append(failures, name+": "+err.Error())
	}
	st.DisabledReason = HWReasonNotFound
	if st.Configured != HWAuto {
		return st, fmt.Errorf("%w: %s", ErrHWAccelUnavailable, strings.Join(failures, "; "))
	}
	return st, nil
}

// probeOne tests one accelerator.
func probeOne(ctx context.Context, ff *FFmpeg, a accel, device string, hwaccels, encoders []string) (HWStatus, error) {
	st := HWStatus{Active: a.name}
	if !slices.Contains(encoders, encoderFor(a.name, "h264")) {
		return st, errors.New("no " + encoderFor(a.name, "h264") + " in this FFmpeg")
	}
	if a.device {
		if device == "" {
			nodes := renderNodes()
			if len(nodes) == 0 {
				return st, errors.New("no /dev/dri/renderD* device")
			}
			device = nodes[0]
		}
		st.Device = device
	}
	dir, err := os.MkdirTemp("", "plaid-hwprobe-")
	if err != nil {
		return st, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	hw := &HWConfig{Accel: a.name, Device: st.Device}
	for _, codec := range []string{"h264", "hevc"} {
		enc := encoderFor(a.name, codec)
		if !slices.Contains(encoders, enc) {
			continue
		}
		clip := filepath.Join(dir, codec+".mp4")
		if err := runProbe(ctx, ff.Path, hwTestEncode(hw, enc, clip)); err != nil {
			if codec == "h264" {
				return st, fmt.Errorf("test encode: %w", err)
			}
			continue
		}
		st.Encoders = append(st.Encoders, enc)
		if a.hwaccel != "" && slices.Contains(hwaccels, a.hwaccel) && runProbe(ctx, ff.Path, hwTestDecode(hw, codec, clip)) == nil {
			st.Decoders = append(st.Decoders, codec)
		}
	}
	switch {
	case a.tonemap == "" || !ff.HasFilter(a.tonemap):
	case a.name == HWQSV:
		// vpp_qsv only tone maps in builds that have the option.
		out, _ := exec.CommandContext(ctx, ff.Path, "-hide_banner", "-h", "filter=vpp_qsv").Output() //nolint:gosec // admin-configured binary
		st.Tonemap = strings.Contains(string(out), "tonemap")
	default:
		st.Tonemap = true
	}
	return st, nil
}

// hwTestEncode is a 1-second test encode with an accelerator's encoder.
func hwTestEncode(hw *HWConfig, encoder, out string) []string {
	a := hw.spec()
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, hwDeviceArgs(hw)...)
	args = append(args, "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=25", "-t", "1")
	if a.upload != "" {
		args = append(args, "-vf", "format=nv12,"+a.upload)
	} else {
		args = append(args, "-pix_fmt", "yuv420p")
	}
	return append(args, "-c:v", encoder, "-b:v", "2M", "-y", out)
}

// hwTestDecode decodes a test clip on the GPU. FFmpeg falls back to
// software when the accelerator fails, saying so on stderr: the probe
// fails on any error output.
func hwTestDecode(hw *HWConfig, codec, clip string) []string {
	args := append([]string{"-hide_banner", "-nostdin", "-loglevel", "error"}, hwDeviceArgs(hw)...)
	args = append(args, hwDecodeArgs(hw, codec, hw.spec().gpuFrames)...)
	return append(args, "-i", clip, "-f", "null", "-")
}

// runProbe runs one test command; any error output is a failure.
func runProbe(ctx context.Context, ffmpeg string, args []string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg, args...) //nolint:gosec // admin-configured binary, argument list
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	msg := strings.TrimSpace(stderr.String())
	switch {
	case err != nil && msg != "":
		return fmt.Errorf("%w: %s", err, lastLine(msg))
	case err != nil:
		return err
	case msg != "":
		return errors.New(lastLine(msg))
	}
	return nil
}

func lastLine(s string) string { return s[strings.LastIndexByte(s, '\n')+1:] }

// listColumn runs `ffmpeg -hwaccels` or `-encoders` and returns one column
// of its listing: hwaccels print one name per line after a header,
// encoders " V....D h264_vaapi  description".
func listColumn(ctx context.Context, ffmpeg, flag string, col int) []string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", flag).Output() //nolint:gosec // admin-configured binary
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) <= col || strings.HasSuffix(line, ":") || strings.HasPrefix(fields[0], "-") {
			continue
		}
		if col == 1 && strings.Contains(fields[1], "=") {
			continue // the legend: " V..... = Video"
		}
		names = append(names, fields[col])
	}
	return names
}

// hwDeviceArgs open the accelerator's device, before -i.
func hwDeviceArgs(hw *HWConfig) []string {
	switch hw.Accel {
	case HWVAAPI:
		return []string{"-init_hw_device", "vaapi=hw:" + hw.Device, "-filter_hw_device", "hw"}
	case HWQSV:
		return []string{"-init_hw_device", "vaapi=va:" + hw.Device, "-init_hw_device", "qsv=hw@va", "-filter_hw_device", "hw"}
	}
	return nil
}

// hwDecodeArgs decode a codec on the GPU, before -i. Without gpuFrames,
// FFmpeg copies the frames to system memory for software filters.
func hwDecodeArgs(hw *HWConfig, codec string, gpuFrames bool) []string {
	a := hw.spec()
	if a.hwaccel == "" {
		return nil
	}
	args := []string{"-hwaccel", a.hwaccel}
	if a.device {
		args = append(args, "-hwaccel_device", "hw")
	}
	if gpuFrames {
		args = append(args, "-hwaccel_output_format", a.hwaccel)
	}
	if hw.Accel == HWQSV {
		args = append(args, "-c:v", codec+"_qsv")
	}
	return args
}
