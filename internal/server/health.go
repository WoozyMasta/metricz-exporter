package server

import (
	"encoding/json"
	"net/http"

	"github.com/woozymasta/metricz-exporter/internal/vars"
)

// HealthResponse wraps the status and build information.
type HealthResponse struct {
	Status string         `json:"status"`
	Build  vars.BuildInfo `json:"build"`
}

// writeHealthResponse sends a JSON response with app info.
func (h *Handler) writeHealthResponse(w http.ResponseWriter) {
	resp := HealthResponse{
		Status: "UP",
		Build:  vars.Info(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(resp)
}

// HandleLiveness reports liveness status.
func (h *Handler) HandleLiveness(w http.ResponseWriter, _ *http.Request) {
	h.writeHealthResponse(w)
}

// HandleReadiness reports readiness status.
func (h *Handler) HandleReadiness(w http.ResponseWriter, _ *http.Request) {
	h.writeHealthResponse(w)
}
