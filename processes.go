//go:build darwin

package main

import (
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ProcessInfo holds per-process resident memory, used as a VRAM consumption proxy.
type ProcessInfo struct {
	PID  int
	Name string
	RSS  int64 // bytes
}

var (
	currentProcs   []ProcessInfo
	currentProcsMu sync.Mutex
)

// getTopProcesses returns the top n user processes by RSS.
// RSS is the best available per-process VRAM proxy without root privileges —
// GPU-heavy apps (browsers, creative tools) consistently top both RSS and VRAM.
func getTopProcesses(n int) []ProcessInfo {
	out, err := exec.Command("ps", "-A", "-o", "pid=,rss=,comm=").Output()
	if err != nil {
		return nil
	}

	selfPID := os.Getpid()
	var procs []ProcessInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 1 || pid == selfPID {
			continue
		}
		rss, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || rss < 10240 { // skip < 10 MB
			continue
		}
		name := fields[2]
		if strings.HasPrefix(name, "(") {
			continue // kernel thread placeholder
		}
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		procs = append(procs, ProcessInfo{PID: pid, Name: name, RSS: rss * 1024})
	}

	sort.Slice(procs, func(i, j int) bool { return procs[i].RSS > procs[j].RSS })
	if len(procs) > n {
		procs = procs[:n]
	}
	return procs
}

// killProcAtIndex confirms with the user then kills the process at slot i.
func killProcAtIndex(i int) {
	currentProcsMu.Lock()
	if i >= len(currentProcs) {
		currentProcsMu.Unlock()
		return
	}
	p := currentProcs[i]
	currentProcsMu.Unlock()
	confirmAndKill(p.PID, p.Name)
}

func truncateName(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
