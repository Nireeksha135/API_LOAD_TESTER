# API Load Tester

[![Go](https://github.com/Nireeksha135/API_LOAD_TESTER/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/Nireeksha135/API_LOAD_TESTER/actions/workflows/go.yml)

### A concurrent HTTP API load testing tool built from scratch in Go.

API Load Tester is a CLI-based load testing framework designed to simulate concurrent HTTP traffic and analyze API performance.

The project focuses on **concurrency, worker pools, HTTP networking, thread-safe metrics, and performance analysis** without relying on existing load-testing tools.

---

## Tech Stack

![Go](https://img.shields.io/badge/GO-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Net HTTP](https://img.shields.io/badge/NET%2FHTTP-111111?style=for-the-badge)
![Goroutines](https://img.shields.io/badge/GOROUTINES-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Channels](https://img.shields.io/badge/CHANNELS-111111?style=for-the-badge&logo=go&logoColor=white)
![Concurrency](https://img.shields.io/badge/CONCURRENCY-7B61FF?style=for-the-badge)
![CLI](https://img.shields.io/badge/CLI-111111?style=for-the-badge)
![Testing](https://img.shields.io/badge/TESTING-2EA44F?style=for-the-badge)

---

## Features

- GET, POST, PUT, DELETE, PATCH and HEAD requests
- Configurable concurrency
- HTTP/2 negotiation for HTTPS targets (enabled by default)
- Worker pool architecture
- Fixed request count mode
- Fixed duration mode
- Open-loop target RPS with dropped-arrival accounting
- Separate tracking for requests cancelled at run shutdown
- Unmeasured warm-up and gradual ramp-up
- Per-second RPS, P95 and error time series
- Round-robin URL targets and per-request counter/random tokens
- YAML configuration files
- Custom HTTP headers
- JSON and raw request bodies
- Per-request latency tracking
- Bounded-memory HDR histogram percentiles
- Error counts grouped by underlying cause
- Min / Max / Mean latency
- P50 / P90 / P95 / P99 / P99.9 percentiles
- Requests Per Second (RPS)
- Throughput measurement
- HTTP status code breakdown
- Live terminal dashboard
- CSV and JSON export
- Normalized per-endpoint request, error-rate, and latency breakdowns
- JSON report comparison for P95/P99 latency, request throughput, and error-rate regressions
- Graceful shutdown
- Race-condition testing

---

## How It Works

```text
Target API
    ↓
CLI Configuration
    ↓
Worker Pool
 ┌──┴──┬──┴──┬──┴──┐
 ↓     ↓     ↓     ↓
HTTP  HTTP  HTTP  HTTP
Req.  Req.  Req.  Req.
 └──┬──┴──┬──┴──┬──┘
    ↓
Metrics Collector
    ↓
Performance Analysis
    ↓
Terminal / CSV / JSON
```

Each worker records results in its own collector. Live dashboard snapshots aggregate those collectors, and the final summary merges their counters and HDR histograms. The shared per-second time series avoids duplicating latency bins for every worker. Summary percentiles use a two-significant-digit HDR histogram across the full supported latency range, so percentile estimates can read about 1% high; a threshold close to the true percentile may fail because of histogram precision. The per-second time-series P95 is approximate: it uses coarse latency bins and reports each bin's upper edge, so it can read up to about 25% higher than the underlying latency.

---

## Metrics

The tool measures:

| Metric | Description |
|---|---|
| RPS | Requests processed per second |
| Throughput | Data processed over time |
| Min Latency | Fastest request |
| Max Latency | Slowest request |
| Mean Latency | Average response time |
| P50 | Median latency |
| P95 | 95th percentile latency |
| P99 | 99th percentile latency |
| Status Codes | HTTP response distribution |
| Errors | Count of each distinct unwrapped non-cancellation error cause |

---

## Usage

### Download a Release

Prebuilt binaries and checksums for tagged releases are published on [GitHub Releases](https://github.com/Nireeksha135/API_LOAD_TESTER/releases).

### Build

```bash
git clone https://github.com/Nireeksha135/API_LOAD_TESTER.git
cd API_LOAD_TESTER

go build -o bin/api-load-tester ./cmd
```

### Basic Load Test

```bash
api-load-tester -url https://api.example.com/health -c 50 -n 1000
```

### POST Request

```bash
api-load-tester \
  -url https://api.example.com/users \
  -method POST \
  -body '{"name":"load-test"}' \
  -c 100 \
  -d 30s
```

### CI Thresholds

Threshold failures leave the final report and requested exports available, then exit with status 1. Error rates accept a fraction or percentage. When thresholds are configured but no requests complete, the run fails because there is no measurement to evaluate.

```bash
api-load-tester -url https://api.example.com/health -n 1000 -c 50 \
    -max-p95 200ms -max-error-rate 1% -json summary.json
```

### Compare Performance Reports

Save JSON reports from the baseline and current revisions, then compare them. Percentage changes are relative to the baseline; latency and error-rate increases and throughput decreases are treated as regressions. The comparison reports regressions but only exits with status 1 when a configured `-max-regression` limit is exceeded. Usage and report errors exit with status 2. These metrics are estimates, not guarantees, and small changes can be affected by run-to-run variation.

```bash
api-load-tester compare -baseline baseline.json -current current.json -max-regression 10%
```

JSON reports and the terminal summary include endpoint metrics, sorted by descending P95 latency. Query strings and trailing slashes are normalized, known dynamic IDs and `{{counter}}`/`{{random}}` template segments are grouped as `:id`, and at most 32 endpoint buckets are tracked (31 named routes plus the shared `(other)` bucket). Endpoint request totals include cancellations, which are shown separately; error rates exclude cancelled requests.

### Arrival Rate and Ramp

`-rps` schedules arrivals independently of request completion, using deadlines anchored to the scheduler start and catching up after late timer wakeups. Target RPS is capped at 100,000, and catch-up work is bounded per timer wake-up. `-c` caps in-flight requests; when all workers are occupied, arrivals are counted as dropped and included in the error-rate threshold. The final report shows scheduled arrivals per scheduling second and adherence to the expected arrival count (including ramp-up), alongside completed requests/sec. It prints a warning when arrivals are dropped because latency percentiles include only requests that started. Requests still in flight when `-d` expires are recorded as cancelled: their elapsed times remain in latency metrics, but they are excluded from failure counts and the error-rate threshold. With `-csv`, results are queued to a single writer; if the disk is slower than the request rate, the bounded queue can fill and briefly apply backpressure to request workers.

```bash
api-load-tester -url https://api.example.com/health -d 30s -c 100 \
    -rps 200 -warmup 5s -ramp-up 10s -json summary.json \
    -time-series-csv seconds.csv
```

### Status Codes and Redirects

Redirects are not followed: the response status is measured as returned by the target. By default, any status below 400—including 3xx redirects—counts as success. Use `-expect-status` to replace that rule with an explicit comma-separated allowlist; for example, `-expect-status 200,201` treats only 200 and 201 as successful. HTTP/2 negotiation is enabled by default for HTTPS targets and can be disabled with `-http2=false`. Since the default HTTP/2 behavior affects how load is distributed across connections, results from before this change are not directly comparable.

```bash
api-load-tester -url https://api.example.com/items -expect-status 200,201
```

### Dynamic Requests

Repeated `-url` values are selected round-robin. The `{{counter}}` and `{{random}}` tokens are expanded independently for each request in URLs, bodies, and headers. Warm-up sends real requests, but its counter values are reset before measurement, so measured requests start at `1`.

```bash
api-load-tester \
    -url 'https://api.example.com/orders/{{counter}}' \
    -url 'https://api.example.com/items/{{counter}}' \
    -body '{"request":"{{counter}}","id":"{{random}}"}' -n 100
```

### YAML Configuration

CLI flags override values loaded from `-config`. Use `-n` to switch a YAML duration-mode test back to request-count mode; a positive CLI `-d` selects duration mode.

```yaml
urls:
    - https://api.example.com/orders/{{counter}}
    - https://api.example.com/items/{{counter}}
method: POST
body: '{"request":"{{counter}}","id":"{{random}}"}'
concurrency: 50
requests: 1000
rps: 200
warmup: 5s
ramp-up: 10s
max-p95: 200ms
max-error-rate: 1%
expect-status: 200,201
http2: true
```

```bash
api-load-tester -config load-test.yaml -c 100 -json summary.json
```

JSON reports include numeric nanosecond/millisecond duration values and a readable `total_duration_human` string. The JSON `errors` field is an object mapping unwrapped error causes to occurrence counts; older report versions used an array, so consumers that expect that prior shape must be updated. Per-request CSV rows are streamed in request-completion order; the `start_time` column records request initiation time, and rows are not sorted by it. Use `-time-series-csv` for a separate per-second CSV alongside the per-request `-csv` export.

---

## Project Structure

```text
API_LOAD_TESTER/
│
├── cmd/
│   └── main.go
│
├── internal/
│   ├── cli/
│   ├── config/
│   ├── engine/
│   ├── metrics/
│   ├── reporter/
│   ├── exporter/
│   ├── models/
│   └── utils/
│
└── README.md
```

---

## Testing

Run the test suite:

```bash
go test ./...
```

Run with Go's race detector:

```bash
go test -race ./...
```

Run static analysis and check formatting:

```bash
go vet ./...
test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path './vendor/*'))"
```

---

## Project Focus

**Go · Concurrency · Worker Pools · HTTP Networking · Performance Engineering · CLI Development · Observability**

---

## Author

### Nireeksha K R

---

<p align="center">
  <strong>API Load Tester</strong>
  <br>
  Measure how your API performs under pressure.
</p>
