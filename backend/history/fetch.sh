#!/usr/bin/env bash
# Download the Wayback Machine snapshots manifest.json lists into pages/, as
# the site served them (gzipped). Run by hand when the manifest changes, then
# review: go test ./history ./scrape
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p pages
ua="prulights-history/1.0"
jq -r '.[] | "\(.snapshot) \(.url)"' manifest.json | while read -r ts url; do
    page=$(basename "$url")
    out="pages/$page-$ts.html.gz"
    [ -s "$out" ] && continue
    echo "$page $ts"
    tmp=$(mktemp)
    curl -fsSL -A "$ua" -o "$tmp" "https://web.archive.org/web/${ts}id_/$url"
    if gzip -t "$tmp" 2>/dev/null; then gunzip -c "$tmp"; else cat "$tmp"; fi | gzip -9n > "$out"
    rm -f "$tmp"
    sleep 5
done
