#!/bin/sh
# Fails if the web's generated api types are out of date with the go code.
set -e
# nothing to generate until tygo.yaml lists a package
grep -q 'path:' tygo.yaml || exit 0
files=$(ls ../web/interfaces/*.ts 2>/dev/null || true)
before=$(cat $files /dev/null | shasum)
go tool tygo generate
files=$(ls ../web/interfaces/*.ts 2>/dev/null || true)
if [ "$(cat $files /dev/null | shasum)" != "$before" ]; then
    echo "web/interfaces were stale and have been regenerated; commit them (just types)"
    exit 1
fi
