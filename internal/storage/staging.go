package storage

import (
	"bytes"
	"io"
	"sort"
	"time"
)

// StagingItem holds the buffer and metadata for an ongoing transaction.
type StagingItem struct {
	ExpiresAt  time.Time
	Chunks     map[int][]byte
	InstanceID string
}

// AppendToStaging appends data.
// It calculates TTL based on defaultTTL OR the instance's known scrape interval (whichever is larger).
// If the transaction is new, it initializes it.
// If the transaction exists but expired, it resets it (starts over).
func (s *Storage) AppendToStaging(txnHash string, instanceID string, seqID int, data []byte, defaultTTL time.Duration) {
	s.stagingMu.Lock()
	defer s.stagingMu.Unlock()

	item, exists := s.stagingStore[txnHash]
	now := time.Now()

	// Logic: If item exists but expired -> Treat as new (Reset)
	if exists && now.After(item.ExpiresAt) {
		delete(s.stagingStore, txnHash)
		exists = false
	}

	if !exists {
		// Delete stale ingest data
		for key, val := range s.stagingStore {
			if val.InstanceID == instanceID {
				delete(s.stagingStore, key)
			}
		}

		// Calculate dynamic TTL
		ttl := defaultTTL

		// Check if we know this instance and if it has a larger scrape interval
		// We use RLock on liveStore to be safe
		s.liveMu.RLock()
		if state, ok := s.liveStore[instanceID]; ok {
			// If scrape interval is defined (e.g. 60s) and larger than default TTL (15s),
			// extend the window to avoid cutting off slow uploads on slow scrape cycles.
			scrapeDuration := time.Duration(state.ScrapeInterval) * time.Second
			if scrapeDuration > ttl {
				ttl = scrapeDuration
			}
		}
		s.liveMu.RUnlock()

		item = &StagingItem{
			Chunks:     make(map[int][]byte),
			ExpiresAt:  now.Add(ttl),
			InstanceID: instanceID,
		}
		s.stagingStore[txnHash] = item
	}

	item.Chunks[seqID] = data
}

// RetrieveStaging returns buffer and chunk count.
func (s *Storage) RetrieveStaging(txnHash string) (io.Reader, int, int, bool) {
	s.stagingMu.Lock()
	defer s.stagingMu.Unlock()

	item, exists := s.stagingStore[txnHash]
	if !exists || time.Now().After(item.ExpiresAt) {
		delete(s.stagingStore, txnHash)
		return nil, 0, 0, false
	}

	delete(s.stagingStore, txnHash)

	if len(item.Chunks) == 0 {
		return nil, 0, 0, false
	}

	keys := make([]int, 0, len(item.Chunks))
	for k := range item.Chunks {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	var totalSize int
	var readers []io.Reader
	newLine := []byte("\n")

	for _, k := range keys {
		chunkData := item.Chunks[k]
		totalSize += len(chunkData)
		readers = append(readers, bytes.NewReader(chunkData))

		if len(chunkData) > 0 && chunkData[len(chunkData)-1] != '\n' {
			readers = append(readers, bytes.NewReader(newLine))
			totalSize++
		}
	}

	return io.MultiReader(readers...), len(keys), totalSize, true
}
