//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// AutoReliefMode controls how aggressively VRAM pressure is automatically relieved.
//
// The mechanism: running /usr/bin/purge flushes the inactive file-system page cache,
// which tightens system memory pressure. The AMD GPU kext listens for VM pressure
// notifications and responds by evicting orphaned VRAM buffers (the ~150-240 MB of
// freed-but-retained texture cache) into GART-backed system RAM — effectively freeing
// physical VRAM headroom without any visible disruption.
//
// At higher modes a display sleep/wake kick is added for deeper reclamation when
// purge alone isn't enough.
type AutoReliefMode int32

const (
	AutoReliefOff        AutoReliefMode = iota // no automatic actions
	AutoReliefGentle                           // purge when VRAM > 80%
	AutoReliefModerate                         // purge > 75%; sleep kick > 88%
	AutoReliefAggressive                       // purge > 70%; sleep kick > 80%
)

var autoReliefModeVal atomic.Int32 // read by pollLoop, written by click handler

// Package-level state — only accessed from the single pollLoop goroutine, no mutex needed.
var (
	arConsecPurge int
	arConsecSleep int
	arLastPurge   time.Time
	arLastSleep   time.Time
)

func autoReliefPrefPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "VRAMGuard", "autorelief")
}

// loadAutoReliefMode reads the persisted mode from disk.
// Returns AutoReliefOff if the file is missing or invalid.
func loadAutoReliefMode() AutoReliefMode {
	data, err := os.ReadFile(autoReliefPrefPath())
	if err != nil {
		return AutoReliefOff
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < int(AutoReliefOff) || n > int(AutoReliefAggressive) {
		return AutoReliefOff
	}
	return AutoReliefMode(n)
}

// saveAutoReliefMode persists the current mode to disk.
func saveAutoReliefMode(m AutoReliefMode) {
	dir := filepath.Dir(autoReliefPrefPath())
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(autoReliefPrefPath(), []byte(strconv.Itoa(int(m))), 0644)
}

func currentAutoReliefMode() AutoReliefMode {
	return AutoReliefMode(autoReliefModeVal.Load())
}

// cycleAutoReliefMode advances to the next mode, persists it, and returns the new label.
func cycleAutoReliefMode() string {
	next := (AutoReliefMode(autoReliefModeVal.Load()) + 1) % 4
	autoReliefModeVal.Store(int32(next))
	saveAutoReliefMode(next)
	label := autoReliefLabel(next)
	dbgf("autoRelief mode → %s", label)
	return label
}

func autoReliefLabel(m AutoReliefMode) string {
	switch m {
	case AutoReliefGentle:
		return "Auto-Relief: Gentle  (purge >80%)"
	case AutoReliefModerate:
		return "Auto-Relief: Moderate  (purge >75%, sleep >88%)"
	case AutoReliefAggressive:
		return "Auto-Relief: Aggressive  (purge >70%, sleep >80%)"
	default:
		return "Auto-Relief: Off"
	}
}

// checkAutoRelief is called once per poll tick from pollLoop.
// It fires non-disruptive recovery actions automatically based on the configured mode.
func checkAutoRelief(pct int) {
	mode := currentAutoReliefMode()
	if mode == AutoReliefOff {
		arConsecPurge = 0
		arConsecSleep = 0
		return
	}

	var purgeThresh, sleepThresh int
	switch mode {
	case AutoReliefGentle:
		purgeThresh = 80
		sleepThresh = 0 // no auto sleep in gentle mode
	case AutoReliefModerate:
		purgeThresh = 75
		sleepThresh = 88
	case AutoReliefAggressive:
		purgeThresh = 70
		sleepThresh = 80
	}

	// Count consecutive polls above each threshold
	if pct >= purgeThresh {
		arConsecPurge++
	} else {
		arConsecPurge = 0
	}
	if sleepThresh > 0 && pct >= sleepThresh {
		arConsecSleep++
	} else {
		arConsecSleep = 0
	}

	now := time.Now()

	// Purge: trigger after 2 consecutive polls (10 s), cooldown 2 min
	if arConsecPurge >= 2 && now.Sub(arLastPurge) > 2*time.Minute {
		arLastPurge = now
		arConsecPurge = 0
		dbgf("autoRelief: purging inactive memory (VRAM %d%%)", pct)
		go purgeInactive()
	}

	// Display sleep kick: trigger after 4 consecutive polls (20 s), cooldown 5 min
	if sleepThresh > 0 && arConsecSleep >= 4 && now.Sub(arLastSleep) > 5*time.Minute {
		arLastSleep = now
		arConsecSleep = 0
		dbgf("autoRelief: display sleep kick (VRAM %d%%)", pct)
		go displaySleepKick()
	}
}
