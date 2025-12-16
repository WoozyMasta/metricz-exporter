package server

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/hlog"
	"github.com/woozymasta/metricz-exporter/internal/parser"
)

func (h *Handler) handleChunkIngest(w http.ResponseWriter, r *http.Request) {
	txnHash := chi.URLParam(r, "txn_hash")
	instanceID := chi.URLParam(r, "instance_id")
	seqIDStr := chi.URLParam(r, "seq_id")
	logger := hlog.FromRequest(r)

	seqID, err := strconv.Atoi(seqIDStr)
	if err != nil {
		logger.Error().
			Err(err).
			Str("sequence_id", seqIDStr).
			Msg("failed to read sequence id")
		http.Error(w, "Invalid seq_id", http.StatusBadRequest)

		return
	}

	bodyReader := http.MaxBytesReader(w, r.Body, h.cfg.App.Ingest.MaxBodySize)
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		logger.Error().
			Err(err).
			Str("instance_id", instanceID).
			Msg("failed to read chunk body")
		http.Error(w, "Failed to read body", http.StatusInternalServerError)

		return
	}
	defer func() { _ = r.Body.Close() }()

	// Pass config TTL and InstanceID for dynamic calculation
	h.store.AppendToStaging(txnHash, instanceID, seqID, body, h.cfg.App.Ingest.TransactionTTL.ToDuration())

	logger.Trace().
		Str("txn", txnHash).
		Str("instance_id", instanceID).
		Int("seq_id", seqID).
		Int("bytes", len(body)).
		Msg("chunk received")

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("OK"))
}

func (h *Handler) handleCommit(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	txnHash := chi.URLParam(r, "txn_hash")
	logger := hlog.FromRequest(r)

	reader, chunkCount, totalBytes, ok := h.store.RetrieveStaging(txnHash)
	if !ok {
		logger.Warn().
			Str("instance_id", instanceID).
			Str("txn", txnHash).
			Msg("commit failed, transaction not found or empty")
		http.Error(w, "Transaction not found or empty", http.StatusNotFound)

		return
	}

	metrics, err := parser.ParseAndValidate(reader, instanceID, h.cfg.App)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("instance_id", instanceID).
			Str("txn", txnHash).
			Int("chunks", chunkCount).
			Int("total_bytes", totalBytes).
			Msg("commit validation failed")
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	h.store.UpdateIngested(instanceID, metrics, totalBytes, chunkCount)

	logger.Debug().
		Str("instance_id", instanceID).
		Str("txn", txnHash).
		Int("chunks", chunkCount).
		Int("total_bytes", totalBytes).
		Int("families", len(metrics)).
		Msg("transaction committed")

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
