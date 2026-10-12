# API Load Tester

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
- Worker pool architecture
- Fixed request count mode
- Fixed duration mode
- Open-loop target RPS with dropped-arrival accounting
- Unmeasured warm-up and gradual ramp-up
- Per-second RPS, P95 and error time series
- Round-robin URL targets and per-request counter/random tokens
- YAML configuration files
- Custom HTTP headers
- JSON and raw request bodies
- Per-request latency tracking
- Bounded-memory HDR histogram percentiles
- Error counts grouped by message
- Min / Max / Mean latency
- P50 / P90 / P95 / P99 / P99.9 percentiles
- Requests Per Second (RPS)
- Throughput measurement
- HTTP status code breakdown
- Live terminal dashboard
- CSV and JSON export
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

Each worker records results in its own collector. Live dashboard snapshots aggregate those collectors, and the final summary merges their counters and HDR histograms. This keeps request recording contention low while latency memory stays bounded for long duration runs.

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
| Errors | Count of each distinct error message |

---

## Usage

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

Threshold failures leave the final report and requested exports available, then exit with status 1. Error rates accept a fraction or percentage.

```bash
api-load-tester -url https://api.example.com/health -n 1000 -c 50 \
    -max-p95 200ms -max-error-rate 1% -json summary.json
```

### Arrival Rate and Ramp

`-rps` schedules arrivals independently of request completion. `-c` caps in-flight requests; when all workers are occupied, arrivals are counted as dropped and included in the error-rate threshold.

```bash
api-load-tester -url https://api.example.com/health -d 30s -c 100 \
    -rps 200 -warmup 5s -ramp-up 10s -json summary.json \
    -time-series-csv seconds.csv
```

### Dynamic Requests

Repeated `-url` values are selected round-robin. The `{{counter}}` and `{{random}}` tokens are expanded independently for each request in URLs, bodies, and headers.

```bash
api-load-tester \
    -url 'https://api.example.com/orders/{{counter}}' \
    -url 'https://api.example.com/items/{{counter}}' \
    -body '{"request":"{{counter}}","id":"{{random}}"}' -n 100
```

### YAML Configuration

CLI flags override values loaded from `-config`.

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
```

```bash
api-load-tester -config load-test.yaml -c 100 -json summary.json
```

JSON reports include numeric nanosecond/millisecond duration values and a readable `total_duration_human` string. Use `-time-series-csv` for a separate per-second CSV alongside the per-request `-csv` export.

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

Computer Science & Design Engineering student focused on **UX Design, AI, and Full-Stack Development**.

---

<p align="center">
  <strong>API Load Tester</strong>
  <br>
  Measure how your API performs under pressure.
</p>
