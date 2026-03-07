package sysinfo

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Packages struct {
	Python []Package `json:"python,omitempty"`
	Node   []Package `json:"node,omitempty"`
}

type Info struct {
	Runtimes  []Runtime `json:"runtimes"`
	Languages []string  `json:"languages"`
	Resources Resources `json:"resources"`
	Packages  Packages  `json:"packages"`
}

var runtimeProbes = []struct {
	name  string
	bin   string
	args  []string
	langs []string // languages this runtime enables
}{
	// Language runtimes
	{"python", "python3", []string{"--version"}, []string{"python"}},
	{"bun", "bun", []string{"--version"}, []string{"javascript", "typescript"}},
	{"bash", "bash", []string{"--version"}, []string{"bash"}},
	{"go", "go", []string{"version"}, []string{"go"}},
	{"php", "php", []string{"--version"}, []string{"php"}},
	{"ruby", "ruby", []string{"--version"}, []string{"ruby"}},
	{"perl", "perl", []string{"--version"}, []string{"perl"}},
	{"lua", "lua", []string{"-v"}, []string{"lua"}},
	{"r", "Rscript", []string{"--version"}, []string{"r"}},
	// Tools (no languages, just presence detection)
	{"node", "node", []string{"--version"}, nil},
	{"npx", "npx", []string{"--version"}, nil},
	{"uv", "uv", []string{"--version"}, nil},
	{"pip", "pip3", []string{"--version"}, nil},
}

func Detect() *Info {
	info := &Info{}

	for _, probe := range runtimeProbes {
		rt := Runtime{Name: probe.name}
		out, err := exec.Command(probe.bin, probe.args...).Output()
		if err == nil {
			v := parseVersion(probe.name, strings.TrimSpace(string(out)))
			rt.Version = &v
			if probe.langs != nil {
				info.Languages = append(info.Languages, probe.langs...)
			}
		}
		info.Runtimes = append(info.Runtimes, rt)
	}

	info.Resources = detectResources()
	info.Packages = detectPackages()
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
	case "uv":
		// "uv 0.4.18" -> "0.4.18"
		if parts := strings.Fields(raw); len(parts) >= 2 {
			return parts[1]
		}
	case "pip":
		// "pip 22.0.2 from /usr/lib/..." -> "22.0.2"
		if parts := strings.Fields(raw); len(parts) >= 2 {
			return parts[1]
		}
	case "node", "npx":
		// "v20.11.1" -> "20.11.1"
		return strings.TrimPrefix(raw, "v")
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

func detectPackages() Packages {
	return Packages{
		Python: detectPythonPackages(),
		Node:   detectNodePackages(),
	}
}

func detectPythonPackages() []Package {
	out, err := exec.Command("pip3", "list", "--format=json").Output()
	if err != nil {
		return nil
	}
	var raw []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}
	pkgs := make([]Package, len(raw))
	for i, r := range raw {
		pkgs[i] = Package{Name: r.Name, Version: r.Version}
	}
	return pkgs
}

func detectNodePackages() []Package {
	// Read from the pre-installed bun packages location
	for _, dir := range []string{"/opt/bun-packages", os.Getenv("NODE_PATH") + "/.."} {
		pkgFile := filepath.Join(dir, "package.json")
		data, err := os.ReadFile(pkgFile)
		if err != nil {
			continue
		}
		var pkg struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			continue
		}
		pkgs := make([]Package, 0, len(pkg.Dependencies))
		for name, version := range pkg.Dependencies {
			pkgs = append(pkgs, Package{Name: name, Version: strings.TrimPrefix(version, "^")})
		}
		return pkgs
	}
	return nil
}
