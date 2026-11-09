# e2e

Browser checks against a real server and a static build: `just e2e` builds both, loads the
scraper fixtures (2026 and 31 Nights) into a fresh database, serves on :8099 and runs every
script, or the named ones (`just e2e public`).

Uses `playwright-core` with its own chromium (`npx playwright-core install chromium` once) or
`CHROMIUM_PATH`. `SHOTS=/some/dir` saves screenshots.
