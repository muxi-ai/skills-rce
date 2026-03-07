package sysinfo

import (
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

type Runtime struct {
	Name    string  `json:"name"`
	Version *string `json:"version"`
}

type Resources struct {
	CPUs     int   `json:"cpus"`
	MemoryMB int64 `json:"memory_mb"`
	DiskMB   int64 `json:"disk_mb"`
}

type Info struct {
	Runtimes  []Runtime `json:"runtimes"`
	Languages []string  `json:"languages"`
	Resources Resources `json:"resources"`
}

var runtimeProbes = []struct {
	name    string
	bin     string
	args    []string
	langs   []string
}{
	{"python", "python3", []string{"--version"}, []string{"python"}},
	{"bun", "bun", []string{"--version"}, []string{"javascript", "typescript", "node"}},
	{"bash", "bash", []string{"--version"}, []string{"bash"}},
	{"go", "go", []string{"version"}, []string{"go"}},
	{"php", "php", []string{"--version"}, []string{"php"}},
	{"ruby", "ruby", []string{"--version"}, []string{"ruby"}},
	{"perl", "perl", []string{"--version"}, []string{"perl"}},
	{"lua", "lua", []string{"-v"}, []string{"lua"}},
	{"r", "Rscript", []string{"--version"}, []string{"r"}},
}

func Detect() *Info {
	info := &Info{}

	for _, probe := range runtimeProbes {
		rt := Runtime{Name: probe.name}
		out, err := exec.Command(probe.bin, probe.args...).Output()
		if err == nil {
			v := parseVersion(probe.name, strings.TrimSpace(string(out)))
			rt.Version = &v
			info.Languages = append(info.Languages, probe.langs...)
		}
		info.Runtimes = append(info.Runtimes, rt)
	}

	info.Resources = detectResources()
	return info
}

func parseVersion(name, raw string) string {
	switch name {
	case "python":
		// "Python 3.11.0" -> "3.11.0"
		if parts := strings.Fields(raw); len(parts) >= 2 {
			return parts[1]
		}
	case "go":
		// "go version go1.26.0 darwin/arm64" -> "1.26.0"
		for _, p := range strings.Fields(raw) {
			if strings.HasPrefix(p, "go1") || strings.HasPrefix(p, "go0") {
				return strings.TrimPrefix(p, "go")
			}
		}
	case "php":
		// "PHP 8.3.6 (cli) ..." -> "8.3.6"
		if parts := strings.Fields(raw); len(parts) >= 2 {
			return parts[1]
		}
	case "ruby":
		// "ruby 3.2.2 (2023-03-30 ...) ..." -> "3.2.2"
		if parts := strings.Fields(raw); len(parts) >= 2 {
			return parts[1]
		}
	case "bash":
		// "GNU bash, version 5.1.16(1)-release ..." -> "5.1.16"
		for _, line := range strings.Split(raw, "\n") {
			if strings.Contains(line, "version") {
				parts := strings.Fields(line)
				for i, p := range parts {
					if p == "version" && i+1 < len(parts) {
						v := parts[i+1]
						if idx := strings.Index(v, "("); idx > 0 {
							v = v[:idx]
						}
						return v
					}
				}
			}
		}
	case "perl":
		// "This is perl 5, version 34, subversion 0 (v5.34.0) ..." -> "5.34.0"
		if idx := strings.Index(raw, "(v"); idx >= 0 {
			end := strings.Index(raw[idx:], ")")
			if end > 0 {
				return strings.TrimPrefix(raw[idx:idx+end], "(v")
			}
		}
	case "lua":
		// "Lua 5.4.6  Copyright ..." -> "5.4.6"
		if parts := strings.Fields(raw); len(parts) >= 2 {
			return parts[1]
		}
	case "r":
		// "R scripting front-end version 4.3.1 ..." -> "4.3.1"
		for _, p := range strings.Fields(raw) {
			if len(p) > 0 && p[0] >= '0' && p[0] <= '9' && strings.Contains(p, ".") {
				return p
			}
		}
	}
	return strings.Split(raw, "\n")[0]
}

func detectResources() Resources {
	r := Resources{
		CPUs: runtime.NumCPU(),
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		r.DiskMB = int64(stat.Bavail) * int64(stat.Bsize) / (1024 * 1024)
	}

	r.MemoryMB = detectMemoryMB()

	return r
}
