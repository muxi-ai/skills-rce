//go:build linux

package sysinfo

import "syscall"

func detectMemoryMB() int64 {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err == nil {
		return int64(info.Totalram) * int64(info.Unit) / (1024 * 1024)
	}
	return 0
}
