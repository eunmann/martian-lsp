#!/usr/bin/env bash
# Build mrlsp and run the headless Neovim integration checks.
# Requires `nvim` (0.12+) and `go` on PATH.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"

if ! command -v nvim >/dev/null 2>&1; then
  echo "SKIP: nvim not found on PATH" >&2
  exit 0
fi

echo "==> building mrlsp"
go build -C "$repo" -o "$repo/mrlsp" ./cmd/mrlsp

export MRLSP="$repo/mrlsp"
export MRO_EXAMPLES="$repo/examples"

status=0
for script in features edit; do
  echo "==> nvim -l test/nvim/$script.lua"
  if ! nvim --clean -l "$here/$script.lua"; then
    status=1
  fi
  echo
done

exit "$status"
