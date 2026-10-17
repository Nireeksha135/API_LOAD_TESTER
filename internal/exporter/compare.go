package exporter

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type reportMetrics struct {
	P95MS     float64 `json:"p95_latency_ms"`
	P99MS     float64 `json:"p99_latency_ms"`
	RPS       float64 `json:"requests_per_second"`
	Failed    int64   `json:"failed_requests"`
	Total     int64   `json:"total_requests"`
	Scheduled int64   `json:"scheduled_requests"`
	Dropped   int64   `json:"dropped_requests"`
	P95NS     int64   `json:"p95_latency_ns"`
	P99NS     int64   `json:"p99_latency_ns"`
}

type Regression struct {
	Name          string
	ChangePercent float64
	Worse         bool
}

// CompareJSON compares two exported reports. A positive maxRegression is a percentage limit.
func CompareJSON(baselinePath, currentPath string, maxRegression float64) ([]Regression, error) {
	read := func(path string) (reportMetrics, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return reportMetrics{}, fmt.Errorf("read report %q: %w", path, err)
		}
		var m reportMetrics
		if err := json.Unmarshal(data, &m); err != nil {
			return m, fmt.Errorf("parse report %q: %w", path, err)
		}
		if m.P95MS == 0 {
			m.P95MS = float64(m.P95NS) / 1e6
		}
		if m.P99MS == 0 {
			m.P99MS = float64(m.P99NS) / 1e6
		}
		if m.Total == 0 {
			return m, fmt.Errorf("report %q has no completed requests", path)
		}
		return m, nil
	}
	base, err := read(baselinePath)
	if err != nil {
		return nil, err
	}
	current, err := read(currentPath)
	if err != nil {
		return nil, err
	}
	rate := func(m reportMetrics) float64 {
		denominator := m.Scheduled
		if denominator <= 0 {
			denominator = m.Total
		}
		return float64(m.Failed+m.Dropped) / float64(denominator)
	}
	values := []struct {
		name          string
		before, after float64
		lowerIsBetter bool
	}{
		{"P95 latency", base.P95MS, current.P95MS, false}, {"P99 latency", base.P99MS, current.P99MS, false}, {"Throughput (requests/sec)", base.RPS, current.RPS, true}, {"Error rate", rate(base), rate(current), false},
	}
	results := make([]Regression, 0, len(values))
	for _, v := range values {
		change := 0.0
		if v.before == 0 {
			if v.after > 0 {
				change = math.Inf(1)
			} else if v.after < 0 {
				change = -100
			}
		} else {
			change = (v.after - v.before) / v.before * 100
		}
		worse := change > 0
		if v.lowerIsBetter {
			worse = change < 0
		}
		results = append(results, Regression{Name: v.name, ChangePercent: change, Worse: worse && abs(change) > maxRegression})
	}
	return results, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
