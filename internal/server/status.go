package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	dto "github.com/prometheus/client_model/go"
	"github.com/woozymasta/metricz-exporter/internal/config"
	"github.com/woozymasta/metricz-exporter/internal/storage"
)

type publicStatusData struct {
	Values map[string]float64             `json:"values"`
	Labels map[string]map[string][]string `json:"labels"`
}

// HandleStatusSingle returns public status for a single instance.
func (h *Handler) HandleStatusSingle(w http.ResponseWriter, r *http.Request) {
	instanceID := chi.URLParam(r, "instance_id")
	h.serveStatusRequest(w, instanceID, false)
}

// HandleStatusFull returns public status for all instances.
func (h *Handler) HandleStatusFull(w http.ResponseWriter, _ *http.Request) {
	h.serveStatusRequest(w, "all", true)
}

func (h *Handler) serveStatusRequest(w http.ResponseWriter, cacheKey string, isAll bool) {
	if !h.cfg.App.Public.Enabled {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// CORS
	if h.cfg.App.Public.PublicCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	}

	// Cache Check
	if item, ok := h.publicCache.Load(cacheKey); ok {
		cacheItem := item.(*publicCacheItem)
		if time.Now().Before(cacheItem.expiresAt) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			_, _ = w.Write(cacheItem.response)
			return
		}
	}

	// Get Data Snapshot
	states := h.store.GetInstanceStates()

	var body []byte
	var err error

	if isAll {
		resp := make(map[string]*publicStatusData)
		for id, state := range states {
			// Reuse the collector logic
			resp[id] = collectPublicData(&state, h.cfg.PublicExport)
		}
		body, err = json.Marshal(resp)
	} else {
		state, exists := states[cacheKey] // cacheKey
		if !exists {
			http.Error(w, "Instance not found", http.StatusNotFound)
			return
		}
		data := collectPublicData(&state, h.cfg.PublicExport)
		body, err = json.Marshal(data)
	}

	if err != nil {
		http.Error(w, "JSON error", http.StatusInternalServerError)
		return
	}

	// Store in Cache
	h.publicCache.Store(cacheKey, &publicCacheItem{
		response:  body,
		expiresAt: time.Now().Add(h.cfg.App.Public.PublicCacheTTL.ToDuration()),
	})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	_, _ = w.Write(body)
}

// collectPublicData helper transforms internal state to clean JSON structure based on config allow-list
func collectPublicData(state *storage.InstanceState, cfg config.PublicExportConfig) *publicStatusData {
	out := &publicStatusData{
		Values: make(map[string]float64),
		Labels: make(map[string]map[string][]string),
	}

	// Maps for fast lookup
	wantValues := make(map[string]bool, len(cfg.Values))
	for _, v := range cfg.Values {
		wantValues[v] = true
	}

	wantLabels := make(map[string]bool, len(cfg.Labels))
	for _, l := range cfg.Labels {
		wantLabels[l] = true
	}

	processFamilies := func(families map[string]*dto.MetricFamily) {
		for name, mf := range families {
			if mf == nil || len(mf.Metric) == 0 {
				continue
			}

			// Values: sum across all samples in the family
			if wantValues[name] {
				var sum float64
				for _, m := range mf.Metric {
					if m == nil {
						continue
					}
					if m.Gauge != nil {
						sum += m.Gauge.GetValue()
					} else if m.Counter != nil {
						sum += m.Counter.GetValue()
					}
				}
				out.Values[name] = sum
			}

			// Labels: collect unique values per label key across all samples
			if wantLabels[name] {
				// labelKey -> set(value)
				sets := make(map[string]map[string]struct{})
				for _, m := range mf.Metric {
					if m == nil {
						continue
					}
					for _, lp := range m.Label {
						if lp == nil {
							continue
						}
						k := lp.GetName()
						if k == "" || slices.Contains(cfg.LabelsExclude, k) {
							continue
						}
						v := lp.GetValue()
						if _, ok := sets[k]; !ok {
							sets[k] = make(map[string]struct{})
						}
						sets[k][v] = struct{}{}
					}
				}

				kv := make(map[string][]string, len(sets))
				for k, set := range sets {
					vals := make([]string, 0, len(set))
					for v := range set {
						vals = append(vals, v)
					}
					slices.Sort(vals)
					kv[k] = vals
				}

				out.Labels[name] = kv
			}
		}
	}

	// Merge sources
	if state.PolledFamilies != nil {
		processFamilies(state.PolledFamilies)
	}
	if state.A2SFamilies != nil {
		processFamilies(state.A2SFamilies)
	}
	if state.RConFamilies != nil {
		processFamilies(state.RConFamilies)
	}

	// Ingested (Using cached status if needed, otherwise raw)
	if state.IngestedFamilies != nil {
		processFamilies(state.IngestedFamilies)
	} else if state.CachedStatusFamily != nil {
		tmp := map[string]*dto.MetricFamily{
			state.CachedStatusFamily.GetName(): state.CachedStatusFamily,
		}

		processFamilies(tmp)
	}

	return out
}
