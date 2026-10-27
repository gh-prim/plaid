// Command plaid runs the PLAID media server.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/buildinfo"
	"github.com/gh-prim/plaid/server/internal/config"
	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/handlers"
	"github.com/gh-prim/plaid/server/internal/httpserver"
	"github.com/gh-prim/plaid/server/internal/webui"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "plaid:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, logOut io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	logger := newLogger(cfg, logOut)
	logger.Info("starting PLAID", "version", buildinfo.Version, "commit", buildinfo.Commit, "data_dir", cfg.DataDir)

	database, err := db.Open(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer database.Close()

	accts := accounts.New(database, accounts.Options{Logger: logger})
	code, err := accts.PrepareSetupCode(ctx)
	if err != nil {
		return err
	}
	if code != "" {
		// The one secret meant for the log: it lets the owner finish setup
		// from outside the local network (F003-R3, docs/install.md).
		logger.Warn("no account yet: open PLAID in a browser to create the admin account",
			"first_run_setup_code", code)
	}
	go accts.RunJanitor(ctx)

	handler := httpserver.New(handlers.New(handlers.Deps{
		DB:         database,
		Accounts:   accts,
		ServerName: cfg.ServerName,
	}), webui.FS(), logger, httpserver.Options{
		TrustedProxies: cfg.TrustedProxies,
		Sessions:       accts,
	})

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Addr())
	if err != nil {
		return err
	}
	logger.Info("listening", "addr", ln.Addr().String())
	return serve(ctx, ln, handler, logger)
}

// serve runs the HTTP server until ctx is cancelled, then gives in-flight
// requests up to shutdownTimeout to finish.
func serve(ctx context.Context, ln net.Listener, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errs := make(chan error, 1)
	go func() { errs <- srv.Serve(ln) }()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errs; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newLogger(cfg config.Config, w io.Writer) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel)) // validated by config.Load
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(w, opts))
	}
	return slog.New(slog.NewTextHandler(w, opts))
}
