// Package storage stores instance states and exposes them to Prometheus collector.
package storage

import (
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// Storage holds live and staging metrics state.
type Storage struct {
	liveStore      map[string]*InstanceState
	stagingStore   map[string]*StagingItem
	stagingSize    int64
	maxStagingSize int64
	liveMu         sync.RWMutex
	stagingMu      sync.Mutex
}

// InstanceState holds the metrics and metadata for a specific game server instance.
type InstanceState struct {
	LastIngestUpdate   time.Time
	IngestedFamilies   map[string]*dto.MetricFamily
	CachedStatusFamily *dto.MetricFamily
	PolledFamilies     map[string]*dto.MetricFamily
	A2SFamilies        map[string]*dto.MetricFamily
	RConFamilies       map[string]*dto.MetricFamily
	IngestStats        IngestStats
	ScrapeInterval     float64
}

// IngestStats holds technical statistics about data ingestion.
type IngestStats struct {
	LastIngest          time.Time
	TotalBytes          int64
	TotalChunks         int64
	ExpiredTransactions int64
}

// New creates a new Storage.
func New(maxStagingSize int64) *Storage {
	return &Storage{
		liveStore:      make(map[string]*InstanceState),
		stagingStore:   make(map[string]*StagingItem),
		maxStagingSize: maxStagingSize,
	}
}

// UpdateIngested updates the metrics received from the mod (Push).
func (s *Storage) UpdateIngested(instanceID string, families map[string]*dto.MetricFamily, bytesAdded int, chunksAdded int) {
	interval := 60.0
	if mf, ok := families["dayz_metricz_scrape_interval_seconds"]; ok {
		if len(mf.Metric) > 0 && mf.Metric[0].Gauge != nil {
			interval = mf.Metric[0].Gauge.GetValue()
		}
	}

	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	state := s.getOrCreateState(instanceID)
	state.IngestedFamilies = families
	state.LastIngestUpdate = time.Now()
	state.ScrapeInterval = interval
	state.IngestStats.LastIngest = time.Now()
	state.IngestStats.TotalBytes += int64(bytesAdded)
	state.IngestStats.TotalChunks += int64(chunksAdded)

	if statusMF, ok := families["dayz_metricz_status"]; ok {
		state.CachedStatusFamily = statusMF
	}
}

// UpdatePolled updates the metrics collected by the exporter itself (A2S/RCon).
func (s *Storage) UpdatePolled(instanceID string, families map[string]*dto.MetricFamily) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	state := s.getOrCreateState(instanceID)
	state.PolledFamilies = families
}

// UpdateA2S stores A2S metrics for instance.
func (s *Storage) UpdateA2S(instanceID string, families map[string]*dto.MetricFamily) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	state := s.getOrCreateState(instanceID)
	state.A2SFamilies = families
}

// UpdateRCon stores RCon metrics for instance.
func (s *Storage) UpdateRCon(instanceID string, families map[string]*dto.MetricFamily) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	state := s.getOrCreateState(instanceID)
	state.RConFamilies = families
}

// getOrCreateState is a helper to ensure instance state exists.
// Must be called under liveMu.Lock()
func (s *Storage) getOrCreateState(instanceID string) *InstanceState {
	state, exists := s.liveStore[instanceID]
	if !exists {
		state = &InstanceState{
			IngestStats: IngestStats{},
		}
		s.liveStore[instanceID] = state
	}

	return state
}

// GetInstanceStates returns a SAFE COPY of the current state.
func (s *Storage) GetInstanceStates() map[string]InstanceState {
	s.liveMu.RLock()
	defer s.liveMu.RUnlock()

	result := make(map[string]InstanceState, len(s.liveStore))
	for k, v := range s.liveStore {
		result[k] = *v
	}

	return result
}
