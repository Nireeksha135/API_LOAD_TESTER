package metrics

import (
	"math/bits"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

const timeSeriesLatencyBinCount = 256

type secondBucket struct {
	requests int64
	errors   int64
	latency  [timeSeriesLatencyBinCount]uint32
}

func (b *secondBucket) record(result models.RequestResult) {
	b.requests++
	if !result.Success {
		b.errors++
	}
	latencyNanos := result.Latency.Nanoseconds()
	if latencyNanos < 1 {
		latencyNanos = 1
	}
	value := uint64(latencyNanos)
	exponent := bits.Len64(value) - 1
	lower := uint64(1) << exponent
	subBucket := int((value - lower) * 4 / lower)
	if subBucket > 3 {
		subBucket = 3
	}
	bin := exponent*4 + subBucket
	b.latency[bin]++
}

func (b *secondBucket) merge(other *secondBucket) {
	b.requests += other.requests
	b.errors += other.errors
	for i, count := range other.latency {
		b.latency[i] += count
	}
}

func (b *secondBucket) p95() time.Duration {
	if b.requests == 0 {
		return 0
	}
	target := uint64((b.requests*95 + 99) / 100)
	var count uint64
	for bin, binCount := range b.latency {
		count += uint64(binCount)
		if count >= target {
			exponent := bin / 4
			subBucket := bin % 4
			lower := int64(1) << exponent
			width := lower / 4
			if width < 1 {
				width = 1
			}
			return time.Duration(lower + int64(subBucket+1)*width - 1)
		}
	}
	return 0
}

func (a aggregate) timeSeriesPoints(totalDuration time.Duration) []models.TimeSeriesPoint {
	if a.timeSeries == nil {
		return nil
	}

	lastSecond := int64(0)
	if totalDuration > 0 {
		lastSecond = int64((totalDuration - 1) / time.Second)
	}
	for second := range a.timeSeries {
		if second > lastSecond {
			lastSecond = second
		}
	}

	points := make([]models.TimeSeriesPoint, 0, lastSecond+1)
	for second := int64(0); second <= lastSecond; second++ {
		bucket := a.timeSeries[second]
		point := models.TimeSeriesPoint{Second: second}
		if bucket != nil {
			point.Requests = bucket.requests
			point.Errors = bucket.errors
			point.P95 = bucket.p95()
		}
		bucketDuration := time.Second
		if second == lastSecond && totalDuration > 0 && totalDuration%time.Second != 0 {
			bucketDuration = totalDuration % time.Second
		}
		if point.Requests > 0 && bucketDuration > 0 {
			point.RPS = float64(point.Requests) / bucketDuration.Seconds()
		}
		points = append(points, point)
	}
	return points
}
