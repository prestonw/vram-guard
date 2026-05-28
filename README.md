# VRAM Guard

A macOS menu bar app that monitors GPU memory on OpenCore Legacy Patcher (OCLP) systems. Built for the 2011 iMac's AMD Radeon HD 6750M/6970M with 512 MB VRAM, which can hit the ceiling under Sonoma's Metal compositor.

## Features

- **Live VRAM %** in the menu bar — updates every 5 seconds
- **Threshold alerts**: `!75%` at ≥75%, `!!90%` at ≥90%
- **OS notification** when crossing 85% from below
- **Top GPU Consumers** — top 5 processes by memory, click any to force-quit with confirmation
- **Recovery actions** (ordered least to most disruptive):
  1. Display Sleep/Wake Kick — `pmset displaysleepnow`, may unstick a hung Metal compositor
  2. Gentle WindowServer Reload — `SIGHUP`, restarts compositor without logging you out
  3. Restart WindowServer ⚠ — `SIGKILL`, frees all VRAM but logs you out

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

The "Top GPU Consumers" list uses RSS (resident memory) as a practical proxy since per-process VRAM counters require root access. GPU-heavy apps consistently dominate both RSS and VRAM.

## Stack

- [fyne.io/systray](https://github.com/fyne-io/systray) v1.11.0
- No database, no network, no background daemons
