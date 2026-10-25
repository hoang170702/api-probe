// Command caserunner is the composition root: it reads the configuration, wires
// the adapters and use cases together and serves the HTTP API (BRD-001 §14.1).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/broker"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/httpapi"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/memstore"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/sender"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/system"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/adapter/web"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/config"
	"github.com/hoang170702/api-probe/caserunner/backend/internal/usecase"
)

// version is set at build time: go build -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("caserunner stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if len(cfg.AllowedHosts) == 0 {
		slog.Warn("CASERUNNER_ALLOWED_HOSTS is empty: requests may target any host; use this only locally")
	}

	policy := usecase.Policy{
		AllowedHosts:   cfg.AllowedHosts,
		AllowInsecure:  cfg.AllowInsecure,
		DefaultTimeout: cfg.DefaultTimeout,
		MaxTimeout:     cfg.MaxTimeout,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	out := sender.New() // one connection pool for every outgoing request (§11.2)
	store := memstore.New(cfg.MaxRuns, cfg.RunTTL, time.Now)
	store.StartJanitor(ctx, time.Minute)
	useCases := httpapi.UseCases{
		ParseCurl:  usecase.ParseCurl{},
		TryPrepare: usecase.TryPrepare{Sender: out, Policy: policy},
		Runs: &usecase.Runs{
			Repo: store, Events: broker.New(), Sender: out, Policy: policy,
			Clock: system.Clock{}, IDs: system.IDs{},
			MaxCases: cfg.MaxCases, MaxConcurrency: cfg.MaxConcurrency, DefaultConcurrency: cfg.DefaultConcurrency,
			Base: ctx, // runs stop on shutdown
		},
	}

	// Bind before logging, so "listening" is only printed once the port is ours.
	ln, err := net.Listen("tcp", cfg.Bind)
	if err != nil {
		return err
	}
	// On a loopback bind only loopback Host names are accepted (DNS rebinding,
	// BRD-001 §13); a network bind is opened by names this process cannot know.
	hosts := httpapi.LoopbackHosts(ln.Addr())
	if hosts == nil {
		slog.Warn("listening beyond this machine: anyone who can reach " + ln.Addr().String() + " can send requests through this tool")
	}
	srv := &http.Server{
		Handler:           httpapi.Guard(httpapi.NewHandler(httpapi.Info{Version: version}, useCases, web.Handler()), hosts),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// No WriteTimeout: SSE streams stay open for the whole run.
	}
	slog.Info("caserunner listening", "url", "http://"+ln.Addr().String(), "version", version)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
