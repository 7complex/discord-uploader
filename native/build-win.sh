#!/usr/bin/env bash
# Cross-compile do Discord Uploader nativo para Windows (amd64).
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Build do Discord Uploader nativo (Windows)"

# Verifica se o MinGW está instalado (obrigatório para Fyne/OpenGL com CGO).
command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1 || {
    echo "✘ Instale o MinGW: sudo apt install gcc-mingw-w64-x86-64"
    exit 1
}

command -v go >/dev/null 2>&1 || {
    echo "✘ Instale Go: sudo apt install golang"
    exit 1
}

echo "==> go mod tidy"
go mod tidy

# Cross-compile (CGO_ENABLED=1 é obrigatório para o Fyne/OpenGL).
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++
export CGO_ENABLED=1
export GOOS=windows
export GOARCH=amd64

echo "==> go build (windows/amd64, CGO=1)"
go build -ldflags="-s -w -H windowsgui" -o DiscordUploader.exe .

# -H windowsgui faz o executável não abrir o console preto do CMD ao clicar.

echo "✔ Executável Windows gerado: native/DiscordUploader.exe"
