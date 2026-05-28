//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"regexp"
	"strconv"
)

// VRAMStats holds current GPU memory state.
type VRAMStats struct {
	UsedBytes  int64
	TotalBytes int64
	Percent    int
}

// Real key names confirmed via `ioreg -r -w 0 -c IOAccelerator | grep -i vram` on target machine.
//
// AMD Radeon discrete GPU (2011 iMac):
//   "inUseVidMemoryBytes" = used discrete VRAM
//   "vramFreeBytes"       = free discrete VRAM (first occurrence = AMD entry, before Intel entry)
//
// Intel HD 3000 integrated GPU (second entry in output):
//   "vramUsedBytes" = used shared VRAM (tiny; irrelevant for our purposes)
var (
	// inUseVidMemoryBytes only appears in the AMD discrete GPU block.
	reUsed = regexp.MustCompile(`"inUseVidMemoryBytes"\s*=\s*(\d+)`)
	// vramFreeBytes appears in both AMD and Intel blocks; first match is AMD.
	reFree = regexp.MustCompile(`"vramFreeBytes"\s*=\s*(\d+)`)
)

// getVRAMStats queries ioreg for live VRAM usage on the AMD discrete GPU.
// Total is derived from used+free (the driver-managed pool, ~438 MB on 512 MB hardware).
// This is more accurate than the declared 512 MB because ~74 MB is firmware-reserved.
func getVRAMStats() (VRAMStats, error) {
	out, err := exec.Command("ioreg", "-r", "-w", "0", "-c", "IOAccelerator").Output()
	if err != nil {
		return VRAMStats{}, fmt.Errorf("ioreg: %w", err)
	}

	text := string(out)
	usedMatch := reUsed.FindStringSubmatch(text)
	freeMatch := reFree.FindStringSubmatch(text)

	if len(usedMatch) < 2 {
		return VRAMStats{}, fmt.Errorf("inUseVidMemoryBytes not found — GPU driver may not expose it via IOAccelerator")
	}
	if len(freeMatch) < 2 {
		return VRAMStats{}, fmt.Errorf("vramFreeBytes not found in IOAccelerator output")
	}

	used, _ := strconv.ParseInt(usedMatch[1], 10, 64)
	free, _ := strconv.ParseInt(freeMatch[1], 10, 64)
	total := used + free

	if total == 0 {
		return VRAMStats{}, fmt.Errorf("total VRAM is zero — unexpected ioreg values")
	}

	pct := int(float64(used) / float64(total) * 100)
	if pct > 100 {
		pct = 100
	}
	return VRAMStats{UsedBytes: used, TotalBytes: total, Percent: pct}, nil
}

// makeIcon returns a minimal 16x16 dark-gray PNG for the menu bar icon.
// macOS renders the SetTitle() text beside it, so the icon is intentionally minimal.
func makeIcon() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	fg := color.RGBA{60, 60, 60, 255}
	bg := color.RGBA{0, 0, 0, 0}

	// Simple GPU "chip" outline: border square with two vertical pin lines
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			onBorder := x == 1 || x == 14 || y == 1 || y == 14
			isPins := (x == 5 || x == 10) && y >= 4 && y <= 11
			if onBorder || isPins {
				img.Set(x, y, fg)
			} else {
				img.Set(x, y, bg)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
