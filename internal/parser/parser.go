// Package parser parses and validates Prometheus text exposition payloads.
package parser

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/cespare/xxhash/v2"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/woozymasta/dzid"
)

// ParseAndValidate parses Prometheus text format, injects/validates the instance_id,
// and deduplicates metrics using "Last Write Wins" strategy.
func ParseAndValidate(input io.Reader, targetInstanceID string, overwrite bool) (map[string]*dto.MetricFamily, error) {
	decoder := expfmt.NewDecoder(input, expfmt.NewFormat(expfmt.TypeTextPlain))
	families := make(map[string]*dto.MetricFamily)

	for {
		// Use a pointer to avoid "copying lock value" errors in protobuf structs
		mf := &dto.MetricFamily{}
		err := decoder.Decode(mf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing failed: %w", err)
		}

		// Check if we need to calculate BUID for this family
		isPlayerLoaded := mf.GetName() == "dayz_metricz_player_loaded"

		// Validation and Injection of instance_id
		for _, metric := range mf.Metric {
			if metric.Label == nil {
				metric.Label = make([]*dto.LabelPair, 0)
			}

			// Identity Enrichment (BUID calculation)
			if isPlayerLoaded {
				var steamIDStr string
				hasBUID := false

				// Quick scan for steam_id and existing buid
				for _, lp := range metric.Label {
					if lp.GetName() == "steam_id" {
						steamIDStr = lp.GetValue()
					} else if lp.GetName() == "buid" {
						hasBUID = true
					}
				}

				if steamIDStr != "" && !hasBUID {
					if sid, err := strconv.ParseInt(steamIDStr, 10, 64); err == nil {
						buid := dzid.BattlEyeString(sid)
						name := "buid" // Var for pointer
						metric.Label = append(metric.Label, &dto.LabelPair{
							Name:  &name,
							Value: &buid,
						})
					}
				}
			}

			// Instance ID Validation/Injection
			instanceIDFound := false
			for _, labelPair := range metric.Label {
				if labelPair.GetName() == "instance_id" {
					currentVal := labelPair.GetValue()
					if currentVal != targetInstanceID {
						if overwrite {
							val := targetInstanceID
							labelPair.Value = &val
						} else {
							return nil, fmt.Errorf(
								"instance_id mismatch in metric '%s': expected '%s', got '%s'",
								mf.GetName(), targetInstanceID, currentVal,
							)
						}
					}
					instanceIDFound = true
					break
				}
			}

			if !instanceIDFound {
				name := "instance_id"
				val := targetInstanceID
				metric.Label = append(metric.Label, &dto.LabelPair{
					Name:  &name,
					Value: &val,
				})
			}

			// Sort Labels
			// Critical for deduplication: {a=1,b=2} must be treated same as {b=2,a=1}
			sort.Slice(metric.Label, func(i, j int) bool {
				return metric.Label[i].GetName() < metric.Label[j].GetName()
			})
		}

		// Deduplication (Last Write Wins)
		// If the source sends duplicate metrics, only the last one is preserved.
		uniqueMetrics := make(map[uint64]*dto.Metric)
		for _, metric := range mf.Metric {
			hash := getLabelHash(metric.Label)
			uniqueMetrics[hash] = metric
		}

		// Rebuild slice if duplicates were removed
		if len(uniqueMetrics) != len(mf.Metric) {
			cleanMetrics := make([]*dto.Metric, 0, len(uniqueMetrics))
			for _, m := range uniqueMetrics {
				cleanMetrics = append(cleanMetrics, m)
			}
			mf.Metric = cleanMetrics
		}

		families[mf.GetName()] = mf
	}

	return families, nil
}

// getLabelHash generates a unique uint64 hash signature for a metric based on its labels.
// It uses xxhash for high performance and low collision probability.
func getLabelHash(labels []*dto.LabelPair) uint64 {
	d := xxhash.New()
	sep := []byte{0}

	for _, lp := range labels {
		_, _ = d.WriteString(lp.GetName())
		_, _ = d.Write(sep)
		_, _ = d.WriteString(lp.GetValue())
		_, _ = d.Write(sep)
	}

	return d.Sum64()
}
