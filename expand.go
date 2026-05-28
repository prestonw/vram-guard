//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// getSystemRAMMB returns total installed RAM in MB via sysctl.
func getSystemRAMMB() int64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	b, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return b / (1024 * 1024)
}

// showExpandVRAMDialog presents an info screen + size picker, then applies the expansion.
// Expansion works by patching OpenCore DeviceProperties to declare more VRAM to the AMD kext.
// The GPU uses system RAM (via GART) to back the extra allocation. Requires reboot.
func showExpandVRAMDialog() {
	totalRAM := getSystemRAMMB()
	stats, _ := getVRAMStats()
	currentMB := stats.TotalBytes / (1024 * 1024)
	if currentMB == 0 {
		currentMB = 512
	}

	// Build preset list: offer sizes above current VRAM, capped to half of system RAM
	// (leave at least half of RAM for macOS and apps)
	presets := []int64{512, 1024, 2048, 3072, 4096}
	var labels []string
	var values []int64
	for _, p := range presets {
		if p <= currentMB {
			continue
		}
		if p > totalRAM/2 {
			continue
		}
		label := fmt.Sprintf("%d MB", p)
		if p%1024 == 0 {
			label = fmt.Sprintf("%d GB", p/1024)
		}
		labels = append(labels, label)
		values = append(values, p)
	}

	if len(labels) == 0 {
		script := fmt.Sprintf(
			`display dialog "No expansion options available.\n\nCurrent VRAM: %d MB\nSystem RAM:   %d MB\n\nYou need at least twice the target VRAM in system RAM (half reserved for macOS)." `+
				`with title "VRAM Guard \u2014 Expand VRAM" buttons {"OK"} default button "OK"`,
			currentMB, totalRAM,
		)
		exec.Command("osascript", "-e", script).Run() //nolint:errcheck
		return
	}

	// Info / warning screen
	infoScript := fmt.Sprintf(
		`display dialog "VRAM Expansion patches your OpenCore bootloader config to declare more VRAM to the AMD GPU driver. The extra allocation is backed by system RAM via GART.\n\nCurrent VRAM:  %d MB\nSystem RAM:    %d MB\nMax safe offer: %d MB (half of RAM)\n\nA reboot is required after applying. Only do this on an OCLP system with OpenCore." `+
			`with title "VRAM Guard \u2014 Expand VRAM" buttons {"Cancel", "Choose Size \u2192"} default button "Choose Size \u2192"`,
		currentMB, totalRAM, totalRAM/2,
	)
	infoOut, _ := exec.Command("osascript", "-e", infoScript).CombinedOutput()
	if !strings.Contains(string(infoOut), "Choose Size") {
		return
	}

	// Size picker via choose from list
	quotedLabels := make([]string, len(labels))
	for i, l := range labels {
		quotedLabels[i] = `"` + l + `"`
	}
	pickScript := fmt.Sprintf(
		`choose from list {%s} with title "VRAM Guard \u2014 Select VRAM Size" `+
			`with prompt "Current VRAM: %d MB  |  System RAM: %d MB\nChoose new total VRAM allocation:" `+
			`default items {"%s"} without multiple selections allowed`,
		strings.Join(quotedLabels, ", "), currentMB, totalRAM, labels[0],
	)
	pickOut, err := exec.Command("osascript", "-e", pickScript).CombinedOutput()
	chosen := strings.TrimSpace(string(pickOut))
	if err != nil || chosen == "false" || chosen == "" {
		return
	}

	var targetMB int64
	for i, l := range labels {
		if l == chosen {
			targetMB = values[i]
			break
		}
	}
	if targetMB == 0 {
		return
	}

	applyVRAMExpansion(targetMB, currentMB)
}

