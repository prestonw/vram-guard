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
- **Expand VRAM** — patches OpenCore `DeviceProperties` to declare more VRAM to the AMD kext, backed by system RAM via GART. Offers sizes from 512 MB up to half of installed RAM. Requires reboot.
- **Revert VRAM Expansion** — removes the injected `VRAM,totalsize` key from OpenCore config. Requires reboot.

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

## VRAM Expansion

The 2011 iMac's AMD Radeon has 512 MB of dedicated VRAM, which Sonoma's Metal compositor can exhaust. The expansion feature works by injecting `VRAM,totalsize` into your OpenCore `DeviceProperties` for the GPU's PCI path, causing the AMD kext to declare a larger pool. The extra allocation is backed by system RAM via GART (Graphics Address Remapping Table) — the same mechanism the GPU already uses for texture overflow.

The feature scans your existing OpenCore config for an AMD GPU path before falling back to the known 2011 iMac default (`PciRoot(0x0)/Pci(0x1,0x0)/Pci(0x0,0x0)`). It offers sizes up to half of your installed RAM, leaving the rest for macOS.

> **Note:** This requires an OCLP system with OpenCore on the EFI partition. It does not modify any kexts or SIP settings.

## How VRAM is measured

VRAM is read from `IOAccelerator` via `ioreg`:

- `inUseVidMemoryBytes` — used discrete VRAM (AMD block only)
- `vramFreeBytes` — free discrete VRAM (first occurrence = AMD block)
- Total = used + free (driver-managed pool, ~438 MB on 512 MB hardware — ~74 MB is firmware-reserved)

The "Top GPU Consumers" list uses RSS (resident memory) as a practical proxy since per-process VRAM counters require root access. GPU-heavy apps consistently dominate both RSS and VRAM.

## Stack

- [fyne.io/systray](https://github.com/fyne-io/systray) v1.11.0
- No database, no network, no background daemons
