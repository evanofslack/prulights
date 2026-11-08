# prulights — plan

The Pru's colors for Mom: a public API for the Prudential Tower lighting schedule, a web app
(phone, desktop, an always-on display, and an emulator of the e-ink frame), server-drawn e-ink
images, and later the firmware for a Waveshare ESP32-S3 PhotoPainter (7.3" Spectra 6) on her desk.

Status: planning (2026-11-08). No hardware in hand yet; firmware is the last phase.

## Decisions

| Area | Decision |
|---|---|
| Name | `prulights`: repo `github.com/evanofslack/prulights`, image `evanofslack/prulights`, `prulights.<your domain>` |
| Backend | Go: one binary that runs the API, scraper, renderer, auth and static file serving. chi router (as in go-poker). |
| Scraper | Go, in the same binary (goquery). No Python. |
| DB | SQLite with modernc (pure Go, CGO off). Migrations are numbered files and never edited, as in go-poker. |
| Web | Next.js (latest) static export served by Go from `STATIC_DIR`. Mantine (latest) for controls, Tailwind (latest) for layout, tygo to generate TypeScript types from the Go structs. No SSR, API routes or middleware. |
| Auth | WebAuthn passkeys plus single-use invite links (as QR codes). Invite-only. No email, ever. |
| Public API | Open, no key, rate-limited per IP, cached (ETag/Cache-Control), CORS open for GETs. |
| Ratings | 1–5 stars plus an optional short note (280 characters). Rating a night marks it seen. One sighting per user per night, editable. |
| Notifications | None: no push, email or text. The QR code on the frame opens a mobile web page. |
| Display | A responsive desktop layout **and** an always-on `/display` route. |
| Renderer | Pure Go drawing → dither → pack into the panel's format. The emulator shows the same images. |
| Frame | Waveshare ESP32-S3 PhotoPainter, **portrait 480×800**, on battery (USB is a bonus). Firmware in C on ESP-IDF via `idf.py`, forked from `aitjcize/esp32-photoframe`. |
| Default color | **White** on any night with no listed lighting (`scheduled:false`). |
| Ops | Docker Compose + Caddy on the amd64 VPS. GitHub Actions publish to Docker Hub (copy go-poker's `publish.yml`). The VPS runs `docker compose pull && up -d`. |
| Commands | A `justfile` (submodules `mod web` / `mod firmware` later if useful). |

## Repo layout

```
prulights/
  justfile                  # top-level recipes (see Justfile)
  Dockerfile                # node build → go build → alpine, like go-poker
  docker-compose.yaml       # prod: image + /data volume + caddy network
  docker-compose-dev.yaml   # optional local container run
  CLAUDE.md
  plans/
  .github/workflows/
    ci.yml                  # = just check
    publish.yml             # go-poker's Docker Hub workflow
  backend/
    cmd/prulights/main.go   # subcommands: serve | scrape | render | invite | migrate
    schedule/               # domain: Lighting, Color, palette, "tonight" resolution
    scrape/                 # sitemap discovery, fetch, two parsers, normalize, diff
      testdata/             # saved HTML snapshots (2024, 2026, 31 nights) + golden JSON
    store/                  # sqlite, migrations/, queries
    server/                 # chi routes: public, me, admin, device, static, /metrics
    auth/                   # webauthn, sessions, invites
    render/                 # views → image → dither → pack; fonts + art embedded
      testdata/golden/      # golden PNGs
    device/                 # device state, bundle building, events
    tygo.yaml
  web/                      # Next static export
  firmware/                 # later: ESP-IDF fork of aitjcize/esp32-photoframe
```

## Data source (verified 2026-11-08)

- `robots.txt` allows everything except the form uploads folder, and asks for **`Crawl-delay: 10`** (wait at least 10 seconds between requests).
- **Discovery:** the Yoast sitemap `https://www.prudentialcenter.com/event-sitemap.xml` lists the
  lighting pages with `<lastmod>`. Page addresses vary by year (`2024-tower-lighting-schedule`,
  `2026-tower-lightings`, `31-nights-of-light`), so match on `/tower-light|nights-of-light/` and
  only re-fetch a page when its `lastmod` changes. The 2025 page is not in the sitemap (it was
  found earlier at `/experiences/events/2025-tower-lightings`), so seed known addresses as well.
- **There is no usable WordPress API.** `/wp-json` is up, but the events post type isn't exposed through it.
- **Two formats:**
  - Yearly page: `<li><strong>November 11 </strong>| Red, White &amp; Blue for Veterans Day</li>`,
    under `<h3>` sections "Upcoming" and "Past".
  - 31 Nights: `<li>December 10 | <a href="http://…">Dress for Success</a> | Red</li>`
    (date | cause with a link | colors). It includes the **charity's website**.
- **Quirks to handle:**
  - Date ranges ("April 18–20", and possibly across months)
  - Separators: `&`, `and`, commas, Oxford commas ("Blue, Pink, &amp; Gold")
  - How the cause is introduced: "for", "in support of", "in recognition of", "in honor of", or nothing
  - Stray spaces inside tags
  - Typos
  - The year comes from the page's title or address
- **More colors than the application form lists:** black, silver and yellow appear on sports nights
  (Bruins, Patriots, Marathon). The palette has to be open-ended: an unknown color is kept as its
  raw text with a warning, never dropped.

## Domain model (SQLite)

- `lightings`
  - Identity: `id`, `date` (Boston local date), `program` (`regular` | `31nights`)
  - Content: `colors` (JSON list of palette keys), `colors_raw`, `cause`, `cause_url`, `kind` (`nonprofit` | `holiday` | `sports` | `civic`)
  - Source: `source_url`, `source_text` (the raw `<li>`)
  - Tracking: `first_seen_at`, `last_seen_at`, `removed_at` (if an entry disappears from the page, mark it, never delete it), `override_id`
- `overrides`: admin corrections, surprise lightings, or "dark" nights. They win over scraped data.
- `palette` (code, not a table): `key`, `label`, screen hex, e-ink recipe (which pigments to dither and in what proportion), and display rule.
  - Patriotic is red/white/blue bands; rainbow is 6 bands.
  - Keys: white, red, blue, green, gold, yellow, orange, pink, purple, teal, maroon, black, silver, rainbow.
- `scrape_runs`: `started_at`, `url`, `status`, `lastmod`, `etag`, `items`, `added`, `changed`, `removed`, `warnings` (JSON), `error`.
- `users`: `id`, `name`, `role` (`admin` | `member`), `created_at`.
- `credentials`: WebAuthn credential id, public key, sign count, transports, `user_id`, `nickname`.
- `sessions`: hash of the token, `user_id`, `expires_at` (sliding, 1 year), `user_agent`, `last_seen_at`.
- `invites`: hash of the token, `role`, `for_user_id` (null means a new user; set means "add a passkey" for recovery), `expires_at` (7 days), `used_at`.
- `sightings`: `user_id`, `date`, `seen_at`, `rating` (1–5, nullable), `note`, `source` (`web` | `device`), `updated_at`. Unique on (`user_id`, `date`).
- `devices`
  - Identity: `id`, hash of the token, `user_id` (who claimed it), `name`, `claim_code`, `claimed_at`
  - Settings (JSON): default view, which views the button cycles through, quiet hours, paused
  - Last reported: `last_checkin_at`, `battery_mv`, `power` (`usb` | `battery`), `fw_version`
- `device_events`: `device_id`, `type`, `at`, `payload`. Events arrive in batches and may be delayed, sent after the frame was offline.

**What "tonight" means:** the date of the evening in America/New_York, rolling over at **04:00** so a
1am glance still shows tonight. The tower lights at sunset, or 5pm during 31 Nights. Compute sunset
for Boston in Go so the app can say "lights on ~4:21pm."

**Nights with no listed lighting:** the API generates them on the fly as
`{scheduled:false, colors:["white"], kind:"default"}`. Responses also include
`published_through`, the last date the source has published, so clients can tell a "default white"
night from a night that simply hasn't been announced yet.

## Scraper

- Runs inside the server on a jittered ticker (every 6h by default), plus `prulights scrape [--dry-run] [--url]`.
- Steps:
  1. Fetch the sitemap.
  2. Fetch only pages whose `lastmod` changed (≥10 s apart, conditional GET).
  3. Run the matching parser.
  4. Normalize.
  5. Compare with the database and write the changes in one transaction.
- **User-Agent:** `prulights/1.0 (+https://prulights.<domain>)`. A contact URL, no email.
- **Safety checks:**
  - If a page parses 0 entries, or the count drops more than 50% from the last run, keep the last good data and record a warning.
  - Unknown color words count as warnings.
  - Report scrape age on `/readyz` and in Prometheus metrics.
  - The admin page lists warnings.
- **Backfill:** a one-time import of 2024, 2025 and 2026 (plus older 31 Nights pages if reachable), so the calendar and stats have history.
- **Tests:** saved HTML snapshots in `testdata/` compared against golden JSON. `just scrape-snapshot` re-downloads the snapshots so changes on the live site can be diffed.

## API

Everything under `/api/v1`. JSON only. tygo generates the TypeScript types, and CI fails if they're out of date.

**Public (no login):**
- `GET /tonight`: tonight's lighting plus the lights-on time.
- `GET /lightings?from&to&fill=true`: at most 400 days. `fill` adds the default-white nights.
- `GET /lightings/{date}`, `GET /lightings/{date}/stats` (average, number of ratings, number of sightings; never individuals' data).
- `GET /palette`, `GET /meta` (sources, last scrape, `published_through`).
- `GET /calendar.ics`: a feed anyone can subscribe to.
- Caching: `ETag` plus `Cache-Control: public, max-age=300`. Rate limit 60 requests/min per IP (httprate).

**Logged in (session cookie):**
- Auth:
  - `POST /auth/register/{begin,finish}` (requires an invite token)
  - `POST /auth/login/{begin,finish}` (discoverable credentials, so no username)
  - `POST /auth/logout`
- `GET /me`
- `PUT /me/sightings/{date}` `{seen, rating?, note?}`, `DELETE /me/sightings/{date}`
- `GET /me/sightings?from&to`, `GET /me/stats` (count, streaks, favorite colors, average by color/kind)
- `POST /devices/claim` `{code}`, `GET /me/devices`, `PATCH /me/devices/{id}` (settings), `DELETE`.

**Admin only:**
- `POST /admin/invites`, `GET` / `DELETE`
- `GET /admin/scrape-runs`, `POST /admin/scrape`
- overrides (full create/read/update/delete)
- `GET /admin/users`

**Device (`/api/device/v1`, bearer device token).** Also used by the emulator.
- `POST /register` → `{device_id, token, claim_code, claim_url}`. The frame shows a QR code linking to `claim_url`.
- `GET /state` → `{claimed, bundle_version, next_checkin_s, server_time, tz, settings}`. Includes the time so the frame can set its clock.
- `GET /bundle?have=<version>` → a manifest of `{date, view, sha256, size, url}` for the **next 7 days × each enabled view** (plus the setup, good-night and stale-data screens).
  - The frame downloads only images whose hash changed.
  - Each image is 480×800 at 4 bits per pixel, about 192 KB raw and far smaller compressed with gzip, since the art is flat.
- `GET /images/{sha}.bin.gz`: never changes, so it can be cached forever.
- `POST /events` `{events:[{type, at, date?}], battery_mv, power, fw}`. Types include the gestures, `seen`, `boot`, `error`.
- **Bundle version:** a hash of the schedule rows in the window, this user's sightings for those dates, the device settings, and the renderer version. Any change to these produces a new version.
- **Compatibility endpoint for the first hardware test:** `GET /api/device/compat/current.png?token=`, serving the image in a format the unmodified aitjcize firmware can show (it fetches from a URL, sends an `Authorization` header and supports ETags).

**Security:**
- Sessions are stored as hashes in an `HttpOnly; Secure; SameSite=Lax` cookie.
- Changes are only accepted from the app's own origin, with a JSON content type.
- Device tokens are stored hashed and are revocable.
- Rate limits on the auth endpoints.

## Auth flow

- **First admin:** run `prulights invite --admin` (via `docker compose exec`). It prints a link and a QR code in the terminal; open it on your phone and register a passkey.
- **Your mom:** the admin page creates an invite and shows a QR code. She scans it and registers a passkey with Face ID. She then stays logged in with a cookie that renews itself.
- **New phone or recovery:** the admin creates an "add a passkey" invite tied to her user.
- **Dev:** WebAuthn works on `localhost`. The relying-party ID is `localhost`, with allowed origins `http://localhost:3000` (Next dev) and `:8080` (Go). In prod the ID is `prulights.<domain>`. **Lock the domain in before inviting her**, because passkeys are tied to it.

## Renderer (Go)

- **Canvas:** draw in portrait at 480×800, rotate to the panel's native 800×480 when packing.
- **Two kinds of pixels:**
  - **Solid:** text, outlines, the sky and black/white areas are drawn in exact pigment colors. Text is drawn with anti-aliasing and then thresholded, so it's never dithered.
  - **Fill:** a region filled with a target color, flagged in a mask. Only these regions are dithered, using an **ordered (Bayer 8×8) dither between the nearest pigments** from the palette's recipe.
- **Three outputs:**
  1. `.bin.gz`: packed in the panel's native codes.
  2. Preview PNG using the *measured* panel colors (aitjcize's calibration), so the emulator shows the real, duller look.
  3. Design PNG using the ideal colors.
- **Libraries:**
  - Go standard library `image`
  - `golang.org/x/image/{font,opentype,vector}`
  - embedded fonts: Atkinson Hyperlegible for readability, plus a display font for headers
  - `skip2/go-qrcode` for QR codes
- **Tower art:** a hand-made black-and-white PNG plus a crown mask. The crown is filled with the
  colors as bands (two colors split it, rainbow uses 6 bands). Swapping the artwork never touches code.
- **Views (portrait):**
  - `tonight`: the main view, with the art, the colors, the cause, ✓/★, the next 3 nights and a QR code
  - `week`: 7 color swatches with names
  - `month`: a calendar grid of colors
  - `log`: her counts, favorite colors and recent ratings
  - system screens: setup (QR to link the frame), good night (quiet hours), stale-data banner, low-battery icon
- **Iteration loop:** `GET /dev/render/{view}?date=&user=&palette=measured|ideal` (dev only), plus
  `just render view=tonight date=2026-12-12` writing a PNG, and `air` to rebuild Go on save. This keeps
  Go layout work quick despite having no HTML.
- **Golden-image tests:** `just golden` checks the output, `just golden-update` accepts new output after reviewing the diff.

## Web app (Next static export)

Static export can't generate pages for dynamic paths, so routes take query parameters (`/night?d=2026-12-12`).

| Route | Purpose |
|---|---|
| `/` | Tonight: a big tower outline with the crown glowing in screen colors, then the colors, cause, charity link and lights-on time. When logged in: **I saw it**, ★1–5, note. Then the next 7 nights. |
| `/night?d=` | The same for any date. **The QR code on the frame points here.** |
| `/calendar` | A month grid of colors. December is the showcase. |
| `/log` | Her sightings, ratings and stats. |
| `/settings` | Frame settings (default view, which views the button cycles, quiet hours, pause) and her passkeys. Says "applies on the frame's next sync, or double-press to sync now." |
| `/claim?code=` | Link a frame to the account. |
| `/invite?t=`, `/login` | Passkey registration and sign-in. |
| `/admin` | Invites, overrides, scrape runs and warnings, devices. |
| `/display` | Always-on mode: full screen, dark, the crown glowing, tonight plus the next 7. Rolls over at 04:00, re-fetches every 5 min, uses the Wake Lock API, hides the cursor, shifts slightly over time to avoid burn-in. Logging in is optional. |
| `/emulator` | The frame emulator (below). |
| `/docs` | Public API documentation. |

**For her:** mobile-first, large type, high contrast, one obvious main action, big tap targets, dark by default.

### Emulator (`/emulator`)
- **A virtual device.** It registers, gets claimed, and uses the same `/api/device/v1` endpoints as the hardware. It keeps its token in localStorage.
- **Screen:**
  - A frame graphic at **true size** (~128 ppi), in portrait.
  - Toggle between measured, ideal and raw-pixel views, with a zoom/pixel inspector.
  - An optional ~20 s redraw flash animation.
- **Controls:** buttons for KEY short, long and double press; a clock override (time travel); USB/battery switch with battery-level slider; offline toggle; "simulate a broken scrape."
- **Debug output:** a log of syncs and redraws, the queued events, and a running estimate of battery use per day.

## Firmware (last phase; ESP-IDF, `idf.py`)

**Stage A, prove the hardware:**
1. Flash `aitjcize/esp32-photoframe` unmodified and point it at the compatibility endpoint.
2. Calibrate the palette on the real panel and check purple, teal and maroon.
3. Hardware questions to answer:
   - Can KEY wake the board from deep sleep?
   - Is there an LED or sound available for instant button feedback?
   - Does the stand support portrait?
   - What's the sleep current, and how much does one sync cost?

**Stage B, fork it:**
- **Wake reasons:** the clock (noon redraw, rollover just after midnight, daily sync) or KEY.
- **Button gestures:** read after waking (hold time measured at boot, 400 ms window for a double press).
  - Short: show the next view from the saved bundle, with no Wi-Fi.
  - Long: log "seen," with immediate feedback, then sync and redraw.
  - Double: sync now.
- **Bundle storage:** saved to flash or the SD card. Downloads are verified by hash. Events are queued in NVS (on-chip settings storage).
- **Power-aware:** checks hourly on USB, once a day on battery. Redraws only when the image changed.
- **Setup and updates:**
  - Wi-Fi via captive portal or `wifi.txt` on the SD card (both already in aitjcize).
  - Then device register, then the claim QR.
  - Over-the-air updates with rollback.
- **Reference code:** Waveshare's repo for pins and charging, `tuanpmt/esp_epaper` for the panel, FlightPortrait's frame for the protocol and backoff patterns.
- `firmware/justfile`: `build`, `flash`, `monitor`, `menuconfig`, `erase` (wrapping `idf.py`).

## Ops

- **Dockerfile:** go-poker's three stages (node:alpine builds `web/out` → golang:alpine builds with `CGO_ENABLED=0` → alpine with `ca-certificates` and `tzdata`, running as a non-root user).
- **Compose on the VPS:** `evanofslack/prulights:latest`, a `prulights-data:/data` volume, env file, on the Caddy network.
  - Caddyfile: `prulights.<domain> { reverse_proxy prulights:8080 }`.
- **Environment variables:**
  - Paths and URLs: `DATABASE_PATH=/data/prulights.db`, `STATIC_DIR=/app/out`, `BASE_URL`
  - Passkeys: `RP_ID`, `RP_ORIGINS`
  - Scraper: `SCRAPE_ENABLED`, `SCRAPE_INTERVAL=6h`
  - Monitoring: `METRICS_ADDR=:9090`, `LOG_LEVEL`
  - The time zone is fixed to America/New_York in code.
- **Backups:** Litestream sidecar to S3 (you already use S3 for analogdb). Alternatively, a nightly `sqlite3 .backup` copied to S3.
- **Monitoring:** Prometheus `/metrics`: scrape age, entries parsed, warnings, requests, device check-ins, frame battery. `/healthz` and `/readyz`.
- **CI** (`ci.yml`, = `just check`): gofmt, vet, `go test -race`, types up to date, golden images, web type-check and build.
- **Publish** (`publish.yml`): go-poker's file unchanged (Docker Hub, `latest` + semver + branch tags). Add `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secrets.
- **Deploy:** `just deploy` runs `ssh $VPS 'cd ~/prulights && docker compose pull && docker compose up -d'`.

## Justfile (planned recipes)

```
default        list recipes
install        npm ci in web if needed
backend        go server :8080 with air (hot reload), dev webauthn origins
web            next dev :3000 → api :8080
start          backend + web together
serve          production-like: static export served by go
fmt / lint     gofmt, go vet, prettier
test           go test -race ./...
types          tygo generate
check          everything ci runs
golden         render golden images and compare; golden-update to accept
render         render one view to a PNG (view=, date=, palette=)
scrape         run the scraper once against live (--dry-run by default)
scrape-snapshot  refresh the saved html snapshots
backfill       import all known lighting pages
invite         print an invite link and QR (admin=true for an admin)
db             open sqlite3 on the dev database
docker-build   build the image locally
deploy         pull + up on the vps
logs           follow container logs on the vps
backup         one-off sqlite backup to s3
mod firmware   (later) build / flash / monitor / menuconfig via idf.py
```

## Phases

| # | Phase | Done when |
|---|---|---|
| 0 | Skeleton: repo, justfile, Dockerfile, CI, publish, compose, CLAUDE.md | An empty server serves the placeholder web app at `prulights.<domain>` |
| 1 | Scraper, store, public API, backfill, `.ics` | `curl /api/v1/tonight` in prod returns real data, the calendar subscription works, snapshot tests pass |
| 2 | Public web: tonight, night, calendar, `/display`, `/docs` | Your mom can bookmark it and it's correct every night |
| 3 | Auth, sightings, ratings, log/stats, admin (invites, overrides) | She's logged in with a passkey and rating nights |
| 4 | Renderer, views, golden tests, dev render loop | Every view looks right in the measured-color preview |
| 5 | Device API, emulator, settings, claim flow, compatibility endpoint | The emulator runs the full setup → claim → sync → button flow |
| 6 | Firmware A (stock firmware + calibration), then B (fork) | The frame on her desk, months on one charge |

Phases 4 and 5 can overlap. Phase 6 waits for the hardware. Order the board during phase 2 so
palette calibration isn't on the critical path.

## Open items

- Which domain to use for `prulights.<domain>` (has to be fixed before passkeys are created).
- Name for her on the frame and in the app ("Mom's view")? Mainly copy.
- Does the tower ever go **dark** instead of white (power outages, Earth Hour)? An admin override `dark` covers it.
- Lighting-on time outside December: sunset is an assumption. Have her watch a few nights to check it.
- Whether to publish an OpenAPI spec for the public API (hand-written YAML now, or generated later).
- Attribution text and a link to prudentialcenter.com on every page and in API `meta`. No Prudential logos or branding.
