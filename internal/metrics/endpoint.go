package metrics

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

const (
	maxEndpointCount   = 31 // plus the shared overflow bucket, for 32 buckets total
	endpointOther      = "(other)"
	endpointShardCount = 4
)

type endpointRegistry struct {
	mu        sync.RWMutex
	endpoints map[string]*endpointAggregate
}

type endpointAggregate struct {
	shards [endpointShardCount]endpointShardMetric
}
type endpointShardMetric struct {
	mu                          sync.Mutex
	requests, errors, cancelled int64
	sum                         time.Duration
	hist                        *hdrhistogram.Histogram
}

func newEndpointRegistry() *endpointRegistry {
	return &endpointRegistry{endpoints: make(map[string]*endpointAggregate)}
}

var (
	numericID     = regexp.MustCompile(`^[0-9]+$`)
	hexID         = regexp.MustCompile(`(?i)^[0-9a-f]{24}$`)
	uuidID        = regexp.MustCompile(`(?i)^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$`)
	mixedID       = regexp.MustCompile(`(?i)^[a-z0-9]{6,}$`)
	templateToken = regexp.MustCompile(`\{\{[^}]+\}\}`)
)

// NormalizeEndpoint groups common dynamic path IDs and removes query strings and trailing slashes.
// Callers should pass configured URL templates so {{counter}} and {{random}} never become distinct endpoints.
func NormalizeEndpoint(raw string) string {
	raw = templateToken.ReplaceAllString(raw, ":id")
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	segments := strings.Split(u.Path, "/")
	for i := range segments {
		segment := segments[i]
		if segment == "" {
			continue
		}
		if isDynamicID(segment) {
			segments[i] = ":id"
		}
	}
	path := strings.TrimRight(strings.Join(segments, "/"), "/")
	if path == "" {
		path = "/"
	}
	if u.Scheme == "" && u.Host == "" {
		return path
	}
	u.Path = path
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

func isDynamicID(segment string) bool {
	if numericID.MatchString(segment) || hexID.MatchString(segment) || uuidID.MatchString(segment) {
		return true
	}
	if !mixedID.MatchString(segment) {
		return false
	}
	hasLetter, hasDigit := false, false
	for _, r := range segment {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

func (r *endpointRegistry) record(key string, shard int, result models.RequestResult) {
	r.mu.RLock()
	metric := r.endpoints[key]
	r.mu.RUnlock()
	if metric == nil {
		r.mu.Lock()
		metric = r.endpoints[key]
		if metric == nil {
			if len(r.endpoints) >= maxEndpointCount {
				key = endpointOther
				metric = r.endpoints[key]
			}
			if metric == nil {
				metric = &endpointAggregate{}
				r.endpoints[key] = metric
			}
		}
		r.mu.Unlock()
	}
	m := &metric.shards[shard%endpointShardCount]
	m.mu.Lock()
	m.requests++
	if result.Cancelled {
		m.cancelled++
	} else if !result.Success {
		m.errors++
	}
	m.sum += result.Latency
	if m.hist == nil {
		m.hist = hdrhistogram.New(1, maxLatencyNanoseconds, 2)
	}
	nanos := result.Latency.Nanoseconds()
	if nanos < 1 {
		nanos = 1
	}
	_ = m.hist.RecordValue(nanos)
	m.mu.Unlock()
}

func (r *endpointRegistry) summary() []models.EndpointSummary {
	r.mu.RLock()
	items := make([]struct {
		key   string
		value *endpointAggregate
	}, 0, len(r.endpoints))
	for key, value := range r.endpoints {
		items = append(items, struct {
			key   string
			value *endpointAggregate
		}{key, value})
	}
	r.mu.RUnlock()
	results := make([]models.EndpointSummary, 0, len(items))
	for _, item := range items {
		merged := hdrhistogram.New(1, maxLatencyNanoseconds, 2)
		var requests, errors, cancelled int64
		var sum time.Duration
		for i := range item.value.shards {
			m := &item.value.shards[i]
			m.mu.Lock()
			requests += m.requests
			errors += m.errors
			cancelled += m.cancelled
			sum += m.sum
			if m.hist != nil {
				merged.Merge(m.hist)
			}
			m.mu.Unlock()
		}
		completed := requests - cancelled
		var rate float64
		if completed > 0 {
			rate = float64(errors) / float64(completed)
		}
		var mean time.Duration
		if requests > 0 {
			mean = time.Duration(int64(sum) / requests)
		}
		results = append(results, models.EndpointSummary{Endpoint: item.key, Requests: requests, Errors: errors, CancelledRequests: cancelled, ErrorRate: rate, MeanLatency: mean, P95: time.Duration(merged.ValueAtQuantile(95)), P99: time.Duration(merged.ValueAtQuantile(99))})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].P95 == results[j].P95 {
			return results[i].Endpoint < results[j].Endpoint
		}
		return results[i].P95 > results[j].P95
	})
	return results
}
