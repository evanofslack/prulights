# recipes read .env (gitignored) for local settings like SIGNUP_CODE
set dotenv-load := true

vps := env("VPS", "")
air := "go run github.com/air-verse/air@v1.67.4"

[private]
default:
    @just --list

# install web deps if missing
[group('dev')]
install:
    cd web && [ -d node_modules ] || npm ci

# backend and web together; ctrl-c stops both
[group('dev')]
start:
    #!/usr/bin/env bash
    trap 'kill 0' EXIT
    just backend & just web & wait

# go server on :8080 with hot reload; dev passkey origins for next on :3000
[group('dev')]
backend:
    cd backend && DEV=1 LOG_LEVEL=debug RP_ORIGINS=http://localhost:3000,http://localhost:8080 \
        DATABASE_PATH=${DATABASE_PATH:-prulights.db} {{ air }}

# next dev server on :3000, passing /api through to the go server
[group('dev')]
web: install
    cd web && npm run dev

# production-like: static export served by the go server on :8080
[group('dev')]
serve: install
    cd web && npm run build
    cd backend && STATIC_DIR=../web/out go run ./cmd/prulights serve

# open sqlite on the dev database
[group('dev')]
db:
    sqlite3 backend/${DATABASE_PATH:-prulights.db}

# made-up people's sightings in the dev database: just seed -names Logan,Shiv -reset
[group('dev')]
seed *args:
    cd backend && DEV=1 DATABASE_PATH=${DATABASE_PATH:-prulights.db} go run ./cmd/prulights seed {{ args }}

# make an invite link and QR code on the dev database: just invite -admin, just invite -for-user Mom
[group('dev')]
invite *args:
    cd backend && go run ./cmd/prulights invite {{ args }}

# same checks as ci
[group('check')]
check: install
    cd backend && test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
    cd backend && go vet ./...
    cd backend && ./types-current.sh
    cd backend && go test -race ./...
    cd web && npm run type-check
    cd web && npm run build

# format go and web code
[group('check')]
fmt:
    cd backend && gofmt -w .
    cd web && npx prettier --write . --log-level warn

# run go tests
[group('check')]
test *args:
    cd backend && go test -race {{ args }} ./...

# generate the web's api types from the go code
[group('check')]
types:
    cd backend && go tool tygo generate

# check the frame views against their golden pngs
[group('check')]
golden:
    cd backend && go test ./render -run TestGolden

# redraw the golden pngs (look at them before committing)
[group('check')]
golden-update:
    cd backend && go test ./render -run TestGolden -update

# browser checks against a static build on :8099: all, or the named scripts (just e2e public)
[group('check')]
e2e *scripts: install
    ./e2e/run.sh {{ scripts }}

# check the source pages once against the live site; -dry-run is on unless you pass args
[group('scrape')]
scrape *args="-dry-run":
    cd backend && go run ./cmd/prulights scrape {{ args }}

# fetch and apply every source page into the dev database
[group('scrape')]
backfill:
    cd backend && go run ./cmd/prulights backfill

# re-download the saved source pages the scraper tests use
[group('scrape')]
scrape-snapshot:
    ./backend/scrape/snapshot.sh

# draw a frame view from sample data to backend/tmp/render-<view>.png
[group('frames')]
render view="tonight" date="2026-12-12" palette="measured" *args="":
    mkdir -p backend/tmp
    cd backend && go run ./cmd/prulights render -view {{ view }} -date {{ date }} -palette {{ palette }} {{ args }} -out tmp/render-{{ view }}.png

# prepare a frame's identity (in prod: docker compose exec prulights prulights frame new -out frame.json)
[group('frames')]
frame-new *args:
    cd backend && go run ./cmd/prulights frame new {{ args }}

# (plans/firmware.md) e.g. just frame-config -from frame.json -wifi "Home:password" -wifi "Cape:password" -out sd
# the files to flash, with Wi-Fi that never reaches the server
[group('frames')]
frame-config *args:
    cd backend && go run ./cmd/prulights frame config {{ args }}

# build the production image locally, for this machine's arch (ci builds amd64 with buildx)
[group('ops')]
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
    docker build --build-arg BUILDPLATFORM=linux/$arch --build-arg TARGETARCH=$arch -t evanofslack/prulights:dev .

# run the production image locally on :8080 with a throwaway volume
[group('ops')]
docker-run: docker-build
    docker run --rm -p 8080:8080 -v prulights-dev-data:/data -e BASE_URL=http://localhost:8080 -e SIGNUP_CODE evanofslack/prulights:dev

# pull and restart on the vps (VPS=user@host); see deploy/RUNBOOK.md
[group('ops')]
deploy:
    @test -n "{{ vps }}" || (echo "set VPS=user@host" && exit 1)
    ssh {{ vps }} 'cd ~/prulights && docker compose pull && docker compose up -d && docker compose ps'

# copy the live database from the vps to backups/prulights-<date>.db
[group('ops')]
backup:
    @test -n "{{ vps }}" || (echo "set VPS=user@host" && exit 1)
    mkdir -p backups
    ssh {{ vps }} 'cd ~/prulights && rm -f backup.db && docker compose exec -T prulights ./prulights backup -out /data/backup.db && docker compose cp prulights:/data/backup.db backup.db && docker compose exec -T prulights rm /data/backup.db'
    scp {{ vps }}:prulights/backup.db backups/prulights-$(date +%F-%H%M).db
    ssh {{ vps }} 'rm ~/prulights/backup.db'

# follow the container logs on the vps
[group('ops')]
logs:
    @test -n "{{ vps }}" || (echo "set VPS=user@host" && exit 1)
    ssh {{ vps }} 'cd ~/prulights && docker compose logs --follow --tail 200'
