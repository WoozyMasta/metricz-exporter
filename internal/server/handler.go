// Package server implements HTTP handlers and middleware.
package server

import (
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/woozymasta/metricz-exporter/internal/config"
	"github.com/woozymasta/metricz-exporter/internal/storage"
)

// Handler manages the HTTP API endpoints.
type Handler struct {
	store       *storage.Storage
	cfg         *config.Config
	publicCache sync.Map
}

type publicCacheItem struct {
	expiresAt time.Time
	response  []byte
}

// NewHandler creates a new API handler with dependencies.
func NewHandler(store *storage.Storage, cfg *config.Config) *Handler {
	return &Handler{
		store: store,
		cfg:   cfg,
	}
}

// RegisterPrivateRoutes registers authenticated ingest endpoints under /api/v1.
func (h *Handler) RegisterPrivateRoutes(r chi.Router) {
	// Apply Basic Auth to this group
	r.Use(h.BasicAuthMiddleware)
	r.Use(h.JSONTranslatorMiddleware)

	// Single-shot upload (entire payload in one request)
	r.Post("/ingest/{instance_id}", h.handleSingleShot)

	// Chunked upload (transaction-based)
	r.Post("/ingest/{instance_id}/{txn_hash}/{seq_id}", h.handleChunkIngest)
	r.Post("/commit/{instance_id}/{txn_hash}", h.handleCommit)
}

// RegisterUI registers the web interface routes.
func (h *Handler) RegisterUI(r chi.Router) {
	r.Get("/", h.HandleIndex)
}

// RegisterPublicRoutes registers endpoints that do not require authentication.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	// Public JSON metrics
	r.Get("/status", h.HandleStatusFull)
	r.Get("/status/{instance_id}", h.HandleStatusSingle)
}

// RegisterHealthRoutes registers health endpoints that do not require authentication.
func (h *Handler) RegisterHealthRoutes(r chi.Router) {
	// Liveness probes
	r.Get("/health", h.HandleLiveness)
	r.Get("/health/liveness", h.HandleLiveness)
	r.Get("/health/readiness", h.HandleReadiness)
}
