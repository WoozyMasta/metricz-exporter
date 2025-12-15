package server

import (
	_ "embed"
	"html/template"
	"net/http"
	"time"

	"github.com/woozymasta/metricz-exporter/internal/vars"
)

//go:embed index.html
var indexHTML string

// Link represents a navigation item on the index page
type Link struct {
	Title string
	URL   string
	Desc  string
	IsExt bool // opens in new tab
}

// IndexData is the structure passed to the template
type IndexData struct {
	Build          vars.BuildInfo
	Internal       []Link
	External       []Link
	Year           int
	InstancesCount int
}

// HandleIndex create simple index page.
func (h *Handler) HandleIndex(w http.ResponseWriter, _ *http.Request) {
	tpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	states := h.store.GetInstanceStates()

	data := IndexData{
		Build:          vars.Info(),
		Year:           time.Now().Year(),
		InstancesCount: len(states),
		// Endpoints
		Internal: []Link{
			{Title: "Metrics", URL: "/metrics", Desc: "Prometheus scrape endpoint"},
			{Title: "Status API", URL: "/api/v1/status", Desc: "Public JSON server status"},
			{Title: "Health", URL: "/health", Desc: "Liveness/Readiness probes"},
		},
		// Resources
		External: []Link{
			{Title: "MetricZ DayZ Mod", URL: "https://github.com/WoozyMasta/metricz", Desc: "DayZ Mod Source Code", IsExt: true},
			{Title: "MetricZ Exporter", URL: vars.URL, Desc: "Exporter Source Code", IsExt: true},
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tpl.Execute(w, data)
}
