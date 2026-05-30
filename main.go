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
	initDebugLog()
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

	// Auto-relief — restore persisted mode, click to cycle through modes
	restoredMode := loadAutoReliefMode()
	autoReliefModeVal.Store(int32(restoredMode))
	mAutoRelief := systray.AddMenuItem(autoReliefLabel(restoredMode), "Click to cycle: Off → Gentle → Moderate → Aggressive")
	systray.AddSeparator()

	mInfo := systray.AddMenuItem("GPU Details...", "Show VRAM breakdown in a dialog")
	systray.AddSeparator()
	mDebugOn := systray.AddMenuItem("Turn on Debugging", "Enable verbose logging to ~/Library/Logs/VRAMGuard/")
	mDebugLog := systray.AddMenuItem("Show Debug Log", "Open ~/Library/Logs/VRAMGuard/vramguard.log in Console.app")
	mDebugOff := systray.AddMenuItem("Turn off Debugging", "Disable verbose logging")
	mDebugLog.Hide()
	mDebugOff.Hide()
	systray.AddSeparator()

	mAbout := systray.AddMenuItem("About VRAM Guard…", "Version "+appVersion+" · "+githubURL)
	systray.AddSeparator()

	mLoginItem := systray.AddMenuItem(launchAtLoginLabel(), "Toggle launch at login via LaunchAgent")
	systray.AddSeparator()

	mUninstall := systray.AddMenuItem("Uninstall VRAM Guard…", "Remove app and login item")
	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit VRAM Guard", "")

	go pollLoop(mStats, procItems)
	go handleClicks(procItems, mSleep, mReload, mRestart, mAutoRelief, mInfo, mDebugOn, mDebugLog, mDebugOff, mAbout, mLoginItem, mUninstall, mQuit)
}

func pollLoop(mStats *systray.MenuItem, procItems []*systray.MenuItem) {
	update := func() {
		stats, err := getVRAMStats()
		if err != nil {
			systray.SetTitle("VRAM?")
			mStats.SetTitle("VRAM unavailable: " + err.Error())
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

			// Auto-relief: runs purge and/or display sleep based on configured mode
			checkAutoRelief(stats.Percent)
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

func handleClicks(procItems []*systray.MenuItem, mSleep, mReload, mRestart, mAutoRelief, mInfo, mDebugOn, mDebugLog, mDebugOff, mAbout, mLoginItem, mUninstall, mQuit *systray.MenuItem) {
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
		case <-mAutoRelief.ClickedCh:
			mAutoRelief.SetTitle(cycleAutoReliefMode())
		case <-mInfo.ClickedCh:
			showGPUInfo()
		case <-mDebugOn.ClickedCh:
			enableDebug()
			mDebugOn.Hide()
			mDebugLog.Show()
			mDebugOff.Show()
		case <-mDebugLog.ClickedCh:
			openDebugLog()
		case <-mDebugOff.ClickedCh:
			disableDebug()
			mDebugLog.Hide()
			mDebugOff.Hide()
			mDebugOn.Show()
		case <-mAbout.ClickedCh:
			go showAbout()
		case <-mLoginItem.ClickedCh:
			go func() {
				if isLaunchAtLoginEnabled() {
					disableLaunchAtLogin()
				} else {
					if err := enableLaunchAtLogin(); err != nil {
						osNotify(appName, "Login item: "+err.Error())
					}
				}
				mLoginItem.SetTitle(launchAtLoginLabel())
			}()
		case <-mUninstall.ClickedCh:
			go uninstall()
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func fmtMB(b int64) string {
	return fmt.Sprintf("%d MB", b/(1024*1024))
}
