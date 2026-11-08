# prulights

What color is the Pru tonight? A Go server (`backend/`) that scrapes the Prudential Tower lighting schedule
and serves a public API, plus a Next.js static web app (`web/`) and server-drawn e-ink frame images. Design
is in `plans/prulights.md`, build plan in `plans/build.md`.

## Commands
- Dev: `just start` (go on :8080 with hot reload, next on :3000 passing `/api` through to go).
  `just serve` runs the production-like build on :8080. `just` lists all recipes.
- `just check` runs exactly what CI runs:
  - Backend (from `backend/`): `gofmt -l .` (must be empty), `go vet ./...`, `./types-current.sh`,
    `go test -race ./...`
  - Web (from `web/`): `npm run type-check`, `npm run build`. No unit tests.
- Run the checks for whichever side you touched before calling it done.

## Architecture
- `backend/config`: everything read from the environment, in one `Config`.
- `backend/store`: one sqlite file (modernc, no cgo). Schema changes are new numbered files in
  `store/migrations/`, never edits to old ones. Tests use `store.Open(ctx, ":memory:")`.
- `backend/server`: chi. `/api/...` is JSON; anything else is the static export from `STATIC_DIR`
  (`/calendar` is served from `calendar.html`, unknown paths fall back to `index.html`).
  `/healthz`, `/readyz`; Prometheus on `METRICS_ADDR`, never the public port.
- `web/` is a static export (`output: 'export'`, App Router). No API routes, SSR or middleware.
  Pages that need a parameter take it in the query string (`/night?d=2026-12-12`), not the path.
- API types: Go structs → `just types` (tygo) → `web/interfaces/*.ts`. CI fails if stale.
- Times: the tower is in Boston. All "tonight" logic uses America/New_York and rolls over at 04:00.

## UI
- Mantine (controls) + Tailwind 4 (layout, config in CSS). Utilities beat Mantine's styles (CSS layers).
- Tokens in `web/styles/index.css` (`surface`, `panel`, `raised`, `fg`, `muted`, `subtle`, `line`,
  `accent`, …) point at the Mantine theme in `web/lib/theme.ts`. Use tokens, not raw Tailwind colors.
- Dark (night sky) by default. The main user is an older reader on a phone: large type, high contrast,
  one obvious main action per screen, big tap targets.

## Conventions
- Commits: short, lowercase, imperative, 3–5 words ("add scraper parsers").
- PRs: 4–6 word titles, 1–2 sentence descriptions. Work goes up as a `gh stack`.
- Credit prudentialcenter.com as the source; no Prudential logos or branding.
