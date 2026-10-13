package metrics

import (
	"math/bits"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

const timeSeriesLatencyBinCount = 256

type secondBucket struct {
	requests int64
	errors   int64
	latency  [timeSeriesLatencyBinCount]uint64
}

type timeSeriesStore struct {
	mu      sync.RWMutex
	buckets map[int64]*atomicSecondBucket
}

type atomicSecondBucket struct {
	requests atomic.Uint64
	errors   atomic.Uint64
	latency  [timeSeriesLatencyBinCount]atomic.Uint64
}

func newTimeSeriesStore() *timeSeriesStore {
	return &timeSeriesStore{buckets: make(map[int64]*atomicSecondBucket)}
}

func (s *timeSeriesStore) record(second int64, result models.RequestResult) {
	s.mu.RLock()
	bucket := s.buckets[second]
	s.mu.RUnlock()
	if bucket == nil {
		s.mu.Lock()
		bucket = s.buckets[second]
		if bucket == nil {
			bucket = &atomicSecondBucket{}
			s.buckets[second] = bucket
		}
		s.mu.Unlock()
	}

	bucket.requests.Add(1)
	if !result.Success {
		bucket.errors.Add(1)
	}
	bucket.latency[latencyBin(result.Latency)].Add(1)
}

func (s *timeSeriesStore) snapshot() map[int64]*secondBucket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[int64]*secondBucket, len(s.buckets))
	for second, source := range s.buckets {
		bucket := &secondBucket{
			requests: int64(source.requests.Load()),
			errors:   int64(source.errors.Load()),
		}
		for i := range source.latency {
			bucket.latency[i] = source.latency[i].Load()
		}
		snapshot[second] = bucket
	}
	return snapshot
}

func latencyBin(latency time.Duration) int {
	latencyNanos := latency.Nanoseconds()
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
	return exponent*4 + subBucket
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
