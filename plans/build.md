# prulights: build everything that doesn't need the hardware

## Context
Mom watches the Prudential Tower's crown change color from her Back Bay window. We're building
`prulights`. It has four parts:
- a scraper and **public API** for the lighting schedule, with an OpenAPI spec
- a **web app** that works on a phone, a desktop and an always-on display, with passkey login and her sightings and ratings
- **server-side drawing** of the e-ink views
- an **emulator of the e-ink frame** (Waveshare PhotoPainter, 7.3" Spectra 6, portrait 480×800), so the frame's views can be refined before the hardware arrives

Firmware is out of scope for now. The design doc is `plans/prulights.md` (already written). This file
is the plan for carrying it out.

Decisions locked this round:
- Domain `prulights.evanslack.dev` (DNS on Cloudflare).
- The tower **never goes dark**: nights with nothing listed are white.
- Lights-on time is a time stated by the source (31 Nights says 5pm), otherwise computed sunset, otherwise 5pm.
- Publish and serve an OpenAPI spec.
- `git init` in the current `prudential/` folder; **public** repo `evanofslack/prulights`.
- I open stacked PRs and **you review and merge**.
- **I write the deploy configs and a runbook; you deploy** (Docker Hub secrets, Cloudflare DNS, first `just deploy`).

## Conventions (copied from go-poker, `/Users/eslack/Desktop/personal/projects/go-poker`)
- **Backend:**
  - `backend/` Go module `github.com/evanofslack/prulights` (Go 1.27)
  - chi with slog logging
  - config from environment variables via the `envInt`/`envDuration`/`envBool` helpers (copy `backend/server/config.go`)
  - `signal.NotifyContext` main with graceful shutdown (copy `backend/cmd/go-poker/main.go`)
- **Storage:** `modernc.org/sqlite` with WAL and busy_timeout, one connection that writes. Embedded numbered migrations run by a version table (copy `openStore`/`migrate` from `backend/server/sqlstore.go`, SQLite only).
- **Static files:** the static export is served from `STATIC_DIR`, falling back to `index.html` (copy `mountStatic` from `backend/server/server.go`). `/api/` returns 404 when nothing matches.
- **Health and metrics:** `/healthz`, `/readyz`. Prometheus on a separate `METRICS_ADDR` (pattern from `backend/server/metrics.go`: fixed label sets, every series starting at 0).
- **Types:** `go tool tygo generate` writes `web/interfaces/*.ts`. `types-current.sh` makes CI fail when they're stale.
- **Web:** Next (latest) `output: 'export'`, App Router, Mantine (latest) plus Tailwind (latest, config in CSS), tokens in `styles/index.css` and a Mantine theme in `lib/theme.ts`. No SSR or API routes. Dynamic pages use query parameters (`/night?d=`).
- **Docker:** go-poker's three-stage Dockerfile (node → `CGO_ENABLED=0` go → alpine with `ca-certificates`/`tzdata`, non-root user).
- **CI:** `ci.yml` (gofmt, vet, types up to date, `go test -race`, golden images, web type-check and build) and `publish.yml` copied as-is (Docker Hub, `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN`).
- **e2e:** `e2e/` Playwright `.mjs` scripts, run with `just e2e` (not in CI), as in go-poker.
- **Git:**
  - commits: 3–5 lowercase imperative words
  - PRs: 4–6 word titles, 1–2 sentence descriptions ending with the Claude Code attribution line
  - each PR is a branch in a `gh stack` stack on top of `main`

## PR stack (gh stack, each reviewable on its own)
`main` starts from a single bootstrap commit (`plans/`, README, MIT LICENSE, `.gitignore`), pushed
when the repo is created so the stack has a base.

1. **`skeleton`: "Add server, web shell, CI"**
   - Go server: config, static serving, health, metrics, SQLite store with migration runner.
   - Next shell: Mantine/Tailwind theme, layout, dark default.
   - justfile; Dockerfile; `ci.yml`/`publish.yml`; `docker-compose.yaml` (image, `/data` volume, Caddy network); `deploy/Caddyfile.snippet`; `deploy/RUNBOOK.md` (Cloudflare A record, Full-strict SSL, secrets, first deploy).
   - `CLAUDE.md`; `.air.toml` (`go tool air`, so the hot-reload tool is pinned in go.mod).
2. **`schedule`: "Add schedule domain and palette"**
   - Pure package `backend/schedule`:
     - `Lighting`, `Color` and the palette: 14 keys including black/silver/yellow/rainbow/patriotic, each with screen hex, an e-ink recipe, and aliases
     - `Tonight(now)`, rolling over at 04:00 America/New_York
     - `LightsOn(date, sourceTime)`: source time, else sunset (NOAA formula implemented in-package, tested against known Boston sunsets), else 17:00
     - default-white filling of nights with nothing listed
   - Table-driven tests.
3. **`scraper`: "Scrape Pru lighting schedule pages"**
   - `backend/scrape`:
     - sitemap discovery (`event-sitemap.xml`, `/tower-light|nights-of-light/`, plus seeded known addresses such as 2025's)
     - conditional GET, ≥10 s apart (robots `Crawl-delay`), UA `prulights/1.0 (+https://prulights.evanslack.dev)`
     - two goquery parsers: yearly `<li><strong>date</strong> | colors for cause` and 31 Nights `<li>date | <a href>cause</a> | colors`
     - normalizer: date ranges, `&`/`and`/Oxford commas, connecting phrases, unknown color becomes a warning
     - diff and upsert in one transaction (`lightings`, soft `removed_at`), `scrape_runs`
     - guard that keeps the last good data when a page parses 0 entries or the count drops more than 50%
   - Runs on a jittered 6h ticker in `serve`; subcommands `prulights scrape [--dry-run]` and `prulights backfill`.
   - Fixtures in `testdata/` (2024, 2026, 31 Nights pages) with golden JSON. `just scrape-snapshot` re-downloads them.
4. **`public-api`: "Add public schedule API"**
   - `/api/v1`: `tonight`, `lightings?from&to&fill`, `lightings/{date}`, `palette`, `meta` (sources, last scrape, `published_through`), `calendar.ics`.
   - ETag/`Cache-Control`; httprate 60/min per IP using the real client address (`CF-Connecting-IP`/`X-Forwarded-For` only when `TRUST_PROXY`); CORS open for GETs.
   - Hand-written `backend/server/openapi.yaml` (embedded), served at `/api/v1/openapi.{yaml,json}`. A test walks the chi routes (`chi.Walk`) and fails if a public route is missing from the spec.
   - tygo types.
5. **`web-public`: "Add tonight, calendar, display pages"**
   - `/`: tower SVG with the crown glowing in screen colors (bands for several colors), the cause, a link to the charity, lights-on time, the next 7 nights.
   - `/night?d=`; `/calendar` (month grid).
   - `/display`: always-on; rolls over at 04:00, re-fetches every 5 min, Wake Lock, hidden cursor, slow drift against burn-in.
   - `/docs`: Scalar API reference reading `/api/v1/openapi.json`.
   - Footer credit linking to prudentialcenter.com.
   - e2e smoke script.
6. **`auth`: "Add passkey login and invites"**
   - `go-webauthn/webauthn`: RP ID taken from the environment (`localhost` in dev with `:3000`/`:8080` origins, `prulights.evanslack.dev` in prod), discoverable credentials.
   - Tables `users`/`credentials`/`sessions`/`invites` (hashed tokens, 1-year sliding session, HttpOnly Secure SameSite=Lax).
   - Changes require a JSON content type and a same-origin check. Auth endpoints are rate-limited.
   - Admin invite API. `prulights invite [--admin] [--for-user]` prints the link and a terminal QR code (`mdp/qrterminal`).
   - Web: `/invite?t=`, `/login`, account menu.
   - e2e uses Playwright's virtual authenticator (CDP `WebAuthn.addVirtualAuthenticator`).
7. **`sightings`: "Add sightings, ratings, and admin"**
   - API:
     - `PUT/DELETE /me/sightings/{date}` `{seen, rating 1–5, note ≤280}`, where rating a night marks it seen
     - `GET /me/sightings`, `/me/stats` (count, streak, favorite colors)
     - public `lightings/{date}/stats` (totals only)
     - admin overrides (corrections only), scrape runs, scrape now, users
   - Web: the "I saw it" button, ★ rating and note on `/` and `/night`; `/log`; `/admin`.
   - Spec and types updated.
8. **`renderer`: "Render e-ink views in Go"**
   - `backend/render`:
     - 480×800 canvas
     - two kinds of pixels: solid (text thresholded, outlines, sky) and flagged fill regions dithered with ordered Bayer 8×8 using the palette recipes
     - Spectra 6 ideal and *measured* palettes (measured values taken from aitjcize/esp32-photoframe's calibration, with credit in comments)
     - outputs: packed 4 bits per pixel, rotated to 800×480 and gzipped (`.bin.gz`), plus preview PNGs in measured and ideal colors
     - embedded Atkinson Hyperlegible font, `golang.org/x/image/{font,opentype,vector}`
     - tower artwork: a black-and-white PNG plus a crown mask, banded fill
     - `skip2/go-qrcode`
   - Views: `tonight`, `week`, `month`, `log`, `setup`, `goodnight`, plus stale-data and low-battery overlays.
   - Dev endpoint `GET /dev/render/{view}?date&user&palette` (only when `DEV=1`); `prulights render`; `just render`.
   - Golden PNG tests: `just golden`, `just golden-update`.
9. **`device-api`: "Add device API and bundles"**
   - `/api/device/v1`:
     - `register` → token and claim code
     - `state` (bundle version, next check-in, server time, settings)
     - `bundle?have=` (manifest of 7 days × enabled views, by sha256)
     - `images/{sha}.bin.gz` (cached forever)
     - `events` (gestures, seen, boot; battery/power/firmware). A `seen` event writes a sighting for the device's owner.
   - Bundle version is a hash of the schedule window, the owner's sightings, the settings and the renderer version.
   - `/api/device/compat/current.png` for the stock aitjcize firmware later.
   - User side: `POST /devices/claim`, `GET/PATCH/DELETE /me/devices`.
   - Tables `devices`/`device_events`.
10. **`emulator`: "Add frame emulator and settings"**
    - `/emulator`, a virtual device that uses only the device API:
      - registration, the claim QR and claiming
      - frame graphic at true size (~128 ppi) with measured, ideal or raw pixels, plus a zoom inspector
      - KEY short (next saved view, no network), long (seen → sync), double (sync)
      - simulated ~20 s redraw flash
      - controls: clock override / time travel, USB or battery with a level slider, offline mode
      - event log and an estimate of battery use per day
    - `/claim?code=` and `/settings` (default view, which views the button cycles, quiet hours, pause; "applies on next sync").
    - e2e: register → claim → long-press → the sighting appears in `/log`.

**Running work in parallel:** the stack is linear for review. Once #2 exists, **#8 (renderer) is built in a separate worktree**, branched from `schedule` and run by a fork agent, while I work through #3–#7. It then goes into its place in the stack (`gh stack` rebase). Everything else is built in order in the main worktree.

## Data notes (verified live, 2026-11-08)
- robots: everything allowed, `Crawl-delay: 10`.
- `event-sitemap.xml` lastmods: `2024-tower-lighting-schedule`, `2026-tower-lightings`, `31-nights-of-light`.
- The 2025 page isn't in the sitemap, so seed it.
- `/wp-json` doesn't expose the events, so scraping HTML is the only route.

## justfile (main recipes)
`install`, `backend` (air), `web`, `start`, `serve`, `fmt`, `lint`, `test`, `types`, `check` (= CI),
`golden`/`golden-update`, `render`, `scrape`, `scrape-snapshot`, `backfill`, `invite`, `db`,
`docker-build`, `e2e`, `deploy` (`ssh $VPS 'cd ~/prulights && docker compose pull && docker compose up -d'`), `logs`.

## Verification
- **Every PR:** `just check` passes locally and in CI.
- **Scraper:** fixture/golden tests; `just scrape` (dry run against the live site) prints the parsed 2026 schedule and 31 Nights with zero warnings; `just backfill` fills `prulights.db`.
- **API:** with `just start`, check `curl :8080/api/v1/tonight`, `lightings?from=2026-12-01&to=2026-12-31&fill=true` (31 nights, Dec 10 = Red, Dress for Success), a night with nothing listed returning white with `scheduled:false`, `calendar.ics` importing into Apple Calendar, `openapi.json` passing validation, and the spec/route test.
- **Web:** `just e2e` covers tonight, calendar, display and docs; login with a virtual authenticator → rate → `/log`; emulator register → claim → gestures. Also look at it in a phone-sized viewport.
- **Renderer:** golden PNGs, plus a manual look at `/dev/render/tonight?date=2026-12-12` in both palettes.
- **Container:** `just docker-build` and run it locally with a volume. The scraper runs, the static site and API are served, and `/readyz` reports scrape age.
- **Hand-off:** you add the Docker Hub secrets, the Cloudflare A record and the Caddy snippet, then run `just deploy` following `deploy/RUNBOOK.md`, then `prulights invite --admin`.
