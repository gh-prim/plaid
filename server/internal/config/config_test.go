package config

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	cfg, err := Load(env(map[string]string{"PLAID_DATA_DIR": dir}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 32500 {
		t.Errorf("Port = %d, want 32500", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want 0.0.0.0", cfg.Host)
	}
	if cfg.LogLevel != "info" || cfg.LogFormat != "text" {
		t.Errorf("log = %s/%s, want info/text", cfg.LogLevel, cfg.LogFormat)
	}
	if cfg.Addr() != "0.0.0.0:32500" {
		t.Errorf("Addr = %q", cfg.Addr())
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Errorf("data dir not created: %v", err)
	}
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(env(map[string]string{
		"PLAID_PORT":       "8080",
		"PLAID_HOST":       "127.0.0.1",
		"PLAID_DATA_DIR":   dir,
		"PLAID_LOG_LEVEL":  "debug",
		"PLAID_LOG_FORMAT": "json",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 || cfg.Host != "127.0.0.1" || cfg.DataDir != dir ||
		cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadInvalid(t *testing.T) {
	cases := map[string]map[string]string{
		"port not a number": {"PLAID_PORT": "abc"},
		"port out of range": {"PLAID_PORT": "70000"},
		"bad log level":     {"PLAID_LOG_LEVEL": "loud"},
		"bad log format":    {"PLAID_LOG_FORMAT": "xml"},
		"bad proxy":         {"PLAID_TRUSTED_PROXIES": "10.0.0.0/8, nope"},
	}
	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			vars["PLAID_DATA_DIR"] = t.TempDir()
			if _, err := Load(env(vars)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestTrustedProxies(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"PLAID_DATA_DIR":        t.TempDir(),
		"PLAID_TRUSTED_PROXIES": " 172.18.0.0/16, 127.0.0.1,::1 ,10.1.2.3/8",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("172.18.0.0/16"),
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("10.0.0.0/8"),
	}
	if len(cfg.TrustedProxies) != len(want) {
		t.Fatalf("TrustedProxies = %v, want %v", cfg.TrustedProxies, want)
	}
	for i := range want {
		if cfg.TrustedProxies[i] != want[i] {
			t.Errorf("TrustedProxies[%d] = %v, want %v", i, cfg.TrustedProxies[i], want[i])
		}
	}
}

func TestNoTrustedProxiesByDefault(t *testing.T) {
	cfg, err := Load(env(map[string]string{"PLAID_DATA_DIR": t.TempDir()}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want none", cfg.TrustedProxies)
	}
}

func TestServerName(t *testing.T) {
	cfg, err := Load(env(map[string]string{"PLAID_DATA_DIR": t.TempDir(), "PLAID_SERVER_NAME": "Living room"}))
	if err != nil || cfg.ServerName != "Living room" {
		t.Errorf("ServerName = %q (%v), want Living room", cfg.ServerName, err)
	}
	cfg, err = Load(env(map[string]string{"PLAID_DATA_DIR": t.TempDir()}))
	if err != nil || cfg.ServerName == "" {
		t.Errorf("default ServerName = %q (%v), want the hostname", cfg.ServerName, err)
	}
}

func TestLoadLibraryOptions(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"PLAID_DATA_DIR":             t.TempDir(),
		"PLAID_FFMPEG_PATH":          "/opt/ffmpeg/bin/ffmpeg",
		"PLAID_FFPROBE_PATH":         "/opt/ffmpeg/bin/ffprobe",
		"PLAID_BROWSE_ROOTS":         "/srv/media: /mnt/usb/ ",
		"PLAID_TMDB_API_KEY":         " abc123 ",
		"PLAID_FANART_API_KEY":       "fan",
		"PLAID_ARTWORK_CACHE_MAX_MB": "512",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TMDBAPIKey != "abc123" || cfg.FanartAPIKey != "fan" || cfg.ArtworkCacheBytes != 512<<20 {
		t.Errorf("metadata options = %q %q %d", cfg.TMDBAPIKey, cfg.FanartAPIKey, cfg.ArtworkCacheBytes)
	}
	for _, bad := range []string{"lots", "10"} {
		if _, err := Load(env(map[string]string{"PLAID_DATA_DIR": t.TempDir(), "PLAID_ARTWORK_CACHE_MAX_MB": bad})); err == nil {
			t.Errorf("PLAID_ARTWORK_CACHE_MAX_MB=%s accepted", bad)
		}
	}
	if cfg.FFprobePath != "/opt/ffmpeg/bin/ffprobe" || cfg.FFmpegPath != "/opt/ffmpeg/bin/ffmpeg" {
		t.Errorf("FFprobePath = %q, FFmpegPath = %q", cfg.FFprobePath, cfg.FFmpegPath)
	}
	if len(cfg.BrowseRoots) != 2 || cfg.BrowseRoots[0] != "/srv/media" || cfg.BrowseRoots[1] != "/mnt/usb" {
		t.Errorf("BrowseRoots = %q", cfg.BrowseRoots)
	}
	if _, err := Load(env(map[string]string{"PLAID_DATA_DIR": t.TempDir(), "PLAID_BROWSE_ROOTS": "media"})); err == nil {
		t.Error("relative browse root accepted")
	}
}
