// Package metrics: percentile.go implements percentile computation
// over a sorted slice of latencies using linear interpolation between
// the two closest ranks (the same method used by many industry
// load-testing tools, sometimes called "R7" / linear interpolation).
package metrics

import (
	"math"
	"time"
)

// Percentile computes the p-th percentile (0 <= p <= 100) of
// sortedLatencies, which MUST already be sorted in ascending order.
// It uses linear interpolation between the two nearest ranks for
// non-integral rank positions, and returns 0 for an empty input.
//
// Examples:
//
//	Percentile([]time.Duration{10,20,30,40,50} (ms), 50)  -> 30ms
//	Percentile([]time.Duration{10,20,30,40,50} (ms), 0)   -> 10ms
//	Percentile([]time.Duration{10,20,30,40,50} (ms), 100) -> 50ms
func Percentile(sortedLatencies []time.Duration, p float64) time.Duration {
	n := len(sortedLatencies)
	if n == 0 {
		return 0
	}
	if p <= 0 {
		return sortedLatencies[0]
	}
	if p >= 100 {
		return sortedLatencies[n-1]
	}

	rank := (p / 100) * float64(n-1)
	lowerIdx := int(math.Floor(rank))
	upperIdx := int(math.Ceil(rank))

	if lowerIdx == upperIdx {
		return sortedLatencies[lowerIdx]
	}

	weight := rank - float64(lowerIdx)
	lowerVal := float64(sortedLatencies[lowerIdx])
	upperVal := float64(sortedLatencies[upperIdx])

	return time.Duration(lowerVal + weight*(upperVal-lowerVal))
}
