//go:build darwin

package sysinfo

import (
	"os/exec"
	"strconv"
	"strings"
)

func detectMemoryMB() int64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return bytes / (1024 * 1024)
}
