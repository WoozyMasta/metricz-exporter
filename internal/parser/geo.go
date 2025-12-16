package parser

import (
	"math"
	"strconv"

	dto "github.com/prometheus/client_model/go"
	"github.com/woozymasta/metricz-exporter/internal/config"
)

// applyGeoTransform applies transformation of game position to EPSG:4326 (WGS84) format
func applyGeoTransform(families map[string]*dto.MetricFamily, cfg config.GeoTransformConfig) {
	mf, ok := families[cfg.WorldSizeMetric]
	if !ok {
		return
	}

	// get world size
	var worldSize float64
	if len(mf.Metric) > 0 {
		if mf.Metric[0].Gauge != nil {
			worldSize = mf.Metric[0].Gauge.GetValue()
		} else if mf.Metric[0].Counter != nil {
			worldSize = mf.Metric[0].Counter.GetValue()
		}
	}
	if worldSize <= 0 {
		return
	}

	// pre-calc coef
	lonScale := 360.0 / worldSize
	mercScale := (2.0 * math.Pi) / worldSize

	// transformation functions
	calcLon := func(x float64) float64 {
		return x*lonScale - 180.0
	}

	calcLat := func(z float64) float64 {
		mercatorY := z*mercScale - math.Pi
		latRad := (2.0 * math.Atan(math.Exp(mercatorY))) - (math.Pi * 0.5)
		lat := latRad * (180.0 / math.Pi)
		if lat > maxLat {
			return maxLat
		}
		if lat < -maxLat {
			return -maxLat
		}
		return lat
	}

	// sets to prevent double-processing the same metric
	processedValX := make(map[string]bool)
	processedValZ := make(map[string]bool)
	processedLabels := make(map[string]bool)

	// transform metric value X -> longitude
	for _, name := range cfg.MetricsValueX {
		if processedValX[name] {
			continue
		} // Skip duplicates

		if mf, ok := families[name]; ok {
			for _, m := range mf.Metric {
				if m.Gauge != nil {
					newVal := calcLon(m.Gauge.GetValue())
					m.Gauge.Value = &newVal
				}
			}
			processedValX[name] = true
		}
	}

	// transform metric value Z -> latitude
	for _, name := range cfg.MetricsValueZ {
		if processedValZ[name] {
			continue
		}

		if mf, ok := families[name]; ok {
			for _, m := range mf.Metric {
				if m.Gauge != nil {
					newVal := calcLat(m.Gauge.GetValue())
					m.Gauge.Value = &newVal
				}
			}
			processedValZ[name] = true
		}
	}

	// transform labels
	for _, name := range cfg.MetricsLabelTargets {
		if processedLabels[name] {
			continue
		}

		if mf, ok := families[name]; ok {
			for _, m := range mf.Metric {
				for _, lp := range m.Label {
					switch lp.GetName() {
					case cfg.MetricsLabelNameX: // longitude
						val, err := strconv.ParseFloat(lp.GetValue(), 64)
						if err == nil {
							newVal := calcLon(val)
							strVal := strconv.FormatFloat(newVal, 'f', 6, 64)
							lp.Value = &strVal
						}

					case cfg.MetricsLabelNameZ: // latitude
						val, err := strconv.ParseFloat(lp.GetValue(), 64)
						if err == nil {
							newVal := calcLat(val)
							strVal := strconv.FormatFloat(newVal, 'f', 6, 64)
							lp.Value = &strVal
						}
					}
				}
			}
			processedLabels[name] = true
		}
	}
}
