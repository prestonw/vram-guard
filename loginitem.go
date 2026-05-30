//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/systray"
)

// launchAgentPath returns ~/Library/LaunchAgents/<bundleID>.plist
func launchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", bundleID+".plist")
}

// appBundlePath resolves the .app bundle containing this binary by walking up
// from the executable path. Returns "" when running outside a bundle (e.g. dev).
func appBundlePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, _ = filepath.EvalSymlinks(exe)
	dir := exe
	for i := 0; i < 4; i++ {
		dir = filepath.Dir(dir)
		if strings.HasSuffix(dir, ".app") {
			return dir
		}
	}
	return ""
}

// isLaunchAtLoginEnabled reports whether the LaunchAgent plist is installed.
func isLaunchAtLoginEnabled() bool {
	_, err := os.Stat(launchAgentPath())
	return err == nil
}

// launchAtLoginLabel returns the menu item title reflecting current state.
func launchAtLoginLabel() string {
	if isLaunchAtLoginEnabled() {
		return "Launch at Login: On  ✓"
	}
	return "Launch at Login: Off"
}

// enableLaunchAtLogin writes the LaunchAgent plist and loads it with launchctl.
func enableLaunchAtLogin() error {
	appPath := appBundlePath()
	if appPath == "" {
		return fmt.Errorf("not running from a .app bundle — install via 'make install' first")
	}
	binaryPath := filepath.Join(appPath, "Contents", "MacOS", "VRAMGuard")

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>
`, bundleID, binaryPath)

	dir := filepath.Dir(launchAgentPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(launchAgentPath(), []byte(plist), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	exec.Command("launchctl", "load", launchAgentPath()).Run() //nolint:errcheck
	dbgf("enableLaunchAtLogin: plist written to %s", launchAgentPath())
	return nil
}

// disableLaunchAtLogin unloads the LaunchAgent and removes the plist.
func disableLaunchAtLogin() {
	plistPath := launchAgentPath()
	exec.Command("launchctl", "unload", plistPath).Run() //nolint:errcheck
	os.Remove(plistPath)                                 //nolint:errcheck
	dbgf("disableLaunchAtLogin: plist removed")
}

// showAbout displays an osascript dialog with version info and a GitHub link.
func showAbout() {
	msg := fmt.Sprintf(
		"%s v%s\\n\\n"+
			"GPU memory monitor for OpenCore Legacy Patcher systems.\\n"+
			"Built for 2011 iMac · AMD Radeon HD 6750M\\/6970M · 512 MB VRAM.\\n\\n"+
			"MIT License\\n"+
			"%s",
		appName, appVersion, githubURL,
	)
	script := fmt.Sprintf(
		`display dialog "%s" with title "About %s" buttons {"View on GitHub", "OK"} default button "OK"`,
		msg, appName,
	)
	out, _ := exec.Command("osascript", "-e", script).CombinedOutput()
	if strings.Contains(string(out), "View on GitHub") {
		exec.Command("open", githubURL).Run() //nolint:errcheck
	}
}

// uninstall confirms with the user, removes the LaunchAgent, moves the .app to
// Trash, and quits. The running binary stays in memory until systray.Quit() exits.
func uninstall() {
	confirmScript := `display dialog "This will remove VRAM Guard from your Mac.\n\n` +
		`• Login item (LaunchAgent) will be removed\n` +
		`• App will be moved to Trash\n` +
		`• Logs remain in ~/Library/Logs/VRAMGuard/\n\n` +
		`Continue?" with title "Uninstall VRAM Guard" ` +
		`buttons {"Cancel", "Uninstall"} default button "Cancel"`

	out, _ := exec.Command("osascript", "-e", confirmScript).CombinedOutput()
	if !strings.Contains(string(out), "button returned:Uninstall") {
		return
	}

	dbgSection("ACTION: uninstall")

	// Remove LaunchAgent
	disableLaunchAtLogin()

	// Move .app bundle to Trash via Finder
	appPath := appBundlePath()
	if appPath != "" {
		trashScript := fmt.Sprintf(
			`tell application "Finder" to move POSIX file "%s" to trash`,
			appPath,
		)
		exec.Command("osascript", "-e", trashScript).Run() //nolint:errcheck
		dbgf("uninstall: moved %s to trash", appPath)
	}

	systray.Quit()
}
