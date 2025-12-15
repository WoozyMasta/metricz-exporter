// Package entrypoint contains the main application bootstrap logic.
package entrypoint

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"

	"github.com/woozymasta/metricz-exporter/internal/config"
	"github.com/woozymasta/metricz-exporter/internal/poller"
	"github.com/woozymasta/metricz-exporter/internal/server"
	"github.com/woozymasta/metricz-exporter/internal/storage"
)

// Execute boots the application and returns an exit code.
func Execute() int {
	// Parse command line flags
	cliCfg := config.ParseFlags()

	// load config
	cfg, err := config.LoadConfig(cliCfg.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load server config: %v", err)
		return 2
	}

	log.Info().
		Str("path", cliCfg.ConfigPath).
		Int("servers_count", len(cfg.Servers)).
		Msg("configuration loaded")

	// Initialize dependencies
	store := storage.New()
	exporter := storage.NewExporter(store, cfg.App.Stale)
	apiHandler := server.NewHandler(store, cfg)
	pollerMgr := poller.NewManager(store, cfg)

	// Start Staging Garbage Collector
	go store.StartGarbageCollector(context.Background(), cfg.App.Ingest.GarbageCollectorTTL.ToDuration())

	// A2S/RCon poller
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pollerMgr.Start(ctx)

	// Registry (implements both Registerer and Gatherer)
	registry := prometheus.NewRegistry()
	var reg prometheus.Registerer = registry

	// Wrapper adds extra labels on all collected metrics
	if len(cfg.App.Prometheus.ExtraLabels) != 0 {
		reg = prometheus.WrapRegistererWith(prometheus.Labels(cfg.App.Prometheus.ExtraLabels), registry)
	}

	// Enable built in collectors
	if !cfg.App.Prometheus.DisableGoCollector {
		reg.MustRegister(collectors.NewGoCollector())
	}
	if !cfg.App.Prometheus.DisableProcessCollector {
		reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}
	reg.MustRegister(exporter)

	// Initialize Router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)

	// HTTP logger
	r.Use(hlog.NewHandler(log.Logger))
	r.Use(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		logger := hlog.FromRequest(r)
		var event *zerolog.Event

		// Log Health checks at DEBUG level, others at INFO
		if status == 200 &&
			strings.HasPrefix(r.URL.Path, "/health") ||
			strings.HasPrefix(r.URL.Path, "/metrics") {
			event = logger.Debug()
		} else {
			event = logger.Info()
		}

		event.
			Str("method", r.Method).
			Stringer("url", r.URL).
			Int("status", status).
			Int("resp_bytes", size).
			Int64("req_bytes", r.ContentLength).
			Dur("duration_ms", duration).
			Msg("HTTP request")
	}))
	r.Use(hlog.UserAgentHandler("user_agent"))
	r.Use(hlog.RemoteAddrHandler("ip"))

	// RegisterUI registers the web interface routes.
	apiHandler.RegisterUI(r)

	// Public Routes (no auth, root level health)
	apiHandler.RegisterHealthRoutes(r)

	// API Routes
	r.Route("/api/v1", func(r chi.Router) {
		apiHandler.RegisterPublicRoutes(r)
		r.Group(apiHandler.RegisterPrivateRoutes)
	})

	// Prometheus Endpoint
	r.With(apiHandler.BasicAuthMiddleware).
		Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	log.Info().
		Str("address", cfg.App.ListenAddr).
		Int64("max_body_size", cfg.App.Ingest.MaxBodySize).
		Dur("cache_ttl", cfg.App.Public.PublicCacheTTL.ToDuration()).
		Dur("transaction_ttl", cfg.App.Ingest.TransactionTTL.ToDuration()).
		Dur("gc_ttl", cfg.App.Ingest.GarbageCollectorTTL.ToDuration()).
		Dur("min_stale_age", cfg.App.Stale.MinStaleAge.ToDuration()).
		Float64("stale_multiplier", cfg.App.Stale.StaleMultiplier).
		Bool("auth_enabled", cfg.App.Auth.User != "" && cfg.App.Auth.Pass != "").
		Msg("starting metricz-exporter")

	srv := &http.Server{
		Addr:              cfg.App.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("server failed")
		return 3
	}

	return 0
}
