package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// acquireSingleInstance kills any prior process recorded in pidPath and
// registers the current process as the sole owner of that pidfile.
func acquireSingleInstance(pidPath string) error {
	if data, err := os.ReadFile(pidPath); err == nil {
		if oldPid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && oldPid > 0 {
			killPreviousInstance(oldPid)
		}
	}

	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func killPreviousInstance(pid int) {
	if syscall.Kill(pid, 0) != nil {
		return // not alive
	}

	_ = syscall.Kill(pid, syscall.SIGTERM)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// releasePidFile removes pidPath, but only if it still records ownPid —
// avoids clobbering a newer instance's pidfile during a takeover race.
func releasePidFile(pidPath string, ownPid int) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	if strings.TrimSpace(string(data)) != fmt.Sprint(ownPid) {
		return
	}
	_ = os.Remove(pidPath)
}
