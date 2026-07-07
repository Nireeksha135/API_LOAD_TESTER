# API Load Testing Framework

A high-performance API load testing framework built from scratch in **Go**.

This project is designed to demonstrate the implementation of concurrent HTTP request processing, worker pool architecture, networking fundamentals, and performance analysis without relying on existing load testing tools.

> **Project Status:** 🚧 Under Development

---

## Features (Planned)

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

## Project Structure

```text
api-load-tester/
├── cmd/
├── internal/
│   ├── cli/
│   ├── config/
│   ├── engine/
│   ├── exporter/
│   ├── metrics/
│   ├── models/
│   ├── reporter/
│   └── utils/
├── examples/
├── docs/
├── screenshots/
├── README.md
├── LICENSE
├── Makefile
├── go.mod
└── .gitignore
```

---

## Current Progress

* [ ] Project initialization
* [ ] CLI implementation
* [ ] HTTP client
* [ ] Worker pool
* [ ] Concurrent request engine
* [ ] Metrics collector
* [ ] Statistical analysis
* [ ] Terminal reporting
* [ ] CSV export
* [ ] JSON export
* [ ] Unit tests
* [ ] Documentation

---

## Tech Stack

* Go
* Standard Library (`net/http`, `time`, `sync`, `context`)
* Cobra (CLI)
* Bubble Tea *(planned)*
* HDR Histogram *(planned)*

---

## Goals

* Understand how modern API load testing tools work internally.
* Build a scalable concurrent request engine.
* Learn Go concurrency patterns using goroutines and channels.
* Implement accurate latency and throughput analysis.
* Create a professional GitHub portfolio project.

---

## Roadmap

* Build the CLI
* Implement the HTTP request engine
* Add configurable concurrency
* Collect and analyze metrics
* Generate terminal reports
* Support CSV and JSON exports
* Add live terminal dashboard
* Improve performance and documentation

---

## License

This project is licensed under the MIT License.
