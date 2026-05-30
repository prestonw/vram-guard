# VRAM Guard

A macOS menu bar app that monitors GPU memory on OpenCore Legacy Patcher (OCLP) systems. Built for the 2011 iMac's AMD Radeon HD 6750M/6970M with 512 MB VRAM, which can hit the ceiling under Sonoma's Metal compositor.

**License:** MIT &nbsp;|&nbsp; **Version:** 1.0.0

---

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
- **Launch at Login** — toggle via the menu; installs a LaunchAgent to `~/Library/LaunchAgents/`
- **About** — version info and GitHub link
- **Uninstall** — removes the LaunchAgent and moves the app to Trash in one click
- **Debug toggle** — verbose logging to `~/Library/Logs/VRAMGuard/vramguard.log`, off by default

---

## Installation

### Option 1 — pkg installer (recommended)

Download `VRAMGuard-1.0.0.pkg` from the [Releases](https://github.com/prestonw/vram-guard/releases) page and double-click to install to `/Applications`.

### Option 2 — install.sh (build from source)

Requires Go 1.22+.

```bash
git clone https://github.com/prestonw/vram-guard
cd vram-guard
./install.sh
```

Installs to `~/Applications/VRAMGuard.app` and launches immediately.

### Option 3 — Makefile

```bash
make install       # build, bundle, install to ~/Applications, and open
make run           # build, bundle, and open locally (dev)
make pkg           # build distributable VRAMGuard-1.0.0.pkg in dist/
```

---

## Launch at Login

Click the VRAM Guard menu bar icon and select **Launch at Login: Off** to toggle it on. This installs a LaunchAgent plist to `~/Library/LaunchAgents/com.prestonw.vramguard.plist`.

To disable, click the same item again (**Launch at Login: On ✓** → off).

---

## Uninstall

### Via the menu (recommended)

Click the menu bar icon → **Uninstall VRAM Guard…** → confirm. This removes the LaunchAgent and moves the app to Trash.

### Via Makefile

```bash
make uninstall
```

### Manually

```bash
launchctl unload ~/Library/LaunchAgents/com.prestonw.vramguard.plist
rm ~/Library/LaunchAgents/com.prestonw.vramguard.plist
rm -rf ~/Applications/VRAMGuard.app   # or /Applications/VRAMGuard.app
```

Logs are left in `~/Library/Logs/VRAMGuard/` — remove manually if desired.

---

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

---

## Requirements

- macOS Ventura 13.0+ (darwin only — uses CGo via Cocoa)
- Go 1.22+ (build from source only)

---

## Build

```bash
go mod tidy
go build -o vram-guard .
```

See `make help` for all available targets.

---

## How VRAM is measured

VRAM is read from `IOAccelerator` via `ioreg`:

- `inUseVidMemoryBytes` — used discrete VRAM (AMD block only)
- `vramFreeBytes` — free discrete VRAM (first occurrence = AMD block)
- Total = used + free (driver-managed pool, ~438 MB on 512 MB hardware — ~74 MB is firmware-reserved)
- `inUseSysMemoryBytes` — system RAM currently backing GPU overflow via GART
- `orphanedReusableVidMemoryBytes` — VRAM held idle by the driver, reclaimable via purge pressure

The "Top GPU Consumers" list uses RSS (resident memory) as a practical proxy since per-process VRAM counters require root access. GPU-heavy apps consistently dominate both RSS and VRAM.

---

## Stack

- [fyne.io/systray](https://github.com/fyne-io/systray) v1.11.0
- No database, no network, no background daemons

## License

[MIT](LICENSE) — Will Preston, 2026
