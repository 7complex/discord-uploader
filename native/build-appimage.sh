#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Build do Discord Uploader Nativo (AppImage)"
export PATH="$HOME/go/bin:$HOME/.local/bin:$PATH"

# 1. Compila o binário nativo do Linux
go build -ldflags="-s -w" -o DiscordUploader .

# 2. Prepara a estrutura do AppDir
rm -rf /tmp/du-appdir
mkdir -p /tmp/du-appdir/usr/bin
mkdir -p /tmp/du-appdir/usr/share/applications
mkdir -p /tmp/du-appdir/usr/share/icons/hicolor/256x256/apps

cp DiscordUploader /tmp/du-appdir/usr/bin/discord-uploader
chmod +x /tmp/du-appdir/usr/bin/discord-uploader

cp icon.png /tmp/du-appdir/com.github.sevencomplex.discorduploader.png
cp icon.png /tmp/du-appdir/usr/share/icons/hicolor/256x256/apps/com.github.sevencomplex.discorduploader.png

cat > /tmp/du-appdir/com.github.sevencomplex.discorduploader.desktop <<'EOF'
[Desktop Entry]
Type=Application
Name=Discord Uploader
Comment=Envia arquivos para o Discord via webhook
Exec=discord-uploader
Icon=com.github.sevencomplex.discorduploader
Categories=Utility;Network;
Terminal=false
EOF

cat > /tmp/du-appdir/AppRun <<'EOF'
#!/bin/bash
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/discord-uploader" "$@"
EOF
chmod +x /tmp/du-appdir/AppRun

# 3. Gera o AppImage
mkdir -p dist
ARCH=x86_64 appimagetool /tmp/du-appdir dist/DiscordUploader-native-x86_64.AppImage
chmod +x dist/DiscordUploader-native-x86_64.AppImage

echo "✔ AppImage gerado: native/dist/DiscordUploader-native-x86_64.AppImage"
