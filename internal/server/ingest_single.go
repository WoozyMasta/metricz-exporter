package server

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/hlog"
	"github.com/woozymasta/metricz-exporter/internal/parser"
)

// countingReader wraps an io.Reader and counts the bytes read.
type countingReader struct {
	r     io.Reader
	count int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.count += int64(n)
	return n, err
}

func (h *Handler) handleSingleShot(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	logger := hlog.FromRequest(r)

	limitedReader := http.MaxBytesReader(w, r.Body, h.cfg.App.Ingest.MaxBodySize)
	counter := &countingReader{r: limitedReader}
	defer func() { _ = r.Body.Close() }()

	metrics, err := parser.ParseAndValidate(counter, instanceID, h.cfg.App)
	readBytes := int(counter.count)

	if err != nil {
		logger.Warn().
			Err(err).
			Str("instance_id", instanceID).
			Int("read_bytes", readBytes).
			Msg("single-shot ingest validation failed")

		if err.Error() == "http: request body too large" ||
			(err.Error() != "" && containsBodyTooLarge(err.Error())) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	h.store.UpdateIngested(instanceID, metrics, readBytes, 1)

	logger.Debug().
		Str("instance_id", instanceID).
		Int("families", len(metrics)).
		Int("bytes", readBytes).
		Msg("single-shot metrics updated")

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func containsBodyTooLarge(s string) bool {
	return len(s) >= 26 && s[len(s)-26:] == "http: request body too large"
}
