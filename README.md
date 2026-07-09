# API Load Testing Framework

A high-performance API load testing framework built from scratch in **Go**.

This project is designed to demonstrate the implementation of concurrent HTTP request processing, worker pool architecture, networking fundamentals, and performance analysis without relying on existing load testing tools.

---

## Features 

* CLI-based interface
* Configurable concurrency
* Request count and duration modes
* GET, POST, PUT, DELETE support
* Custom headers
* JSON request body
* Worker pool architecture
* Goroutines and channels
* Thread-safe metrics collection
* HTTP status code tracking
* Latency measurement
* Min, Max, Mean latency
* P50, P95, P99 percentiles
* Requests Per Second (RPS)
* Throughput calculation
* CSV export
* JSON export
* Clean terminal summary
* Graceful shutdown
* Unit tests

---

## Tech Stack

* Go
* Standard Library (`net/http`, `time`, `sync`, `context`)
* Cobra (CLI)
* Bubble Tea *(planned)*
* HDR Histogram *(planned)*

---

# api-load-tester

A concurrent HTTP API load testing tool written in Go — zero third-party dependencies.

## Features

- CLI-driven: GET / POST / PUT / DELETE / PATCH / HEAD
- Custom headers, JSON/raw request bodies (inline or from file)
- Configurable concurrency via a worker pool
- Fixed request count **or** fixed duration mode
- Thread-safe metrics: min/max/mean, P50/P95/P99 latency
- Requests/sec, throughput, status code breakdown
- Live terminal dashboard + clean final report
- CSV (per-request) and JSON (summary) export
- Graceful shutdown on `Ctrl+C` / SIGTERM

## Install

```bash
git clone https://github.com/Nireeksha135/API_LOAD_TESTER.git
cd API_LOAD_TESTER
go build -o bin/api-load-tester ./cmd
```

## Usage

api-load-tester -url <target> [flags]

**Basic:**
```bash
api-load-tester -url https://api.example.com/health -c 50 -n 1000
```

**POST + headers + duration mode + export:**
```bash
api-load-tester -url https://api.example.com/users -method POST \
    -body '{"name":"load-test"}' \
    -H "Authorization: Bearer TOKEN" \
    -d 30s -c 100 \
    -csv results.csv -json summary.json
```

## Flags

| Flag       | Default | Description                              |
|------------|---------|-------------------------------------------|
| `-url`     | —       | Target URL (required)                     |
| `-method`  | `GET`   | HTTP method                               |
| `-body`    | —       | Request body                              |
| `-body-file` | —     | Read body from a file                     |
| `-H`       | —       | Custom header, repeatable                 |
| `-c`       | `10`    | Concurrency                               |
| `-n`       | `100`   | Total requests (ignored if `-d` is set)   |
| `-d`       | —       | Run for a fixed duration, e.g. `30s`      |
| `-timeout` | `30s`   | Per-request timeout                       |
| `-insecure`| `false` | Skip TLS verification                     |
| `-csv`     | —       | Export per-request CSV                    |
| `-json`    | —       | Export summary JSON                       |
| `-v`       | `false` | Verbose logging                           |

## Project Structure
cmd/main.go       Entrypoint, signal handling
internal/
cli/            Flag parsing + orchestration
config/         Config + validation
engine/         Worker pool, HTTP client, dispatch
metrics/        Thread-safe collector + percentiles
reporter/       Live dashboard + final report
exporter/       CSV / JSON export
models/         Shared data structures
utils/          Formatting + logger helpers

## Testing

```bash
go test ./...
go test -race ./...
```

## License

[MIT](./LICENSE)
