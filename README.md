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
- Custom HTTP headers
- JSON and raw request bodies
- Per-request latency tracking
- Min / Max / Mean latency
- P50 / P95 / P99 percentiles
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

The worker pool distributes requests across concurrent workers while a thread-safe metrics collector tracks latency, throughput, request rates, and HTTP responses.

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
