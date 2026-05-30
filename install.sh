#!/usr/bin/env bash
# install.sh — build VRAM Guard from source and install to ~/Applications
set -euo pipefail

APP_NAME="VRAMGuard"
BUNDLE_ID="com.prestonw.vramguard"
VERSION="1.0.0"
INSTALL_DIR="$HOME/Applications"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "VRAM Guard $VERSION — installer"
echo "================================"

# Require macOS
if [[ "$(uname)" != "Darwin" ]]; then
    echo "Error: VRAM Guard is macOS only." >&2
    exit 1
fi

# Require Go
if ! command -v go &>/dev/null; then
    echo "Error: Go is not installed. Install from https://go.dev/dl/" >&2
    exit 1
fi

cd "$SCRIPT_DIR"

echo "Building..."
go build -ldflags="-s -w" -o "$APP_NAME" .

echo "Assembling .app bundle..."
rm -rf "$APP_NAME.app"
mkdir -p "$APP_NAME.app/Contents/MacOS"
mkdir -p "$APP_NAME.app/Contents/Resources"
cp "$APP_NAME" "$APP_NAME.app/Contents/MacOS/$APP_NAME"

cat > "$APP_NAME.app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>${APP_NAME}</string>
	<key>CFBundleIdentifier</key>
	<string>${BUNDLE_ID}</string>
	<key>CFBundleName</key>
	<string>${APP_NAME}</string>
	<key>CFBundleDisplayName</key>
	<string>VRAM Guard</string>
	<key>CFBundleShortVersionString</key>
	<string>${VERSION}</string>
	<key>CFBundleVersion</key>
	<string>${VERSION}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>LSUIElement</key>
	<true/>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>LSMinimumSystemVersion</key>
	<string>13.0</string>
</dict>
</plist>
PLIST

echo "Installing to $INSTALL_DIR/$APP_NAME.app..."
mkdir -p "$INSTALL_DIR"
rm -rf "$INSTALL_DIR/$APP_NAME.app"
cp -R "$APP_NAME.app" "$INSTALL_DIR/"

# Clean up local build artifacts
rm -f "$APP_NAME"
rm -rf "$APP_NAME.app"

echo ""
echo "Done. Launching VRAM Guard..."
open "$INSTALL_DIR/$APP_NAME.app"
echo ""
echo "Tip: click the menu bar icon and choose 'Launch at Login: Off' to enable auto-start."
