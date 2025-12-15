package poller

import (
	"fmt"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog/log"
	"github.com/woozymasta/bercon-cli/pkg/beparser"
	"github.com/woozymasta/bercon-cli/pkg/bercon"
	"github.com/woozymasta/metricz-exporter/internal/config"
)

// RConSession manages a persistent connection to one server.
type RConSession struct {
	lastActive time.Time
	conn       *bercon.Connection
	geoDB      *geoip2.Reader
	cfg        config.ServerDefinition
	mu         sync.Mutex
}

// NewRConSession creates an RCon session for a server.
func NewRConSession(cfg config.ServerDefinition, geoDB *geoip2.Reader) *RConSession {
	return &RConSession{
		cfg:   cfg,
		geoDB: geoDB,
	}
}

// Poll executes the "players" command with retry logic.
func (s *RConSession) Poll() (map[string]*dto.MetricFamily, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure connection
	if s.conn == nil || !s.conn.IsAlive() {
		log.Trace().Str("instance_id", s.cfg.InstanceID).Msg("inactive RCon connection, connecting...")
		if err := s.connect(); err != nil {
			return s.generateMetrics(nil), fmt.Errorf("initial connect failed: %w", err)
		}
	}

	// Try to send command
	command := "players"
	resp, err := s.conn.Send(command)
	if err != nil {
		log.Debug().
			Err(err).
			Str("command", command).
			Str("instance_id", s.cfg.InstanceID).
			Msg("failed execute RCon command, reconnecting...")

		// On failure: Force Reconnect
		s.closeInternal()
		if connErr := s.connect(); connErr != nil {
			log.Warn().
				Err(connErr).
				Str("command", command).
				Str("instance_id", s.cfg.InstanceID).
				Msg("reconnect to RCon failed")
			return s.generateMetrics(nil), connErr
		}

		// Retry command with new connection
		log.Trace().
			Str("command", command).
			Str("instance_id", s.cfg.InstanceID).
			Msg("retrying RCon command...")

		resp, err = s.conn.Send(command)
		if err != nil {
			log.Warn().
				Err(err).
				Str("command", command).
				Str("instance_id", s.cfg.InstanceID).
				Msg("retry RCon command failed")

			// Close connection to ensure fresh start next time
			s.closeInternal()
			return s.generateMetrics(nil), err
		}
	}

	s.lastActive = time.Now()
	log.Trace().
		Str("command", command).
		Str("instance_id", s.cfg.InstanceID).
		Int("response_bytes", len(resp)).
		Msg("success executed RCon command")

	// Parse
	players := beparser.NewPlayers()
	players.Parse(resp)
	if s.geoDB != nil {
		players.SetGeo(s.geoDB)
	}

	return s.generateMetrics(players), nil
}

func (s *RConSession) connect() error {
	s.closeInternal() // Safety cleanup

	start := time.Now()
	conn, err := bercon.Open(s.cfg.RCon.Address, s.cfg.RCon.Password)
	if err != nil {
		return err
	}

	// Apply config
	conn.SetKeepalive(s.cfg.RCon.KeepaliveTimeout.ToDuration())
	conn.SetDeadline(s.cfg.RCon.DeadlineTimeout.ToDuration())
	conn.SetBufferSize(s.cfg.RCon.BufferSize)

	conn.StartKeepAlive()
	s.conn = conn

	log.Debug().
		Str("instance_id", s.cfg.InstanceID).
		Dur("deadline", s.cfg.RCon.DeadlineTimeout.ToDuration()).
		Dur("keepalive", s.cfg.RCon.KeepaliveTimeout.ToDuration()).
		Dur("duration_ms", time.Since(start)).
		Msg("successfully connected to RCon")

	return nil
}

// Close closes the session safely.
func (s *RConSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeInternal()
}

// closeInternal closes connection without locking.
func (s *RConSession) closeInternal() {
	if s.conn != nil {
		log.Trace().
			Str("instance_id", s.cfg.InstanceID).
			Msg("closing RCon connection")

		_ = s.conn.Close()
		s.conn = nil
	}
}

// generateMetrics converts parsed players to Prometheus metrics.
// If players == nil, it assumes the server is down (up=0).
func (s *RConSession) generateMetrics(players *beparser.Players) map[string]*dto.MetricFamily {
	families := make(map[string]*dto.MetricFamily)
	var up, total, inLobby float64

	if players != nil {
		up = 1
		total = float64(len(*players))

		for _, p := range *players {
			var joined float64
			if p.Lobby {
				inLobby++
			} else {
				joined = 1
			}

			extraLabels := map[string]string{
				"instance_id": s.cfg.InstanceID,
				"buid":        p.GUID,
				"name":        p.Name,
				"ip":          p.IP,
			}

			if p.Country != "" {
				extraLabels["country"] = p.Country
			}
			if p.City != "" {
				extraLabels["city"] = p.City
			}

			addGaugeWithLabels(
				families,
				"metricz_rcon_player_joined",
				"Player joined to server (0=lobby, loading or in queue. 1=playing).",
				joined,
				extraLabels)

			labels := map[string]string{
				"instance_id": s.cfg.InstanceID,
				"buid":        p.GUID,
			}

			if p.Latitude != 0 && p.Longitude != 0 {
				addGaugeWithLabels(
					families,
					"metricz_rcon_player_lat",
					"Player Latitude.",
					p.Latitude,
					labels)

				addGaugeWithLabels(
					families,
					"metricz_rcon_player_lon",
					"Player Longitude.",
					p.Longitude,
					labels)
			}

			addGaugeWithLabels(
				families,
				"metricz_rcon_player_ping_seconds",
				"Player latency.",
				float64(p.Ping)/1000.0,
				labels)
		}
	}

	addGauge(
		families,
		"metricz_rcon_up",
		"RCon availability.",
		up,
		s.cfg.InstanceID)

	addGauge(
		families,
		"metricz_rcon_players_total",
		"Total clients connected (including lobby).",
		total,
		s.cfg.InstanceID)

	addGauge(
		families,
		"metricz_rcon_players_lobby",
		"Players in lobby.",
		inLobby,
		s.cfg.InstanceID)

	return families
}
