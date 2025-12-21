package poller

import dto "github.com/prometheus/client_model/go"

// Helper to create DTO structures cleanly
func addGauge(families map[string]*dto.MetricFamily, name, help string, value float64, instanceID string) {
	labelName := "instance_id"
	metric := &dto.Metric{
		Label: []*dto.LabelPair{
			{
				Name:  &labelName,
				Value: &instanceID,
			},
		},
		Gauge: &dto.Gauge{
			Value: &value,
		},
	}

	// Create new series or add to exists
	if mf, ok := families[name]; ok {
		mf.Metric = append(mf.Metric, metric)
	} else {
		families[name] = &dto.MetricFamily{
			Name:   &name,
			Help:   &help,
			Type:   dto.MetricType_GAUGE.Enum(),
			Metric: []*dto.Metric{metric},
		}
	}
}

func addGaugeWithLabels(families map[string]*dto.MetricFamily, name, help string, value float64, labels map[string]string) {
	var labelPairs []*dto.LabelPair
	for k, v := range labels {
		kCopy := k
		vCopy := v
		labelPairs = append(labelPairs, &dto.LabelPair{
			Name:  &kCopy,
			Value: &vCopy,
		})
	}

	metric := &dto.Metric{
		Label: labelPairs,
		Gauge: &dto.Gauge{Value: &value},
	}

	// Create new series or add to exists
	if mf, ok := families[name]; ok {
		mf.Metric = append(mf.Metric, metric)
	} else {
		families[name] = &dto.MetricFamily{
			Name:   &name,
			Help:   &help,
			Type:   dto.MetricType_GAUGE.Enum(),
			Metric: []*dto.Metric{metric},
		}
	}
}
