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

	"github.com/prometheus/client_golang/prometheus"

	"github.com/vodafone/vois-speechmark-demo/internal/config"
	"github.com/vodafone/vois-speechmark-demo/internal/httpapi"
	"github.com/vodafone/vois-speechmark-demo/internal/observability"
	"github.com/vodafone/vois-speechmark-demo/internal/store"
	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
	"github.com/vodafone/vois-speechmark-demo/internal/webhook"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Config (env-based; planted hardcoded-secret fallbacks live in internal/config).
	cfg := config.Load()

	// 2. Logger (slog JSON to stdout).
	logger := observability.InitLogger(cfg.LogLevel)
	logger.Info("starting vois-speechmark-demo",
		"addr", cfg.Addr,
		"store", cfg.Store,
		"otlp_endpoint", cfg.OTLPEndpoint,
	)

	// 3. Tracer (+ deferred shutdown of the tracer provider).
	//    Empty OTLP endpoint => stdout exporter (default demo mode).
	rootCtx := context.Background()
	tracer, tracerShutdown, err := observability.InitTracer(rootCtx, cfg.OTLPEndpoint)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shErr := tracerShutdown(shutdownCtx); shErr != nil {
			logger.Error("tracer shutdown failed", "error", shErr)
		}
	}()

	// 4. Metrics (own Prometheus registry; exposed at GET /metrics via the router).
	metrics := observability.NewMetrics(prometheus.NewRegistry())

	// 5. Store — memory (default) or sqlite, selected by cfg.Store.
	st, err := newStore(cfg)
	if err != nil {
		return err
	}

	// 6. Domain service.
	svc := subscriber.NewService(st)

	// 7. Outbound webhook client + simulated carrier downstream.
	webhookClient := webhook.New()
	carrier := httpapi.NewCarrierClient(cfg.CarrierFailureRate)

	// 8. Router — the single place routes/middleware are wired (TG-HTTPCORE owns NewRouter).
	router := httpapi.NewRouter(httpapi.Deps{
		Svc:     svc,
		Store:   st,
		Metrics: metrics,
		Tracer:  tracer,
		Logger:  logger,
		Cfg:     cfg,
		Webhook: webhookClient,
		Carrier: carrier,
	})

	// 9. HTTP server + graceful shutdown on SIGINT/SIGTERM.
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(rootCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.Addr)
		if lErr := srv.ListenAndServe(); lErr != nil && !errors.Is(lErr, http.ErrServerClosed) {
			serveErr <- lErr
			return
		}
		serveErr <- nil
	}()

	select {
	case lErr := <-serveErr:
		return lErr
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if sErr := srv.Shutdown(shutdownCtx); sErr != nil {
			logger.Error("graceful shutdown failed", "error", sErr)
			return sErr
		}
		logger.Info("server stopped cleanly")
		return nil
	}
}

// newStore selects the concrete Store implementation by cfg.Store.
// Both concrete types structurally satisfy subscriber.Store.
func newStore(cfg config.Config) (subscriber.Store, error) {
	switch cfg.Store {
	case "sqlite":
		return store.NewSQLiteStore(cfg.SQLiteDSN)
	case "memory", "":
		return store.NewMemoryStore(), nil
	default:
		return nil, errors.New("unknown STORE value: " + cfg.Store + ` (want "memory" or "sqlite")`)
	}
}
