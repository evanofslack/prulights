# Deploy runbook

prulights runs as one container (`evanofslack/prulights`) on the amd64 VPS. Caddy sits in front
and handles HTTPS, and Cloudflare handles DNS. Data is a single SQLite file in the `prulights-data` volume.

## One-time setup

1. **Docker Hub secrets.** In GitHub, go to repo → Settings → Secrets and variables → Actions, and add
   `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`. `publish.yml` pushes `latest` on every merge to `main`, and
   semver tags on `v*` tags.
2. **Cloudflare DNS.** In `evanslack.dev`, add an `A` record `prulights` pointing to the VPS IP.
   - **Proxied** (orange cloud) is fine. Set SSL/TLS mode to **Full (strict)**, so Cloudflare
     talks HTTPS to Caddy and Caddy's certificate is checked.
   - If Caddy fails to get a certificate through the proxy, switch the record to DNS-only (grey cloud) once until the
     certificate is issued, then turn the proxy back on.
3. **On the VPS:**
   ```sh
   mkdir -p ~/prulights && cd ~/prulights
   # copy docker-compose.yaml from the repo here
   # if caddy's docker network isn't called "caddy":
   echo "CADDY_NETWORK=<name>" > .env
   docker compose pull && docker compose up -d
   ```
4. **Caddy.** Add `deploy/Caddyfile.snippet` to the Caddyfile and reload Caddy.
5. **Check:** `curl https://prulights.evanslack.dev/readyz` should return `{"status":"ok"}`.
6. **First admin.** Passkeys are tied to `prulights.evanslack.dev`, so do this on the real domain:
   ```sh
   docker compose exec prulights ./prulights invite --admin
   ```
   Open the printed link (or scan the QR code) on your phone and register a passkey.

## Every deploy

```sh
VPS=user@host just deploy   # docker compose pull && up -d
VPS=user@host just logs
```

## Backups

The database is `/data/prulights.db` inside the volume. For a one-off copy:
```sh
docker compose exec prulights sh -c 'sqlite3 /data/prulights.db ".backup /data/backup.db"' # if sqlite3 is installed
docker cp prulights:/data/prulights.db ./prulights-$(date +%F).db                          # or copy the file
```
The scraped schedule can always be rebuilt with `prulights backfill`. Sightings, users and devices can't, so back them up.

## Settings (environment variables)

| Variable | Default | Notes |
|---|---|---|
| `BASE_URL` | `http://localhost:8080` | Public origin. Also sets the passkey RP ID and origin. |
| `TRUST_PROXY` | `false` | `true` behind Caddy/Cloudflare, so client IPs are read from the proxy headers |
| `DATABASE_PATH` | `/data/prulights.db` (image) | |
| `SCRAPE_ENABLED` / `SCRAPE_INTERVAL` | `true` / `6h` | |
| `METRICS_ADDR` | `:2112` | Prometheus; not published by compose |
| `LOG_LEVEL` | `info` | `debug`, `warn`, `error` |
| `RP_ID` / `RP_ORIGINS` | from `BASE_URL` | Only for unusual setups |
