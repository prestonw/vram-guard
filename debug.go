//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Log file: ~/Library/Logs/VRAMGuard/vramguard.log
// Rotated to vramguard.log.1 when it exceeds 2 MB.
// Open with: open -a Console ~/Library/Logs/VRAMGuard/vramguard.log

const (
	logMaxBytes = 2 * 1024 * 1024 // 2 MB before rotation
)

var (
	dbgFile *os.File
	dbgMu   sync.Mutex
	dbgPath string
)

// initDebugLog opens (or creates) the log file and writes a session banner.
// Call once at startup before any other dbg* calls.
func initDebugLog() {
	dir := filepath.Join(os.Getenv("HOME"), "Library", "Logs", "VRAMGuard")
	_ = os.MkdirAll(dir, 0755)
	dbgPath = filepath.Join(dir, "vramguard.log")

	rotateLogs(dbgPath)

	f, err := os.OpenFile(dbgPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return // silent — logging is best-effort
	}
	dbgFile = f

	dbgSection("SESSION START")
	dbgf("Time:       %s", time.Now().Format("2006-01-02 15:04:05 MST"))
	dbgf("Go runtime: %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// macOS version
	dbgRawCmd("sw_vers")
	// Hardware model + RAM
	dbgRawCmd("sysctl", "-n", "hw.model")
	dbgRawCmd("sysctl", "-n", "hw.memsize")
	// Disk / EFI partition presence
	dbgRawCmd("diskutil", "list")
	// GPU device tree path (helps verify PCI path)
	dbgRawCmd("ioreg", "-r", "-w", "0", "-c", "IOAccelerator", "-l")
}

func rotateLogs(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < logMaxBytes {
		return
	}
	_ = os.Rename(path, path+".1")
}

// dbgSection writes a prominent section header.
func dbgSection(name string) {
	line := "\n" + strings.Repeat("=", 60) + "\n"
	line += fmt.Sprintf("  %s  —  %s\n", name, time.Now().Format("15:04:05"))
	line += strings.Repeat("=", 60) + "\n"
	dbgWrite(line)
}

// dbgf writes a timestamped log line.
func dbgf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	dbgWrite(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05.000"), msg))
}

// dbgCmd logs a labelled command result: the command itself, combined output, and error.
func dbgCmd(label string, out []byte, err error) {
	dbgf("CMD  %s", label)
	if err != nil {
		dbgf("ERR  %v", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		dbgf("OUT  (empty)")
	} else {
		// Indent each output line
		for _, line := range strings.Split(trimmed, "\n") {
			dbgWrite("       " + line + "\n")
		}
	}
}

// dbgScript logs the full body of a generated script (Python, shell, AppleScript).
func dbgScript(label, body string) {
	dbgf("SCRIPT  %s", label)
	dbgWrite("--- BEGIN ---\n")
	for _, line := range strings.Split(body, "\n") {
		dbgWrite("  " + line + "\n")
	}
	dbgWrite("--- END ---\n")
}

// dbgRawCmd runs a command purely for logging purposes and discards result errors.
func dbgRawCmd(name string, args ...string) {
	out, err := exec.Command(name, args...).CombinedOutput()
	dbgCmd(name+" "+strings.Join(args, " "), out, err)
}

func dbgWrite(s string) {
	dbgMu.Lock()
	defer dbgMu.Unlock()
	if dbgFile != nil {
		_, _ = dbgFile.WriteString(s)
	}
}

// debugLogPath returns the path to the current log file.
func debugLogPath() string {
	return dbgPath
}

// openDebugLog opens the log file in Console.app.
func openDebugLog() {
	if dbgPath == "" {
		osNotify("VRAM Guard", "Debug log not initialised yet")
		return
	}
	exec.Command("open", "-a", "Console", dbgPath).Run() //nolint:errcheck
}
