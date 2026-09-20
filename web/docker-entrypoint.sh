#!/bin/sh
set -eu

marker="node_modules/.clip-share-package-lock.sha256"
expected="$(sha256sum package-lock.json | cut -d ' ' -f 1)"
installed=""

if [ -f "$marker" ]; then
  installed="$(tr -d '\r\n' < "$marker")"
fi

if [ "$installed" != "$expected" ]; then
  echo "package-lock.json changed; synchronizing web dependencies"
  npm ci
  printf '%s\n' "$expected" > "$marker"
fi

exec npm run dev
