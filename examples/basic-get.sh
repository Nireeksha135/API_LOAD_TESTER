#!/usr/bin/env bash
# Basic GET load test: 1,000 requests across 50 concurrent workers.
set -euo pipefail

api-load-tester \
  -url "https://api.example.com/health" \
  -method GET \
  -c 50 \
  -n 1000 \
  -timeout 10s
