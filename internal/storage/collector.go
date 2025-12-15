package storage

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog/log"
	"github.com/woozymasta/metricz-exporter/internal/config"
)

// Exporter implements prometheus.Collector.
type Exporter struct {
	store             *Storage
	descIngestBytes   *prometheus.Desc
	descIngestChunks  *prometheus.Desc
	descIngestExpired *prometheus.Desc
	descLastIngest    *prometheus.Desc
	staleMultiplier   float64
	minStaleAge       time.Duration
}

// NewExporter creates a Prometheus collector for the internal storage state.
func NewExporter(s *Storage, staleCfg config.StaleConfig) *Exporter {
	return &Exporter{
		store:           s,
		staleMultiplier: staleCfg.StaleMultiplier,
		minStaleAge:     staleCfg.MinStaleAge.ToDuration(),
		descIngestBytes: prometheus.NewDesc(
			"metricz_ingest_bytes_total",
			"Total bytes received from the instance via ingest API.",
			[]string{"instance_id"}, nil,
		),
		descIngestChunks: prometheus.NewDesc(
			"metricz_ingest_chunks_total",
			"Total chunks received from the instance via ingest API.",
			[]string{"instance_id"}, nil,
		),
		descIngestExpired: prometheus.NewDesc(
			"metricz_ingest_transactions_expired_total",
			"Total chunked transactions dropped due to TTL expiration.",
			[]string{"instance_id"}, nil,
		),
		descLastIngest: prometheus.NewDesc(
			"metricz_ingest_last_timestamp_seconds",
			"Unix timestamp of the last successful ingest.",
			[]string{"instance_id"}, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.descIngestBytes
	ch <- e.descIngestChunks
	ch <- e.descIngestExpired
	ch <- e.descLastIngest
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	// state snapshot
	states := e.store.GetInstanceStates()
	now := time.Now()

	for instanceID, state := range states {
		// internal technical metrics
		ch <- prometheus.MustNewConstMetric(
			e.descIngestBytes,
			prometheus.CounterValue,
			float64(state.IngestStats.TotalBytes),
			instanceID)

		ch <- prometheus.MustNewConstMetric(
			e.descIngestChunks,
			prometheus.CounterValue,
			float64(state.IngestStats.TotalChunks),
			instanceID)

		ch <- prometheus.MustNewConstMetric(
			e.descIngestExpired,
			prometheus.CounterValue,
			float64(state.IngestStats.ExpiredTransactions),
			instanceID)

		if !state.IngestStats.LastIngest.IsZero() {
			ch <- prometheus.MustNewConstMetric(
				e.descLastIngest,
				prometheus.GaugeValue,
				float64(state.IngestStats.LastIngest.Unix()),
				instanceID)
		}

		// Ingest/A2S/RCon
		if state.PolledFamilies != nil {
			e.emitFamilies(ch, state.PolledFamilies)
		}
		if state.A2SFamilies != nil {
			e.emitFamilies(ch, state.A2SFamilies)
		}
		if state.RConFamilies != nil {
			e.emitFamilies(ch, state.RConFamilies)
		}

		// state metric
		if state.IngestedFamilies != nil {
			timeSince := now.Sub(state.LastIngestUpdate)

			calcThreshold := time.Duration(state.ScrapeInterval * e.staleMultiplier * float64(time.Second))
			threshold := max(calcThreshold, e.minStaleAge)

			if timeSince > threshold {
				log.Warn().
					Str("instance_id", instanceID).
					Dur("since_update", timeSince).
					Dur("threshold", threshold).
					Dur("interval", time.Duration(state.ScrapeInterval)).
					Msg("ingest metrics are stale, resetting status to 0")

				if state.CachedStatusFamily != nil {
					e.emitStatusZero(ch, state.CachedStatusFamily)
				}
			} else {
				e.emitFamilies(ch, state.IngestedFamilies)
			}
		}
	}
}

func (e *Exporter) emitFamilies(ch chan<- prometheus.Metric, families map[string]*dto.MetricFamily) {
	for _, family := range families {
		for _, m := range family.Metric {
			var labelNames []string
			var labelValues []string
			for _, pair := range m.Label {
				labelNames = append(labelNames, pair.GetName())
				labelValues = append(labelValues, pair.GetValue())
			}

			desc := prometheus.NewDesc(family.GetName(), family.GetHelp(), labelNames, nil)

			var val float64
			var valType prometheus.ValueType

			switch family.GetType() {
			case dto.MetricType_GAUGE:
				val = m.GetGauge().GetValue()
				valType = prometheus.GaugeValue
			case dto.MetricType_COUNTER:
				val = m.GetCounter().GetValue()
				valType = prometheus.CounterValue
			default:
				continue
			}

			metric, err := prometheus.NewConstMetric(desc, valType, val, labelValues...)
			if err == nil {
				ch <- metric
			} else {
				log.Error().Err(err).Str("metric", family.GetName()).Msg("failed to create metric")
			}
		}
	}
}

func (e *Exporter) emitStatusZero(ch chan<- prometheus.Metric, family *dto.MetricFamily) {
	for _, m := range family.Metric {
		var labelNames []string
		var labelValues []string
		for _, pair := range m.Label {
			labelNames = append(labelNames, pair.GetName())
			labelValues = append(labelValues, pair.GetValue())
		}

		desc := prometheus.NewDesc(family.GetName(), family.GetHelp(), labelNames, nil)
		metric, _ := prometheus.NewConstMetric(desc, prometheus.GaugeValue, 0, labelValues...)
		ch <- metric
	}
}
