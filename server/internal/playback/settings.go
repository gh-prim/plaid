package playback

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"

	"github.com/gh-prim/plaid/server/internal/settings"
)

// Settings keys (R30), stored as JSON in F005's settings table.
const (
	keyRemoteMaxBitrate     = "playback.remote_max_bitrate"
	keyLocalNetworks        = "playback.local_networks"
	keyMaxTranscodes        = "playback.max_transcodes"
	keyTranscodeDirMaxBytes = "playback.transcode_dir_max_bytes"
	keyEncoderPreset        = "playback.encoder_preset"
	keyHWAccel              = "playback.hw_accel"
	keyHWDevice             = "playback.hw_device"
)

// Config is the admin's playback settings. Zero values mean "default".
type Config struct {
	RemoteMaxBitrate     int64 // bits per second, 0 for no limit
	LocalNetworks        []netip.Prefix
	MaxTranscodes        int   // 0: DefaultMaxTranscodes
	TranscodeDirMaxBytes int64 // 0: the cap computed at startup
	EncoderPreset        string
	HWAccel              string // R24: auto (default), none or an accelerator
	HWDevice             string // "": the first /dev/dri/renderD*
}

// LoadConfig reads the playback settings, with defaults for the unset ones.
// A malformed value is ignored rather than breaking playback.
func LoadConfig(ctx context.Context, s *settings.Store) (Config, error) {
	c := Config{EncoderPreset: PresetFast, HWAccel: HWAuto}
	if s == nil {
		return c, nil
	}
	get := func(key string, dst any) error {
		v, ok, err := s.Get(ctx, key)
		if err == nil && ok {
			_ = json.Unmarshal([]byte(v), dst)
		}
		return err
	}
	var nets []string
	for key, dst := range map[string]any{
		keyRemoteMaxBitrate: &c.RemoteMaxBitrate, keyLocalNetworks: &nets, keyMaxTranscodes: &c.MaxTranscodes,
		keyTranscodeDirMaxBytes: &c.TranscodeDirMaxBytes, keyEncoderPreset: &c.EncoderPreset,
		keyHWAccel: &c.HWAccel, keyHWDevice: &c.HWDevice,
	} {
		if err := get(key, dst); err != nil {
			return c, err
		}
	}
	for _, n := range nets {
		if p, err := netip.ParsePrefix(n); err == nil {
			c.LocalNetworks = append(c.LocalNetworks, p.Masked())
		}
	}
	if _, ok := x264Presets[c.EncoderPreset]; !ok {
		c.EncoderPreset = PresetFast
	}
	if !slices.Contains(HWChoices, c.HWAccel) {
		c.HWAccel = HWAuto
	}
	c.MaxTranscodes = max(c.MaxTranscodes, 0)
	c.TranscodeDirMaxBytes = max(c.TranscodeDirMaxBytes, 0)
	return c, nil
}

// ConfigUpdate is a partial change of the settings. A nil field keeps its
// value; Reset* fields go back to the default.
type ConfigUpdate struct {
	RemoteMaxBitrate          *int64
	LocalNetworks             *[]string
	MaxTranscodes             *int
	ResetMaxTranscodes        bool
	TranscodeDirMaxBytes      *int64
	ResetTranscodeDirMaxBytes bool
	EncoderPreset             *string
	HWAccel                   *string
	HWDevice                  *string
	ResetHWDevice             bool
}

// FieldError is a setting that failed validation.
type FieldError struct{ Field, Code string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Code }

// validate checks a change before anything runs or is stored.
func (u ConfigUpdate) validate() error {
	if v := u.RemoteMaxBitrate; v != nil && *v < 0 {
		return &FieldError{"remoteMaxBitrate", "out_of_range"}
	}
	if v := u.LocalNetworks; v != nil {
		for _, n := range *v {
			if _, err := netip.ParsePrefix(n); err != nil {
				return &FieldError{"localNetworks", "invalid"}
			}
		}
	}
	if v := u.MaxTranscodes; !u.ResetMaxTranscodes && v != nil && (*v < 1 || *v > 32) {
		return &FieldError{"maxTranscodes", "out_of_range"}
	}
	if v := u.TranscodeDirMaxBytes; !u.ResetTranscodeDirMaxBytes && v != nil && *v < 1 {
		return &FieldError{"transcodeDirMaxBytes", "out_of_range"}
	}
	if v := u.EncoderPreset; v != nil {
		if _, ok := x264Presets[*v]; !ok {
			return &FieldError{"encoderPreset", "invalid"}
		}
	}
	if v := u.HWAccel; v != nil && !slices.Contains(HWChoices, *v) {
		return &FieldError{"hwAccel", "invalid"}
	}
	if v := u.HWDevice; !u.ResetHWDevice && v != nil && (!filepath.IsAbs(*v) || filepath.Clean(*v) != *v) {
		return &FieldError{"hwDevice", "invalid"}
	}
	return nil
}

// SaveConfig validates and stores a settings change (R30).
func SaveConfig(ctx context.Context, s *settings.Store, u ConfigUpdate) error {
	if s == nil {
		return errors.New("no settings store")
	}
	if err := u.validate(); err != nil {
		return err
	}
	set := map[string]any{}
	var del []string
	if v := u.RemoteMaxBitrate; v != nil {
		set[keyRemoteMaxBitrate] = *v
	}
	if v := u.LocalNetworks; v != nil {
		nets := []string{}
		for _, n := range *v {
			nets = append(nets, netip.MustParsePrefix(n).Masked().String())
		}
		set[keyLocalNetworks] = nets
	}
	switch {
	case u.ResetMaxTranscodes:
		del = append(del, keyMaxTranscodes)
	case u.MaxTranscodes != nil:
		set[keyMaxTranscodes] = *u.MaxTranscodes
	}
	switch {
	case u.ResetTranscodeDirMaxBytes:
		del = append(del, keyTranscodeDirMaxBytes)
	case u.TranscodeDirMaxBytes != nil:
		set[keyTranscodeDirMaxBytes] = *u.TranscodeDirMaxBytes
	}
	if v := u.EncoderPreset; v != nil {
		set[keyEncoderPreset] = *v
	}
	if v := u.HWAccel; v != nil {
		set[keyHWAccel] = *v
	}
	switch {
	case u.ResetHWDevice:
		del = append(del, keyHWDevice)
	case u.HWDevice != nil:
		set[keyHWDevice] = *u.HWDevice
	}
	for key, v := range set {
		b, _ := json.Marshal(v)
		if err := s.Set(ctx, key, string(b)); err != nil {
			return err
		}
	}
	for _, key := range del {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// DefaultMaxTranscodes is R26's default: 4 with hardware, else 1 on four
// cores or fewer, 2 above.
func DefaultMaxTranscodes(cpus int, hardware bool) int {
	switch {
	case hardware:
		return 4
	case cpus <= 4:
		return 1
	}
	return 2
}

// DefaultStorageCap is R27's default: 20% of the free space, at most
// 10 GiB and at least 1 GiB.
func DefaultStorageCap(free uint64) int64 {
	const gib = 1 << 30
	c := int64(free / 5) //nolint:gosec // free space fits
	return min(10*gib, max(gib, c))
}

func cpuCount() int { return runtime.NumCPU() }

// usableCPUs is the cores this process may use: unlike NumCPU, it follows
// a container's CPU quota (Go 1.25+ sets GOMAXPROCS from it).
func usableCPUs() int { return runtime.GOMAXPROCS(0) }

// formatBytes is for logs.
func formatBytes(n int64) string { return strconv.FormatFloat(float64(n)/(1<<30), 'f', 1, 64) + " GiB" }
