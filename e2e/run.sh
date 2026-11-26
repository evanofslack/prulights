#!/usr/bin/env bash
# Browser checks against a static build on :8099: all scripts, or the named ones.
# Run from the repo root (just e2e). See e2e/README.md.
set -euo pipefail
! curl -s localhost:8099 >/dev/null || { echo "port 8099 is already in use"; exit 1; }
(cd e2e && [ -d node_modules ] || npm ci)
(cd web && npm run build)
dir="$(mktemp -d)"
(cd backend && go build -o "$dir/prulights" ./cmd/prulights)
# serve the scraper fixtures and load them into a fresh database
python3 -m http.server 8098 -d backend/scrape/testdata >/dev/null 2>&1 &
fixtures=$!
trap 'kill $fixtures' EXIT
for _ in $(seq 50); do curl -sf localhost:8098 >/dev/null && break; sleep 0.2; done
# BASE_URL makes passkeys accept this origin; BIN lets scripts make invites
export DATABASE_PATH="$dir/e2e.db" METRICS_ADDR= SCRAPE_ENABLED=false BASE_URL=http://localhost:8099 BIN="$dir/prulights"
"$dir/prulights" scrape -delay 0 -url http://localhost:8098/2026-tower-lightings.html -url http://localhost:8098/31-nights-of-light.html >/dev/null
DEV=1 PORT=8099 STATIC_DIR=web/out PUBLIC_RATE_LIMIT=10000 AUTH_RATE_LIMIT=10000 SIGNUP_CODE="back bay lights" "$dir/prulights" serve >/dev/null 2>&1 &
server=$!
trap 'kill $fixtures $server' EXIT
for _ in $(seq 50); do curl -sf localhost:8099/readyz >/dev/null && break; sleep 0.2; done
cd e2e && node run.mjs "$@"