// applyVRAMExpansion writes VRAM,totalsize into OpenCore config.plist via a
// Python helper script run with administrator privileges.
func applyVRAMExpansion(targetMB, currentMB int64) {
	targetBytes := targetMB * 1024 * 1024
	tmpPath := fmt.Sprintf("/tmp/vramguard_apply_%d.py", os.Getpid())

	// Python 3 is always available on macOS Sonoma (/usr/bin/python3).
	// plistlib handles both XML and binary OpenCore configs automatically.
	py := fmt.Sprintf(`import plistlib, struct, subprocess, sys, os

efi = '/Volumes/EFI'
subprocess.run(['diskutil', 'mount', 'EFI'], capture_output=True)

cfg = efi + '/EFI/OC/config.plist'
if not os.path.exists(cfg):
    subprocess.run(['diskutil', 'unmount', efi], capture_output=True)
    print('ERROR: OpenCore config not found at ' + cfg + ' — is this an OCLP system with OpenCore?')
    sys.exit(1)

with open(cfg, 'rb') as f:
    config = plistlib.load(f)

# Default PCI path for 2011 iMac dGPU; scan existing entries first in case
# a custom path is already set (e.g. from a previous OCLP patch).
gpu = 'PciRoot(0x0)/Pci(0x1,0x0)/Pci(0x0,0x0)'
for p in config.get('DeviceProperties', {}).get('Add', {}):
    if 'Pci(0x1' in p:
        gpu = p
        break

config.setdefault('DeviceProperties', {}).setdefault('Add', {}).setdefault(gpu, {})['VRAM,totalsize'] = struct.pack('<Q', %d)

with open(cfg, 'wb') as f:
    plistlib.dump(config, f, fmt=plistlib.FMT_XML)

subprocess.run(['diskutil', 'unmount', efi], capture_output=True)
print('OK:' + gpu)
`, targetBytes)

	if err := os.WriteFile(tmpPath, []byte(py), 0644); err != nil {
		osNotify("VRAM Guard", "Could not write expansion script: "+err.Error())
		return
	}
	defer os.Remove(tmpPath)

	out, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`do shell script "/usr/bin/python3 %s" with administrator privileges`, tmpPath),
	).CombinedOutput()

	result := strings.TrimSpace(string(out))
	if err != nil || !strings.HasPrefix(result, "OK:") {
		msg := result
		if msg == "" {
			msg = err.Error()
		}
		script := fmt.Sprintf(
			`display dialog "Expansion failed:\n\n%s" `+
				`with title "VRAM Guard" buttons {"OK"} default button "OK"`,
			strings.ReplaceAll(msg, `"`, `\"`),
		)
		exec.Command("osascript", "-e", script).Run() //nolint:errcheck
		return
	}

	gpuPath := strings.TrimPrefix(result, "OK:")
	doneScript := fmt.Sprintf(
		`display dialog "VRAM expanded to %d MB (was %d MB).\n\nInjected into: %s\n\nReboot to apply the change." `+
			`with title "VRAM Guard \u2014 Done" buttons {"Later", "Reboot Now"} default button "Later"`,
		targetMB, currentMB, gpuPath,
	)
	rebootOut, _ := exec.Command("osascript", "-e", doneScript).CombinedOutput()
	if strings.Contains(string(rebootOut), "Reboot Now") {
		exec.Command("osascript", "-e",
			`do shell script "shutdown -r now" with administrator privileges`,
		).Run() //nolint:errcheck
	}
}

// revertVRAMExpansion removes the VRAM,totalsize key from OpenCore config.plist.
func revertVRAMExpansion() {
	confirmScript := `display dialog "This removes the VRAM expansion from your OpenCore config and restores the default VRAM size.\n\nA reboot is required to take effect." ` +
		`with title "VRAM Guard \u2014 Revert Expansion" buttons {"Cancel", "Revert"} default button "Cancel"`
	out, _ := exec.Command("osascript", "-e", confirmScript).CombinedOutput()
	if !strings.Contains(string(out), "button returned:Revert") {
		return
	}

	tmpPath := fmt.Sprintf("/tmp/vramguard_revert_%d.py", os.Getpid())
	py := `import plistlib, subprocess, sys, os

efi = '/Volumes/EFI'
subprocess.run(['diskutil', 'mount', 'EFI'], capture_output=True)

cfg = efi + '/EFI/OC/config.plist'
if not os.path.exists(cfg):
    subprocess.run(['diskutil', 'unmount', efi], capture_output=True)
    print('ERROR: OpenCore config not found')
    sys.exit(1)

with open(cfg, 'rb') as f:
    config = plistlib.load(f)

changed = False
for props in config.get('DeviceProperties', {}).get('Add', {}).values():
    if 'VRAM,totalsize' in props:
        del props['VRAM,totalsize']
        changed = True

if changed:
    with open(cfg, 'wb') as f:
        plistlib.dump(config, f, fmt=plistlib.FMT_XML)
    print('OK:VRAM expansion removed')
else:
    print('OK:No VRAM expansion was applied')

subprocess.run(['diskutil', 'unmount', efi], capture_output=True)
`

	if err := os.WriteFile(tmpPath, []byte(py), 0644); err != nil {
		osNotify("VRAM Guard", "Could not write revert script: "+err.Error())
		return
	}
	defer os.Remove(tmpPath)

	revertOut, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`do shell script "/usr/bin/python3 %s" with administrator privileges`, tmpPath),
	).CombinedOutput()

	result := strings.TrimSpace(string(revertOut))
	if err != nil || !strings.HasPrefix(result, "OK:") {
		msg := result
		if msg == "" {
			msg = err.Error()
		}
		osNotify("VRAM Guard", "Revert failed: "+msg)
		return
	}

	msg := strings.TrimPrefix(result, "OK:")
	doneScript := fmt.Sprintf(
		`display dialog "%s\n\nReboot to apply." `+
			`with title "VRAM Guard \u2014 Reverted" buttons {"Later", "Reboot Now"} default button "Later"`,
		msg,
	)
	rebootOut, _ := exec.Command("osascript", "-e", doneScript).CombinedOutput()
	if strings.Contains(string(rebootOut), "Reboot Now") {
		exec.Command("osascript", "-e",
			`do shell script "shutdown -r now" with administrator privileges`,
		).Run() //nolint:errcheck
	}
}
