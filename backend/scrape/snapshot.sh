#!/usr/bin/env bash
# Re-download the saved source pages the scraper tests use, 10s apart as
# robots.txt asks, then check them against the golden json.
set -euo pipefail
cd "$(dirname "$0")/testdata"
ua="prulights/1.0 (+https://prulights.evanslack.dev)"
site=https://www.prudentialcenter.com
curl -fsSL -A "$ua" -o event-sitemap.xml "$site/event-sitemap.xml"
for page in 2024-tower-lighting-schedule 2026-tower-lightings 31-nights-of-light; do
    sleep 10
    echo "$page"
    curl -fsSL -A "$ua" -o "$page.html" "$site/experiences/events/$page/"
done
cd .. && go test . || echo "the pages changed: review, then go test ./scrape -update"
