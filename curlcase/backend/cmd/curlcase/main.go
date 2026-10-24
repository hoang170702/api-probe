// Command curlcase is the composition root: it reads the configuration, wires
// the adapters and use cases together and serves the HTTP API (BRD-001 §14.1).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hoang170702/api-probe/curlcase/backend/internal/adapter/httpapi"
	"github.com/hoang170702/api-probe/curlcase/backend/internal/config"
)

// version is set at build time: go build -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("curlcase stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if len(cfg.AllowedHosts) == 0 {
		slog.Warn("CURLCASE_ALLOWED_HOSTS is empty: requests may target any host; use this only locally")
	}

	srv := &http.Server{
		Addr:              cfg.Bind,
		Handler:           httpapi.NewHandler(httpapi.Info{Version: version}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// No WriteTimeout: SSE streams stay open for the whole run.
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		slog.Info("curlcase listening", "addr", cfg.Bind, "version", version)
		errc <- srv.ListenAndServe()
	}()

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
