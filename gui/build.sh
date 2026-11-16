#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Build do Discord Uploader (GUI)"

command -v go >/dev/null || { echo "✘ Instale Go: sudo apt install golang"; exit 1; }
export PATH="$HOME/go/bin:$HOME/.local/bin:$PATH"

# 1. Copia CLI para o bundle
cp -f ../discord-uploader.sh ./discord-uploader.sh
chmod 755 ./discord-uploader.sh

# 2. Fyne CLI
if ! command -v fyne >/dev/null; then
    echo "==> Instalando fyne CLI..."
    go install fyne.io/tools/cmd/fyne@latest
fi

# 3. appimagetool
if ! command -v appimagetool >/dev/null; then
    echo "✘ Instale appimagetool:"
    echo "  wget -O ~/.local/bin/appimagetool https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage"
    echo "  chmod +x ~/.local/bin/appimagetool"
    exit 1
fi

# 4. Ícone (gera placeholder se não existir)
if [[ ! -f icon.png ]] && command -v convert >/dev/null; then
    echo "! Gerando ícone placeholder (substitua por um icon.png real depois)"
    convert -size 256x256 xc:'#5865F2' -gravity center -pointsize 96 -fill white -annotate 0 'DU' icon.png
fi

# 5. go mod tidy
echo "==> go mod tidy"
go mod tidy

# 6. Fyne package
echo "==> fyne package -os linux"
rm -f ./*.tar.xz ./*.AppImage
fyne package -os linux -icon icon.png -name "Discord Uploader" --release

# 7. Extrair binário do tar.xz
BIN=""
if [[ -f ./gui ]]; then
    BIN=./gui
else
    TARXZ=$(ls -1 ./*.tar.xz 2>/dev/null | head -1)
    if [[ -z "$TARXZ" ]]; then
        echo "✘ Nem binário nem .tar.xz gerados"
        exit 1
    fi
    rm -rf /tmp/du-full && mkdir -p /tmp/du-full
    tar -xJf "$TARXZ" -C /tmp/du-full
    BIN=$(find /tmp/du-full -type f -name gui | head -1)
    if [[ -z "$BIN" ]]; then
        echo "✘ Binário não encontrado no tar.xz"
        exit 1
    fi
fi

# 8. Montar AppDir
echo "==> Montando AppDir"
rm -rf /tmp/AppDir
mkdir -p /tmp/AppDir/usr/bin /tmp/AppDir/usr/share/applications /tmp/AppDir/usr/share/icons/hicolor/256x256/apps
cp "$BIN" /tmp/AppDir/usr/bin/discord-uploader-gui
chmod +x /tmp/AppDir/usr/bin/discord-uploader-gui
cp ./discord-uploader.sh /tmp/AppDir/usr/bin/discord-uploader.sh
chmod +x /tmp/AppDir/usr/bin/discord-uploader.sh
cp icon.png /tmp/AppDir/com.github.sevencomplex.discord-uploader.png
cp icon.png /tmp/AppDir/usr/share/icons/hicolor/256x256/apps/com.github.sevencomplex.discord-uploader.png

cat > /tmp/AppDir/com.github.sevencomplex.discord-uploader.desktop <<'DEOF'
[Desktop Entry]
Type=Application
Name=Discord Uploader
Comment=Envia arquivos para o Discord via webhook
Exec=discord-uploader-gui
Icon=com.github.sevencomplex.discord-uploader
Categories=Utility;Network;
Terminal=false
DEOF

cat > /tmp/AppDir/AppRun << 'AEOF'
#!/bin/bash
HERE="$(dirname "$(readlink -f "$0")")"
export PATH="$HERE/usr/bin:$PATH"
exec "$HERE/usr/bin/discord-uploader-gui" "$@"
AEOF
chmod +x /tmp/AppDir/AppRun

# 9. Gerar AppImage
echo "==> Gerando AppImage"
mkdir -p dist
ARCH=x86_64 appimagetool /tmp/AppDir dist/DiscordUploader-x86_64.AppImage 2>&1 | tail -5 || \
    ARCH=x86_64 appimagetool --no-appstream /tmp/AppDir dist/DiscordUploader-x86_64.AppImage
chmod 755 dist/DiscordUploader-x86_64.AppImage
rm -f ./*.tar.xz

echo "✔ AppImage: gui/dist/DiscordUploader-x86_64.AppImage"
