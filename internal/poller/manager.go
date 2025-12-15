// Package poller collects metrics from external sources (A2S/RCon).
package poller

import (
	"context"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/rs/zerolog/log"
	"github.com/woozymasta/metricz-exporter/internal/config"
	"github.com/woozymasta/metricz-exporter/internal/geoip"
	"github.com/woozymasta/metricz-exporter/internal/storage"
)

// Manager runs polling workers for configured servers.
type Manager struct {
	store *storage.Storage
	cfg   *config.Config
	geoDB *geoip2.Reader
}

// NewManager creates a new poller manager.
func NewManager(store *storage.Storage, cfg *config.Config) *Manager {
	var geoDB *geoip2.Reader

	if cfg.App.GeoIP.Path == "" {
		return &Manager{store: store, cfg: cfg, geoDB: nil}
	}

	if cfg.App.GeoIP.URL != "" {
		if err := geoip.EnsureDB(cfg.App.GeoIP); err != nil {
			log.Error().
				Err(err).
				Str("path", cfg.App.GeoIP.Path).
				Str("url", cfg.App.GeoIP.URL).
				Msg("failed to ensure GeoIP database (will try to open existing file if present)")
		}
	}

	db, err := geoip2.Open(cfg.App.GeoIP.Path)
	if err != nil {
		log.Error().Err(err).Str("path", cfg.App.GeoIP.Path).Msg("failed to open GeoIP database")
	} else {
		geoDB = db
		log.Info().Str("path", cfg.App.GeoIP.Path).Msg("GeoIP database loaded")
	}

	return &Manager{store: store, cfg: cfg, geoDB: geoDB}
}

// Close releases resources held by Manager.
func (m *Manager) Close() {
	if m.geoDB != nil {
		_ = m.geoDB.Close()
	}
}

// Start launches pollers until ctx is canceled.
func (m *Manager) Start(ctx context.Context) {
	for _, srv := range m.cfg.Servers {
		if srv.A2S != nil && srv.A2S.Address != "" {
			go m.runA2SWorker(ctx, srv)
		}

		if srv.RCon != nil && srv.RCon.Address != "" {
			go m.runRConWorker(ctx, srv)
		}
	}
}

func (m *Manager) runA2SWorker(ctx context.Context, srv config.ServerDefinition) {
	ticker := time.NewTicker(srv.A2S.PoolInterval.ToDuration())
	defer ticker.Stop()

	log.Info().
		Str("instance_id", srv.InstanceID).
		Str("address", srv.A2S.Address).
		Msg("starting A2S poller")

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			start := time.Now()
			info, err := pollA2S(srv.A2S)

			if err != nil {
				log.Warn().
					Err(err).
					Str("instance_id", srv.InstanceID).
					Dur("duration_ms", time.Since(start)).
					Msg("fail in A2S poll metrics collection")
			} else {
				log.Debug().
					Str("instance_id", srv.InstanceID).
					Dur("duration_ms", time.Since(start)).
					Str("server_name", info.Name).
					Int("players", int(info.Players)).
					Msg("metrics in A2S pool collected")
			}

			m.store.UpdatePolled(srv.InstanceID, setA2SMetrics(srv, info))
		}
	}
}

func (m *Manager) runRConWorker(ctx context.Context, srv config.ServerDefinition) {
	session := NewRConSession(srv, m.geoDB)
	defer session.Close()

	ticker := time.NewTicker(srv.RCon.PoolInterval.ToDuration())
	defer ticker.Stop()

	log.Info().
		Str("instance_id", srv.InstanceID).
		Str("address", srv.RCon.Address).
		Msg("starting RCon poller")

	// Initial poll
	m.pollRConAndSubmit(session, srv.InstanceID)

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			m.pollRConAndSubmit(session, srv.InstanceID)
		}
	}
}

func (m *Manager) pollRConAndSubmit(session *RConSession, instanceID string) {
	metrics, err := session.Poll()
	if err == nil {
		log.Debug().
			Str("instance_id", instanceID).
			Int("families", len(metrics)).
			Msg("metrics in RCon pool collected")
	}

	m.store.UpdateRCon(instanceID, metrics)
}
