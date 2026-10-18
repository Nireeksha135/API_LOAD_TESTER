package metrics

import (
	"sort"
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

const (
	// endpointShards spreads writers for one endpoint over several locks so
	// per-endpoint tracking does not turn into a single hot mutex.
	endpointShards = 16

	// maxTrackedEndpoints bounds memory: endpoints beyond this many are folded
	// into OtherEndpoint.
	maxTrackedEndpoints = 64

	// OtherEndpoint is the label that collects endpoints beyond the tracking limit.
	OtherEndpoint = "(other)"
)

// WithEndpointStats enables per-endpoint statistics. Results are grouped by
// models.RequestResult.Endpoint; results without an endpoint are ignored.
func WithEndpointStats() Option {
	return func(c *Collector) {
		c.endpoints = newEndpointStore()
	}
}

// endpointStore is shared by a parent collector and all of its worker collectors.
type endpointStore struct {
	mu    sync.RWMutex
	stats map[string]*endpointStat
}

type endpointStat struct {
	shards [endpointShards]endpointShard
}

type endpointShard struct {
	mu         sync.Mutex
	requests   int64
	failed     int64
	cancelled  int64
	sumLatency time.Duration
	latencies  *hdrhistogram.Histogram // allocated on first use
}

func newEndpointStore() *endpointStore {
	return &endpointStore{stats: make(map[string]*endpointStat)}
}

func (s *endpointStore) stat(endpoint string) *endpointStat {
	s.mu.RLock()
	stat := s.stats[endpoint]
	s.mu.RUnlock()
	if stat != nil {
		return stat
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if stat = s.stats[endpoint]; stat != nil {
		return stat
	}
	if len(s.stats) >= maxTrackedEndpoints {
		endpoint = OtherEndpoint
		if stat = s.stats[endpoint]; stat != nil {
			return stat
		}
	}
	stat = &endpointStat{}
	s.stats[endpoint] = stat
	return stat
}

func (s *endpointStore) record(result models.RequestResult) {
	stat := s.stat(result.Endpoint)
	shard := &stat.shards[uint(result.WorkerID)%endpointShards]

	latencyNanos := result.Latency.Nanoseconds()
	if latencyNanos < 1 {
		latencyNanos = 1
	}

	shard.mu.Lock()
	shard.requests++
	shard.sumLatency += result.Latency
	switch {
	case result.Cancelled:
		shard.cancelled++
	case !result.Success:
		shard.failed++
	}
	if shard.latencies == nil {
		shard.latencies = hdrhistogram.New(1, maxLatencyNanoseconds, 2)
	}
	// Latencies are always within the histogram range, so the error is ignored
	// rather than allowed to take down a running load test.
	_ = shard.latencies.RecordValue(latencyNanos)
	shard.mu.Unlock()
}

// snapshot merges every shard and returns the endpoints sorted from slowest to
// fastest by P95, then P99, then mean latency, then name.
func (s *endpointStore) snapshot() []models.EndpointSummary {
	s.mu.RLock()
	names := make([]string, 0, len(s.stats))
	stats := make(map[string]*endpointStat, len(s.stats))
	for name, stat := range s.stats {
		names = append(names, name)
		stats[name] = stat
	}
	s.mu.RUnlock()

	out := make([]models.EndpointSummary, 0, len(names))
	for _, name := range names {
		merged := hdrhistogram.New(1, maxLatencyNanoseconds, 2)
		summary := models.EndpointSummary{Endpoint: name}
		var sum time.Duration
		for i := range stats[name].shards {
			shard := &stats[name].shards[i]
			shard.mu.Lock()
			summary.Requests += shard.requests
			summary.Failed += shard.failed
			summary.Cancelled += shard.cancelled
			sum += shard.sumLatency
			if shard.latencies != nil {
				merged.Merge(shard.latencies)
			}
			shard.mu.Unlock()
		}
		if summary.Requests > 0 {
			summary.MeanLatency = time.Duration(int64(sum) / summary.Requests)
		}
		summary.P50 = time.Duration(merged.ValueAtQuantile(50))
		summary.P95 = time.Duration(merged.ValueAtQuantile(95))
		summary.P99 = time.Duration(merged.ValueAtQuantile(99))
		out = append(out, summary)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.P95 != b.P95 {
			return a.P95 > b.P95
		}
		if a.P99 != b.P99 {
			return a.P99 > b.P99
		}
		if a.MeanLatency != b.MeanLatency {
			return a.MeanLatency > b.MeanLatency
		}
		return a.Endpoint < b.Endpoint
	})
	return out
}
