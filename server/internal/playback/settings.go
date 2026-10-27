package playback

import (
	"context"
	"encoding/json"
	"net/netip"

	"github.com/gh-prim/plaid/server/internal/settings"
)

// Settings keys (R30), stored as JSON in F005's settings table. The admin
// routes that change them come with HLS (plan PR B); until then they keep
// their defaults unless set in the database.
const (
	keyRemoteMaxBitrate = "playback.remote_max_bitrate"
	keyLocalNetworks    = "playback.local_networks"
)

// Config is the admin's playback settings.
type Config struct {
	RemoteMaxBitrate int64 // bits per second, 0 for no limit
	LocalNetworks    []netip.Prefix
}

// LoadConfig reads the playback settings, with defaults for the unset ones.
// A malformed value is ignored rather than breaking playback.
func LoadConfig(ctx context.Context, s *settings.Store) (Config, error) {
	var c Config
	if s == nil {
		return c, nil
	}
	if v, ok, err := s.Get(ctx, keyRemoteMaxBitrate); err != nil {
		return c, err
	} else if ok {
		_ = json.Unmarshal([]byte(v), &c.RemoteMaxBitrate)
	}
	if v, ok, err := s.Get(ctx, keyLocalNetworks); err != nil {
		return c, err
	} else if ok {
		var nets []string
		if json.Unmarshal([]byte(v), &nets) == nil {
			for _, n := range nets {
				if p, err := netip.ParsePrefix(n); err == nil {
					c.LocalNetworks = append(c.LocalNetworks, p.Masked())
				}
			}
		}
	}
	return c, nil
}
