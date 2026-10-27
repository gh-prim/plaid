package playback

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"runtime"
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
)

// Config is the admin's playback settings. Zero values mean "default".
type Config struct {
	RemoteMaxBitrate     int64 // bits per second, 0 for no limit
	LocalNetworks        []netip.Prefix
	MaxTranscodes        int   // 0: DefaultMaxTranscodes
	TranscodeDirMaxBytes int64 // 0: the cap computed at startup
	EncoderPreset        string
}

// LoadConfig reads the playback settings, with defaults for the unset ones.
// A malformed value is ignored rather than breaking playback.
func LoadConfig(ctx context.Context, s *settings.Store) (Config, error) {
	c := Config{EncoderPreset: PresetFast}
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
}

// FieldError is a setting that failed validation.
type FieldError struct{ Field, Code string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Code }

// SaveConfig validates and stores a settings change (R30).
func SaveConfig(ctx context.Context, s *settings.Store, u ConfigUpdate) error {
	if s == nil {
		return errors.New("no settings store")
	}
	set := map[string]any{}
	var del []string
	if v := u.RemoteMaxBitrate; v != nil {
		if *v < 0 {
			return &FieldError{"remoteMaxBitrate", "out_of_range"}
		}
		set[keyRemoteMaxBitrate] = *v
	}
	if v := u.LocalNetworks; v != nil {
		nets := []string{}
		for _, n := range *v {
			p, err := netip.ParsePrefix(n)
			if err != nil {
				return &FieldError{"localNetworks", "invalid"}
			}
			nets = append(nets, p.Masked().String())
		}
		set[keyLocalNetworks] = nets
	}
	switch {
	case u.ResetMaxTranscodes:
		del = append(del, keyMaxTranscodes)
	case u.MaxTranscodes != nil:
		if *u.MaxTranscodes < 1 || *u.MaxTranscodes > 32 {
			return &FieldError{"maxTranscodes", "out_of_range"}
		}
		set[keyMaxTranscodes] = *u.MaxTranscodes
	}
	switch {
	case u.ResetTranscodeDirMaxBytes:
		del = append(del, keyTranscodeDirMaxBytes)
	case u.TranscodeDirMaxBytes != nil:
		if *u.TranscodeDirMaxBytes < 1 {
			return &FieldError{"transcodeDirMaxBytes", "out_of_range"}
		}
		set[keyTranscodeDirMaxBytes] = *u.TranscodeDirMaxBytes
	}
	if v := u.EncoderPreset; v != nil {
		if _, ok := x264Presets[*v]; !ok {
			return &FieldError{"encoderPreset", "invalid"}
		}
		set[keyEncoderPreset] = *v
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

// formatBytes is for logs.
func formatBytes(n int64) string { return strconv.FormatFloat(float64(n)/(1<<30), 'f', 1, 64) + " GiB" }
