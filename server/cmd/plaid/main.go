// Command plaid runs the PLAID media server.
package main

import (
	"context"
	"database/sql"
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
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
	"github.com/gh-prim/plaid/server/internal/settings"
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
	db.CheckPermissions(cfg.DataDir, logger)

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

	// The metadata service starts before the library engine so the
	// engine's first scans already feed it (F005 R1).
	meta := metadata.New(metadata.Options{
		DB: database, Settings: settings.New(database), TMDBKey: cfg.TMDBAPIKey, Log: logger,
	})
	meta.Start(ctx)
	defer meta.Close()

	engine := newLibraryEngine(ctx, cfg, database, logger, library.Hooks{ItemsCreated: meta.ItemsCreated})
	defer engine.Close()

	// Cancelled by srv.Shutdown so open event streams don't hold it up.
	streams, endStreams := context.WithCancel(context.Background())
	defer endStreams()
	handler := httpserver.New(handlers.New(handlers.Deps{
		DB:         database,
		Accounts:   accts,
		ServerName: cfg.ServerName,
		Library:    engine,
		Metadata:   meta,
		Streams:    streams,
	}), webui.FS(), logger, httpserver.Options{
		TrustedProxies: cfg.TrustedProxies,
		Sessions:       accts,
	})

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Addr())
	if err != nil {
		return err
	}
	logger.Info("listening", "addr", ln.Addr().String())
	return serve(ctx, ln, handler, logger, endStreams)
}

// newLibraryEngine finds ffprobe and starts the library engine in the
// background (F004-R25, NFR startup). A missing ffprobe is a warning:
// files are then listed without analysis.
func newLibraryEngine(ctx context.Context, cfg config.Config, database *sql.DB, logger *slog.Logger, hooks library.Hooks) *library.Engine {
	opts := library.EngineOptions{BrowseRoots: cfg.BrowseRoots, Log: logger, Hooks: hooks}
	probe, err := library.FindFFprobe(ctx, cfg.FFprobePath, logger)
	if err != nil {
		logger.Warn("ffprobe not found: media files are listed but not analyzed; install ffmpeg or set PLAID_FFPROBE_PATH",
			"code", "ffprobe_missing", "err", err)
	} else {
		logger.Info("ffprobe found", "path", probe.Path, "version", probe.Version)
		opts.Prober, opts.FFprobeVersion = probe, probe.Version
	}
	engine := library.NewEngine(database, opts)
	engine.Start(ctx)
	return engine
}

// serve runs the HTTP server until ctx is cancelled, then gives in-flight
// requests up to shutdownTimeout to finish. onShutdown runs as shutdown
// begins, to end long-lived responses.
func serve(ctx context.Context, ln net.Listener, handler http.Handler, logger *slog.Logger, onShutdown func()) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if onShutdown != nil {
		srv.RegisterOnShutdown(onShutdown)
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
