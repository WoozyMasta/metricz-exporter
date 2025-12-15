package poller

import (
	"fmt"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog/log"
	"github.com/woozymasta/a2s/pkg/a2s"
	"github.com/woozymasta/a2s/pkg/keywords"
	"github.com/woozymasta/metricz-exporter/internal/config"
)

func pollA2S(cfg *config.A2SConfig) (*a2s.Info, error) {
	log.Trace().
		Str("address", cfg.Address).
		Msg("dialing A2S")

	client, err := a2s.NewWithString(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	client.Timeout = cfg.DeadlineTimeout.ToDuration()
	client.BufferSize = cfg.BufferSize

	info, err := client.GetInfo()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return info, nil
}

// setA2SMetrics returns a metric set containing 1 = up, 0 = down
func setA2SMetrics(srv config.ServerDefinition, info *a2s.Info) map[string]*dto.MetricFamily {
	families := make(map[string]*dto.MetricFamily)

	var state, ping, players, slots, queue float64
	if info != nil {
		state = 1
		ping = info.Ping.Seconds()
		players = float64(info.Players)
		slots = float64(info.MaxPlayers)

		dayzInfo := keywords.ParseDayZ(info.Keywords)
		if dayzInfo != nil {
			queue = float64(dayzInfo.PlayersQueue)
		}

		metaLabels := map[string]string{
			"instance_id":        srv.InstanceID,
			"server_name":        info.Name,
			"server_description": info.Game,
			"world":              info.Map,
			"version":            info.Version,
			"query_address":      srv.A2S.Address,
			"game_address":       fmt.Sprintf("%s:%d", strings.Split(srv.A2S.Address, ":")[0], info.Port),
			"environment":        info.Environment.String(),
		}

		addGaugeWithLabels(
			families,
			"metricz_a2s_info",
			"Static metadata about the game server.",
			1,
			metaLabels,
		)
	}

	addGauge(
		families,
		"metricz_a2s_up",
		"A2S server availability (1 = up, 0 = down).",
		state,
		srv.InstanceID)

	addGauge(
		families,
		"metricz_a2s_info_response_time_seconds",
		"Server A2S_INFO response time.",
		ping,
		srv.InstanceID)

	addGauge(
		families,
		"metricz_a2s_info_players_online",
		"Online players.",
		players,
		srv.InstanceID)

	addGauge(
		families,
		"metricz_a2s_info_players_slots",
		"Players slots count.",
		slots,
		srv.InstanceID)

	addGauge(
		families,
		"metricz_a2s_info_players_queue",
		"Players wait in queue.",
		queue,
		srv.InstanceID)

	return families
}
