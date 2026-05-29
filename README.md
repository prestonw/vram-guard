# VRAM Guard

A macOS menu bar app that monitors GPU memory on OpenCore Legacy Patcher (OCLP) systems. Built for the 2011 iMac's AMD Radeon HD 6750M/6970M with 512 MB VRAM, which can hit the ceiling under Sonoma's Metal compositor.

## Features

- **Live VRAM %** in the menu bar — updates every 5 seconds
- **Threshold alerts**: `!75%` at ≥75%, `!!90%` at ≥90%
- **Top GPU Consumers** — top 5 processes by memory, click any to force-quit with confirmation
- **Recovery actions** (ordered least to most disruptive):
  1. Display Sleep/Wake Kick — `pmset displaysleepnow`, may unstick a hung Metal compositor
  2. Gentle WindowServer Reload — `SIGHUP`, restarts compositor without logging you out
  3. Restart WindowServer ⚠ — `SIGKILL`, frees all VRAM but logs you out
- **Auto-Relief** — automatically reclaims orphaned VRAM in the background, configurable in four levels
- **GPU Details** — dialog showing used/free physical VRAM, system RAM GPU overflow, and reclaimable orphaned VRAM
- **Debug toggle** — verbose logging to `~/Library/Logs/VRAMGuard/vramguard.log`, off by default

## Auto-Relief

The AMD GPU kext holds idle texture buffers in physical VRAM (`orphanedReusableVidMemoryBytes`) even after the app that created them has released them. Running `/usr/sbin/purge` flushes the inactive file-system page cache, which raises VM pressure. The AMD kext responds by evicting these orphaned buffers into GART-backed system RAM — freeing physical VRAM headroom without any visible disruption.

Four levels are available (click the menu item to cycle):

| Level | Purge threshold | Display sleep kick |
|---|---|---|
| Off | — | — |
| Gentle | VRAM > 80% | — |
| Moderate | VRAM > 75% | VRAM > 88% |
| Aggressive | VRAM > 70% | VRAM > 80% |

Purge fires after 2 consecutive polls above the threshold (10 s), with a 2-minute cooldown. Display sleep/wake kick fires after 4 consecutive polls (20 s), with a 5-minute cooldown.

## Requirements

- macOS (darwin only — uses CGo via Cocoa)
- Go 1.22+

## Build

```bash
go mod tidy
go build -o vram-guard .
./vram-guard
```

## Run as a proper menu-bar-only app (no Dock icon)

```bash
mkdir -p VRAMGuard.app/Contents/MacOS
cp vram-guard VRAMGuard.app/Contents/MacOS/VRAMGuard
/usr/libexec/PlistBuddy -c "Add :LSUIElement bool true" VRAMGuard.app/Contents/Info.plist
open VRAMGuard.app
```

Then add `VRAMGuard.app` to **System Settings → General → Login Items** to launch at login.

## How VRAM is measured

VRAM is read from `IOAccelerator` via `ioreg`:

- `inUseVidMemoryBytes` — used discrete VRAM (AMD block only)
- `vramFreeBytes` — free discrete VRAM (first occurrence = AMD block)
- Total = used + free (driver-managed pool, ~438 MB on 512 MB hardware — ~74 MB is firmware-reserved)
- `inUseSysMemoryBytes` — system RAM currently backing GPU overflow via GART
- `orphanedReusableVidMemoryBytes` — VRAM held idle by the driver, reclaimable via purge pressure

The "Top GPU Consumers" list uses RSS (resident memory) as a practical proxy since per-process VRAM counters require root access. GPU-heavy apps consistently dominate both RSS and VRAM.

## Stack

- [fyne.io/systray](https://github.com/fyne-io/systray) v1.11.0
- No database, no network, no background daemons
