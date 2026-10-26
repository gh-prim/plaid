// Package config loads the server configuration from environment variables.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// Config is the runtime configuration of the server.
type Config struct {
	Host      string
	Port      int
	DataDir   string
	LogLevel  string
	LogFormat string
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

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return Config{}, fmt.Errorf("create data dir %s: %w", cfg.DataDir, err)
	}
	return cfg, nil
}
