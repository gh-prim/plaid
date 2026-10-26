package config

import (
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
