package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           int
	CacheDir       string
	DefaultTimeout int
	MaxTimeout     int
	MaxOutputBytes int
	AuthToken      string
}

func Load() *Config {
	cfg := &Config{
		Port:           5580,
		CacheDir:       "/cache/skills",
		DefaultTimeout: 30,
		MaxTimeout:     300,
		MaxOutputBytes: 100 * 1024,
		AuthToken:      "",
	}

	if v := os.Getenv("RCE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := os.Getenv("RCE_CACHE_DIR"); v != "" {
		cfg.CacheDir = v
	}
	if v := os.Getenv("RCE_DEFAULT_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DefaultTimeout = n
		}
	}
	if v := os.Getenv("RCE_MAX_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxTimeout = n
		}
	}
	if v := os.Getenv("RCE_AUTH_TOKEN"); v != "" {
		cfg.AuthToken = v
	}

	return cfg
}
