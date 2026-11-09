set dotenv-load := false

vps := env("VPS", "")
air := "go run github.com/air-verse/air@v1.67.4"

[private]
default:
    @just --list

# install web deps if missing
install:
    cd web && [ -d node_modules ] || npm ci

# go server on :8080 with hot reload; dev passkey origins for next on :3000
backend:
    cd backend && DEV=1 LOG_LEVEL=debug RP_ORIGINS=http://localhost:3000,http://localhost:8080 \
        DATABASE_PATH=${DATABASE_PATH:-prulights.db} {{ air }}

# next dev server on :3000, passing /api through to the go server
web: install
    cd web && npm run dev

# backend and web together; ctrl-c stops both
start:
    #!/usr/bin/env bash
    trap 'kill 0' EXIT
    just backend & just web & wait

# production-like: static export served by the go server on :8080
serve: install
    cd web && npm run build
    cd backend && STATIC_DIR=../web/out go run ./cmd/prulights serve

# format go and web code
fmt:
    cd backend && gofmt -w .
    cd web && npx prettier --write . --log-level warn

# vet go code
lint:
    cd backend && go vet ./...

# run go tests
test *args:
    cd backend && go test -race {{ args }} ./...

# draw a frame view from sample data to backend/tmp/render-<view>.png
render view="tonight" date="2026-12-12" palette="measured":
    mkdir -p backend/tmp
    cd backend && go run ./cmd/prulights render -view {{ view }} -date {{ date }} -palette {{ palette }} -out tmp/render-{{ view }}.png

# check the frame views against their golden pngs
golden:
    cd backend && go test ./render -run TestGolden

# redraw the golden pngs (look at them before committing)
golden-update:
    cd backend && go test ./render -run TestGolden -update

# generate the web's api types from the go code
types:
    cd backend && go tool tygo generate

# same checks as ci
check: install
    cd backend && test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
    cd backend && go vet ./...
    cd backend && ./types-current.sh
    cd backend && go test -race ./...
    cd web && npm run type-check
    cd web && npm run build

# check the source pages once against the live site; -dry-run is on unless you pass args
scrape *args="-dry-run":
    cd backend && go run ./cmd/prulights scrape {{ args }}

# fetch and apply every source page into the dev database
backfill:
    cd backend && go run ./cmd/prulights backfill

# re-download the saved source pages the scraper tests use (10s apart, as robots.txt asks)
scrape-snapshot:
    #!/usr/bin/env bash
    set -euo pipefail
    cd backend/scrape/testdata
    ua="prulights/1.0 (+https://prulights.evanslack.dev)"
    site=https://www.prudentialcenter.com
    curl -fsSL -A "$ua" -o event-sitemap.xml "$site/event-sitemap.xml"
    for page in 2024-tower-lighting-schedule 2026-tower-lightings 31-nights-of-light; do
        sleep 10
        echo "$page"
        curl -fsSL -A "$ua" -o "$page.html" "$site/experiences/events/$page/"
    done
    cd .. && go test . || echo "the pages changed: review, then go test ./scrape -update"

# open sqlite on the dev database
db:
    sqlite3 backend/${DATABASE_PATH:-prulights.db}

# build the production image locally, for this machine's arch (ci builds amd64 with buildx)
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
    docker build --build-arg BUILDPLATFORM=linux/$arch --build-arg TARGETARCH=$arch -t evanofslack/prulights:dev .

# run the production image locally on :8080 with a throwaway volume
docker-run: docker-build
    docker run --rm -p 8080:8080 -v prulights-dev-data:/data -e BASE_URL=http://localhost:8080 evanofslack/prulights:dev

# pull and restart on the vps (VPS=user@host); see deploy/RUNBOOK.md
deploy:
    @test -n "{{ vps }}" || (echo "set VPS=user@host" && exit 1)
    ssh {{ vps }} 'cd ~/prulights && docker compose pull && docker compose up -d && docker compose ps'

# follow the container logs on the vps
logs:
    @test -n "{{ vps }}" || (echo "set VPS=user@host" && exit 1)
    ssh {{ vps }} 'cd ~/prulights && docker compose logs --follow --tail 200'

# browser checks against a static build on :8099: all, or the named scripts (just e2e public). See e2e/README.md
e2e *scripts: install
    #!/usr/bin/env bash
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
    export DATABASE_PATH="$dir/e2e.db" METRICS_ADDR= SCRAPE_ENABLED=false
    "$dir/prulights" scrape -delay 0 -url http://localhost:8098/2026-tower-lightings.html -url http://localhost:8098/31-nights-of-light.html >/dev/null
    PORT=8099 STATIC_DIR=web/out PUBLIC_RATE_LIMIT=10000 "$dir/prulights" serve >/dev/null 2>&1 &
    server=$!
    trap 'kill $fixtures $server' EXIT
    for _ in $(seq 50); do curl -sf localhost:8099/readyz >/dev/null && break; sleep 0.2; done
    cd e2e && node run.mjs {{ scripts }}
