//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// restartWindowServer kills WindowServer (SIGKILL).
// This logs the user out and reinitialises the GPU driver — the closest thing to a
// soft GPU reset on macOS. The app itself will be killed as part of this process.
func restartWindowServer() {
	osNotify("VRAM Guard", "Restarting WindowServer — you will be logged out in 2 seconds")
	time.Sleep(2 * time.Second)
	// WindowServer runs as the current user; no elevation needed.
	exec.Command("killall", "-9", "WindowServer").Run() //nolint:errcheck
}

// reloadWindowServer sends SIGHUP to WindowServer.
// Less destructive than a full kill — may recover a stuck compositor without logging out.
func reloadWindowServer() {
	out, err := exec.Command("pkill", "-HUP", "WindowServer").CombinedOutput()
	if err != nil {
		osNotify("VRAM Guard", "Reload failed: "+strings.TrimSpace(string(out)))
	}
}

// displaySleepKick puts the display to sleep immediately.
// Sometimes wakes a stuck Metal compositor when the display comes back on.
func displaySleepKick() {
	// pmset displaysleepnow works without sudo on macOS for the current user's session.
	out, err := exec.Command("pmset", "displaysleepnow").CombinedOutput()
	if err != nil {
		// Fall back to elevated run if it failed without root
		if elevErr := runElevated("pmset displaysleepnow"); elevErr != nil {
			osNotify("VRAM Guard", "Sleep kick failed: "+strings.TrimSpace(string(out)))
		}
	}
}

// showGPUInfo displays a macOS dialog with current VRAM breakdown.
func showGPUInfo() {
	stats, err := getVRAMStats()
	var body string
	if err != nil {
		body = "Could not read VRAM data:\\n\\n" + err.Error()
	} else {
		body = fmt.Sprintf(
			"Used:  %s\\nFree:  %s\\nTotal: %s\\n\\nUsage: %d%%",
			fmtMB(stats.UsedBytes),
			fmtMB(stats.TotalBytes-stats.UsedBytes),
			fmtMB(stats.TotalBytes),
			stats.Percent,
		)
	}
	script := fmt.Sprintf(
		`display dialog "%s" with title "VRAM Guard — GPU Info" buttons {"OK"} default button "OK"`,
		body,
	)
	exec.Command("osascript", "-e", script).Run() //nolint:errcheck
}

// osNotify sends a macOS notification banner.
func osNotify(title, message string) {
	script := fmt.Sprintf(
		`display notification "%s" with title "%s"`,
		strings.ReplaceAll(message, `"`, `\"`),
		strings.ReplaceAll(title, `"`, `\"`),
	)
	exec.Command("osascript", "-e", script).Run() //nolint:errcheck
}

// confirmAndKill shows a native macOS confirmation dialog then kills the process if confirmed.
func confirmAndKill(pid int, name string) {
	script := fmt.Sprintf(
		`display dialog "Kill \"%s\" (PID %d)?\n\nThis may free GPU memory but will force-quit the app." `+
			`with title "VRAM Guard" buttons {"Cancel", "Kill"} default button "Cancel"`,
		strings.ReplaceAll(name, `"`, `\"`), pid,
	)
	out, _ := exec.Command("osascript", "-e", script).CombinedOutput()
	if strings.Contains(string(out), "button returned:Kill") {
		exec.Command("kill", strconv.Itoa(pid)).Run() //nolint:errcheck
	}
}

// runElevated runs a shell command via osascript with administrator privileges,
// triggering macOS's native password dialog.
func runElevated(command string) error {
	script := fmt.Sprintf(
		`do shell script "%s" with administrator privileges`,
		strings.ReplaceAll(command, `"`, `\"`),
	)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
