#!/usr/bin/env bash
# Duration-mode load test: run continuously for 2 minutes at a fixed
# concurrency, useful for soak/endurance testing rather than a burst.
set -euo pipefail

api-load-tester \
  -url "https://api.example.com/health" \
  -method GET \
  -c 25 \
  -d 2m \
  -timeout 5s \
  -json soak-test-summary.json
