// Package config loads the server configuration from environment variables.
package config

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the runtime configuration of the server.
type Config struct {
	Host      string
	Port      int
	DataDir   string
	LogLevel  string
	LogFormat string
	// TrustedProxies are the peers whose X-Forwarded-* headers are believed.
	TrustedProxies []netip.Prefix
	// ServerName is shown to clients by GET /server. Defaults to the hostname.
	ServerName string
	// FFprobePath overrides the ffprobe found on PATH (F004-R25).
	FFprobePath string
	// FFmpegPath overrides the ffmpeg found on PATH (ADR 0006, F006).
	FFmpegPath string
	// TranscodeDir holds HLS segments while sessions play (F006 R27).
	// Defaults to $PLAID_DATA_DIR/transcodes.
	TranscodeDir string
	// BrowseRoots restricts the library folder picker (F004-R3); empty
	// means the whole file system.
	BrowseRoots []string
	// TMDBAPIKey is the admin's TMDB key (F005 R34, ADR 0007); empty means
	// the key saved in settings or the one built into release binaries.
	TMDBAPIKey string
	// FanartAPIKey is the admin's fanart.tv project key (F005 R34).
	FanartAPIKey string
	// ArtworkCacheBytes caps resized artwork (F005 R20); 0 = 2 GiB.
	ArtworkCacheBytes int64
}

// Addr returns the host:port the HTTP server listens on.
func (c Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// Load reads the configuration through getenv (os.Getenv in production),
// applies defaults, validates values and creates the data directory.
func Load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Host:      get("PLAID_HOST", "0.0.0.0"),
		DataDir:   get("PLAID_DATA_DIR", "./data"),
		LogLevel:  get("PLAID_LOG_LEVEL", "info"),
		LogFormat: get("PLAID_LOG_FORMAT", "text"),
	}
	cfg.ServerName = getenv("PLAID_SERVER_NAME")
	if cfg.ServerName == "" {
		cfg.ServerName, _ = os.Hostname()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = "PLAID"
	}

	port, err := strconv.Atoi(get("PLAID_PORT", "32500"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PLAID_PORT must be a number between 1 and 65535, got %q", getenv("PLAID_PORT"))
	}
	cfg.Port = port

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("PLAID_LOG_LEVEL must be debug, info, warn or error, got %q", cfg.LogLevel)
	}
	switch cfg.LogFormat {
	case "text", "json":
	default:
		return Config{}, fmt.Errorf("PLAID_LOG_FORMAT must be text or json, got %q", cfg.LogFormat)
	}

	proxies, err := parsePrefixes(getenv("PLAID_TRUSTED_PROXIES"))
	if err != nil {
		return Config{}, fmt.Errorf("PLAID_TRUSTED_PROXIES: %w", err)
	}
	cfg.TrustedProxies = proxies

	cfg.FFprobePath = getenv("PLAID_FFPROBE_PATH")
	cfg.FFmpegPath = getenv("PLAID_FFMPEG_PATH")
	cfg.TranscodeDir = getenv("PLAID_TRANSCODE_DIR")
	cfg.TMDBAPIKey = strings.TrimSpace(getenv("PLAID_TMDB_API_KEY"))
	cfg.FanartAPIKey = strings.TrimSpace(getenv("PLAID_FANART_API_KEY"))
	if v := getenv("PLAID_ARTWORK_CACHE_MAX_MB"); v != "" {
		mb, err := strconv.Atoi(v)
		if err != nil || mb < 64 {
			return Config{}, fmt.Errorf("PLAID_ARTWORK_CACHE_MAX_MB must be a number of megabytes, at least 64, got %q", v)
		}
		cfg.ArtworkCacheBytes = int64(mb) << 20
	}
	roots, err := parseRoots(getenv("PLAID_BROWSE_ROOTS"))
	if err != nil {
		return Config{}, fmt.Errorf("PLAID_BROWSE_ROOTS: %w", err)
	}
	cfg.BrowseRoots = roots

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return Config{}, fmt.Errorf("create data dir %s: %w", cfg.DataDir, err)
	}
	if cfg.TranscodeDir == "" {
		cfg.TranscodeDir = filepath.Join(cfg.DataDir, "transcodes")
	}
	if !filepath.IsAbs(cfg.TranscodeDir) {
		abs, err := filepath.Abs(cfg.TranscodeDir)
		if err != nil {
			return Config{}, fmt.Errorf("PLAID_TRANSCODE_DIR: %w", err)
		}
		cfg.TranscodeDir = abs
	}
	if err := os.MkdirAll(cfg.TranscodeDir, 0o750); err != nil {
		return Config{}, fmt.Errorf("create transcode dir %s: %w", cfg.TranscodeDir, err)
	}
	return cfg, nil
}

// parseRoots reads a colon-separated list of absolute paths.
func parseRoots(list string) ([]string, error) {
	var out []string
	for _, item := range strings.Split(list, ":") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !filepath.IsAbs(item) {
			return nil, fmt.Errorf("%q is not an absolute path", item)
		}
		out = append(out, filepath.Clean(item))
	}
	return out, nil
}

// parsePrefixes reads a comma-separated list of CIDRs or bare IP addresses.
func parsePrefixes(list string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if p, err := netip.ParsePrefix(item); err == nil {
			out = append(out, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(item)
		if err != nil {
			return nil, fmt.Errorf("%q is neither an IP address nor a CIDR", item)
		}
		addr = addr.Unmap()
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return out, nil
}
