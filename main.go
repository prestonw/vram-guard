//go:build darwin

// VRAM Guard — macOS menu bar GPU memory monitor for OpenCore Legacy Patcher systems.
//
// Build:
//   go mod tidy
//   go build -o vram-guard .
//   ./vram-guard
//
// To run without a Dock icon (proper menu-bar-only app), wrap in an .app bundle:
//   mkdir -p VRAMGuard.app/Contents/MacOS
//   cp vram-guard VRAMGuard.app/Contents/MacOS/VRAMGuard
//   /usr/libexec/PlistBuddy -c "Add :LSUIElement bool true" VRAMGuard.app/Contents/Info.plist
//   open VRAMGuard.app
//
// Add to Login Items: System Settings → General → Login Items → add VRAMGuard.app

package main

import (
	"fmt"
	"time"

	"fyne.io/systray"
)

const maxProcSlots = 5

func main() {
	systray.Run(onReady, nil)
}

func onReady() {
	systray.SetIcon(makeIcon())
	systray.SetTitle("VRAM")
	systray.SetTooltip("VRAM Guard — GPU memory monitor")

	// Status line (disabled — display only)
	mStats := systray.AddMenuItem("Querying VRAM...", "Live GPU memory usage")
	mStats.Disable()
	systray.AddSeparator()

	// Top consumers section
	mProcHeader := systray.AddMenuItem("Top GPU Consumers", "Click a process to kill it and free GPU memory")
	mProcHeader.Disable()
	procItems := make([]*systray.MenuItem, maxProcSlots)
	for i := range procItems {
		procItems[i] = systray.AddMenuItem("", "")
		procItems[i].Hide()
	}
	systray.AddSeparator()

	// Recovery actions — ordered least to most disruptive
	mSleep := systray.AddMenuItem("Display Sleep/Wake Kick", "pmset displaysleepnow — may unstick a hung Metal compositor")
	mReload := systray.AddMenuItem("Gentle WindowServer Reload", "SIGHUP — compositor restarts without logging you out")
	mRestart := systray.AddMenuItem("Restart WindowServer  ⚠", "SIGKILL — frees all VRAM; you will be logged out")
	systray.AddSeparator()

	mInfo := systray.AddMenuItem("GPU Details...", "Show VRAM breakdown in a dialog")
	systray.AddSeparator()
	mExpand := systray.AddMenuItem("Expand VRAM...", "Patch OpenCore to allocate system RAM as extra VRAM (reboot required)")
	mRevert := systray.AddMenuItem("Revert VRAM Expansion", "Remove the VRAM expansion from OpenCore config (reboot required)")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit VRAM Guard", "")

	go pollLoop(mStats, procItems)
	go handleClicks(procItems, mSleep, mReload, mRestart, mInfo, mExpand, mRevert, mQuit)
}

// prevPercent tracks the last reading to fire threshold alerts only on crossing.
var prevPercent int

func pollLoop(mStats *systray.MenuItem, procItems []*systray.MenuItem) {
	update := func() {
		stats, err := getVRAMStats()
		if err != nil {
			systray.SetTitle("VRAM?")
			mStats.SetTitle("VRAM unavailable: " + err.Error())
			prevPercent = 0
		} else {
			label := fmt.Sprintf("%d%%", stats.Percent)
			switch {
			case stats.Percent >= 90:
				label = "!!" + label
			case stats.Percent >= 75:
				label = "!" + label
			}
			systray.SetTitle(label)

			mStats.SetTitle(fmt.Sprintf(
				"VRAM: %s / %s  (%d%%)",
				fmtMB(stats.UsedBytes),
				fmtMB(stats.TotalBytes),
				stats.Percent,
			))

			// Notify once when crossing 85% from below
			if stats.Percent >= 85 && prevPercent < 85 {
				osNotify("VRAM Guard", fmt.Sprintf(
					"GPU memory at %d%% — close GPU-heavy apps or use Recovery Actions",
					stats.Percent,
				))
			}
			prevPercent = stats.Percent
		}

		// Update top process slots
		procs := getTopProcesses(maxProcSlots)
		currentProcsMu.Lock()
		currentProcs = procs
		currentProcsMu.Unlock()
		for i, item := range procItems {
			if i < len(procs) {
				p := procs[i]
				item.SetTitle(fmt.Sprintf("%s  —  %s", truncateName(p.Name, 22), fmtMB(p.RSS)))
				item.SetTooltip(fmt.Sprintf("PID %d — click to force-quit", p.PID))
				item.Show()
			} else {
				item.Hide()
			}
		}
	}

	update() // immediate first read
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		update()
	}
}

func handleClicks(procItems []*systray.MenuItem, mSleep, mReload, mRestart, mInfo, mExpand, mRevert, mQuit *systray.MenuItem) {
	for {
		select {
		case <-procItems[0].ClickedCh:
			go killProcAtIndex(0)
		case <-procItems[1].ClickedCh:
			go killProcAtIndex(1)
		case <-procItems[2].ClickedCh:
			go killProcAtIndex(2)
		case <-procItems[3].ClickedCh:
			go killProcAtIndex(3)
		case <-procItems[4].ClickedCh:
			go killProcAtIndex(4)
		case <-mSleep.ClickedCh:
			displaySleepKick()
		case <-mReload.ClickedCh:
			reloadWindowServer()
		case <-mRestart.ClickedCh:
			restartWindowServer()
		case <-mInfo.ClickedCh:
			showGPUInfo()
		case <-mExpand.ClickedCh:
			go showExpandVRAMDialog()
		case <-mRevert.ClickedCh:
			go revertVRAMExpansion()
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func fmtMB(b int64) string {
	return fmt.Sprintf("%d MB", b/(1024*1024))
}
