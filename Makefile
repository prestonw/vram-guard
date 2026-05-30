VERSION     := 1.0.0
APP_NAME    := VRAMGuard
BUNDLE_ID   := com.prestonw.vramguard
BINARY      := vram-guard
APP_DIR     := $(APP_NAME).app
INSTALL_DIR := $(HOME)/Applications
DIST_DIR    := dist
ICNS        := pkg/VRAMGuard.icns
COPYRIGHT   := © 2026 Will Preston. MIT License.

.PHONY: build icon sign bundle install run pkg uninstall clean

## build: compile the Go binary (stripped)
build:
	go build -ldflags="-s -w" -o $(BINARY) .

## icon: render 🎮 emoji into pkg/VRAMGuard.icns  (runs once; reruns if script changes)
$(ICNS): scripts/make_icon.swift
	@echo "Generating icon..."
	swift scripts/make_icon.swift
	iconutil -c icns VRAMGuard.iconset -o $(ICNS)
	rm -rf VRAMGuard.iconset
	@echo "Icon: $(ICNS)"

icon: $(ICNS)

## sign: ad-hoc codesign the .app bundle
sign: bundle
	codesign --deep --force --sign - $(APP_DIR)
	@echo "Signed: $(APP_DIR)"

## bundle: build + icon + assemble .app with full metadata
bundle: build $(ICNS)
	rm -rf $(APP_DIR)
	mkdir -p $(APP_DIR)/Contents/MacOS
	mkdir -p $(APP_DIR)/Contents/Resources
	cp $(BINARY) $(APP_DIR)/Contents/MacOS/$(APP_NAME)
	cp $(ICNS)   $(APP_DIR)/Contents/Resources/$(APP_NAME).icns
	@printf '<?xml version="1.0" encoding="UTF-8"?>\n\
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n\
<plist version="1.0">\n\
<dict>\n\
\t<key>CFBundleExecutable</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleIdentifier</key>\n\
\t<string>$(BUNDLE_ID)</string>\n\
\t<key>CFBundleName</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleDisplayName</key>\n\
\t<string>VRAM Guard</string>\n\
\t<key>CFBundleIconFile</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleShortVersionString</key>\n\
\t<string>$(VERSION)</string>\n\
\t<key>CFBundleVersion</key>\n\
\t<string>$(VERSION)</string>\n\
\t<key>CFBundleGetInfoString</key>\n\
\t<string>VRAM Guard $(VERSION), $(COPYRIGHT)</string>\n\
\t<key>NSHumanReadableCopyright</key>\n\
\t<string>$(COPYRIGHT)</string>\n\
\t<key>CFBundlePackageType</key>\n\
\t<string>APPL</string>\n\
\t<key>LSUIElement</key>\n\
\t<true/>\n\
\t<key>NSHighResolutionCapable</key>\n\
\t<true/>\n\
\t<key>NSRequiresAquaSystemAppearance</key>\n\
\t<false/>\n\
\t<key>LSApplicationCategoryType</key>\n\
\t<string>public.app-category.utilities</string>\n\
\t<key>LSMinimumSystemVersion</key>\n\
\t<string>13.0</string>\n\
</dict>\n\
</plist>\n' > $(APP_DIR)/Contents/Info.plist
	codesign --deep --force --sign - $(APP_DIR)
	@echo "Bundle: $(APP_DIR)"

## install: bundle + copy to ~/Applications and launch
install: bundle
	mkdir -p $(INSTALL_DIR)
	rm -rf $(INSTALL_DIR)/$(APP_DIR)
	cp -R $(APP_DIR) $(INSTALL_DIR)/
	@echo "Installed: $(INSTALL_DIR)/$(APP_DIR)"
	@echo "Use the menu bar icon to enable Launch at Login."
	open $(INSTALL_DIR)/$(APP_DIR)

## run: bundle and open immediately (dev shortcut)
run: bundle
	open $(APP_DIR)

## zip: create a distributable .zip of the .app (no pkg/signing needed)
zip: bundle
	mkdir -p $(DIST_DIR)
	ditto -c -k --sequesterRsrc --keepParent $(APP_DIR) $(DIST_DIR)/$(APP_NAME)-$(VERSION).zip
	@echo "Archive: $(DIST_DIR)/$(APP_NAME)-$(VERSION).zip"

## pkg: create a distributable .pkg installer (installs to /Applications)
## Requires macOS with Xcode Command Line Tools.
pkg: bundle
	mkdir -p $(DIST_DIR)/root/Applications
	cp -R $(APP_DIR) $(DIST_DIR)/root/Applications/
	pkgbuild \
		--root $(DIST_DIR)/root \
		--scripts pkg/scripts \
		--identifier $(BUNDLE_ID) \
		--version $(VERSION) \
		--ownership recommended \
		$(DIST_DIR)/$(APP_NAME)-$(VERSION).pkg
	rm -rf $(DIST_DIR)/root
	@echo "Package: $(DIST_DIR)/$(APP_NAME)-$(VERSION).pkg"

## uninstall: remove from ~/Applications and clean up login item
uninstall:
	launchctl unload ~/Library/LaunchAgents/$(BUNDLE_ID).plist 2>/dev/null || true
	rm -f ~/Library/LaunchAgents/$(BUNDLE_ID).plist
	rm -rf $(INSTALL_DIR)/$(APP_DIR)
	@echo "VRAM Guard uninstalled."

## clean: remove local build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(APP_DIR)
	rm -rf $(DIST_DIR)
	rm -rf VRAMGuard.iconset
