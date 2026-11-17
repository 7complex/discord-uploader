#!/usr/bin/env bash
# Build do APK Android para o Discord Uploader (nativo Go + Fyne).
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Build do Discord Uploader (Android APK)"

export PATH="$HOME/go/bin:$HOME/Android/Sdk/platform-tools:$PATH"

# CLI do Fyne
if ! command -v fyne >/dev/null 2>&1; then
    echo "==> Instalando fyne CLI..."
    go install fyne.io/tools/cmd/fyne@latest
fi

# Variáveis obrigatórias do Android
if [[ -z "${ANDROID_HOME:-}" ]]; then
    echo "✘ Erro: ANDROID_HOME não configurado."
    echo "  Instale o Android Studio e exporte no seu .bashrc/.zshrc:"
    echo "  export ANDROID_HOME=\$HOME/Android/Sdk"
    exit 1
fi

if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
    echo "✘ Erro: ANDROID_NDK_HOME não configurado."
    echo "  Instale o NDK pelo Android Studio (SDK Manager > SDK Tools > NDK)"
    echo "  e exporte: export ANDROID_NDK_HOME=\$ANDROID_HOME/ndk/<versao>"
    exit 1
fi

echo "==> go mod tidy"
go mod tidy

echo "==> Compilando APK Android..."
fyne package -os android \
    -app-id com.github.sevencomplex.discorduploader \
    -icon icon.png

echo "✔ APK Android gerado: native/Discord Uploader.apk (ou similar na pasta native/)"
