package storage

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// StartGarbageCollector runs a background loop to clean up expired staging transactions.
func (s *Storage) StartGarbageCollector(ctx context.Context, checkInterval time.Duration) {
	log.Info().
		Dur("interval", checkInterval).
		Msg("starting staging garbage collector")

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Msg("stopping staging garbage collector")
			return

		case <-ticker.C:
			count := s.cleanupStaging()
			if count > 0 {
				log.Info().
					Int("expired_transactions", count).
					Msg("cleaned up expired transactions with staging garbage collector")
			}
		}
	}
}

// cleanupStaging removes expired items and returns the count of removed items.
func (s *Storage) cleanupStaging() int {
	s.stagingMu.Lock()
	defer s.stagingMu.Unlock()

	// We need liveMu because we update stats in liveStore
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	now := time.Now()
	removedCount := 0

	for key, item := range s.stagingStore {
		if now.After(item.ExpiresAt) {
			state, exists := s.liveStore[item.InstanceID]
			if !exists {
				state = &InstanceState{
					IngestStats: IngestStats{},
				}
				s.liveStore[item.InstanceID] = state
			}

			state.IngestStats.ExpiredTransactions++
			s.stagingSize -= item.ByteSize
			delete(s.stagingStore, key)
			removedCount++

			log.Trace().
				Str("txn", key).
				Str("instance_id", item.InstanceID).
				Msg("staging garbage collector drop transaction")
		}
	}

	return removedCount
}
