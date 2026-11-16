#!/usr/bin/env bash
# Build do Discord Uploader nativo para Linux.
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Build do Discord Uploader nativo (Linux)"

command -v go >/dev/null 2>&1 || {
    echo "✘ Instale Go: sudo apt install golang"
    exit 1
}

echo "==> go mod tidy"
go mod tidy

echo "==> go build"
go build -o DiscordUploader .

echo "✔ Binário Linux gerado: native/DiscordUploader"
