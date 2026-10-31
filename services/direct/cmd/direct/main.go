// Command direct runs getplaid.direct: the registration API, short links and
// the authoritative DNS server that give PLAID servers a stable hostname.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gh-prim/plaid/services/direct/internal/api"
	"github.com/gh-prim/plaid/services/direct/internal/config"
	"github.com/gh-prim/plaid/services/direct/internal/dnsserver"
	"github.com/gh-prim/plaid/services/direct/internal/probe"
	"github.com/gh-prim/plaid/services/direct/internal/store"
)

const (
	// staleAfter is how long a server may go without a heartbeat before its
	// names are removed.
	staleAfter     = 30 * 24 * time.Hour
	janitorEvery   = 10 * time.Minute
	shutdownBudget = 10 * time.Second
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "query /healthz on DIRECT_HTTP_ADDR and exit 0 if healthy (for container health checks)")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(os.Getenv)
	if *healthcheck {
		os.Exit(runHealthcheck(cfg.HTTPAddr))
	}
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(2)
	}
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	httpSrv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: api.New(api.Config{
			Store:          st,
			Prober:         &probe.Prober{},
			Zone:           cfg.Zone,
			SiteURL:        cfg.SiteURL,
			TrustedProxies: cfg.TrustedProxies,
			Logger:         log,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second, // the reachability probe takes up to 5 s
		IdleTimeout:       60 * time.Second,
	}
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("http listen: %w", err)
	}
	errc := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(l); !errors.Is(err, http.ErrServerClosed) {
			errc <- fmt.Errorf("http: %w", err)
		}
	}()
	log.Info("http listening", "addr", l.Addr().String(), "zone", cfg.Zone)

	var dnsSrv *dnsserver.Server
	if cfg.DNSEnabled() {
		dnsSrv = dnsserver.New(dnsserver.Config{
			Zone:       cfg.Zone,
			IPv4:       cfg.PublicIP,
			IPv6:       cfg.PublicIPv6,
			AdminEmail: cfg.AdminEmail,
			Records:    st,
			Logger:     log,
		})
		udp, _, err := dnsSrv.Start(cfg.DNSAddr)
		if err != nil {
			_ = httpSrv.Close()
			return err
		}
		log.Info("dns listening", "addr", udp.String(), "publicIp", cfg.PublicIP.String())
	} else {
		log.Warn("dns disabled (DIRECT_DNS_ADDR=off)")
	}

	go janitor(ctx, st, log)

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-errc:
		return err
	}
	sctx, cancel := context.WithTimeout(context.Background(), shutdownBudget)
	defer cancel()
	var errs []error
	if err := httpSrv.Shutdown(sctx); err != nil {
		errs = append(errs, fmt.Errorf("http shutdown: %w", err))
	}
	if dnsSrv != nil {
		if err := dnsSrv.Shutdown(sctx); err != nil {
			errs = append(errs, fmt.Errorf("dns shutdown: %w", err))
		}
	}
	return errors.Join(errs...)
}

// janitor removes servers unseen for 30 days and expired ACME values. DNS
// already ignores expired values; this only keeps the tables small.
func janitor(ctx context.Context, st *store.Store, log *slog.Logger) {
	t := time.NewTicker(janitorEvery)
	defer t.Stop()
	for {
		now := time.Now()
		servers, values, err := st.Purge(ctx, now.Add(-staleAfter), now)
		switch {
		case err != nil && ctx.Err() == nil:
			log.Error("janitor failed", "err", err)
		case servers > 0 || values > 0:
			log.Info("janitor", "staleServers", servers, "expiredAcmeValues", values)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func runHealthcheck(addr string) int {
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/healthz", nil)
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, resp.Status)
		return 1
	}
	return 0
}
