package config

import (
	"os"
	"strings"
	"time"
)

type Route struct {
	Upstream string
}

type Config struct {
	Addr           string
	CacheDir       string
	RequestTimeout time.Duration
	MetadataTTL    time.Duration
	Routes         map[string]Route
}

func LoadFromEnv() Config {
	cfg := Config{
		Addr:           ":8080",
		CacheDir:       "/var/cache/openmirror",
		RequestTimeout: 20 * time.Second,
		MetadataTTL:    10 * time.Minute,
		Routes: map[string]Route{
			"docker": {Upstream: "https://registry-1.docker.io"},
			"npm":    {Upstream: "https://registry.npmjs.org"},
			"pypi":   {Upstream: "https://pypi.org"},
		},
	}

	if v := os.Getenv("OPENMIRROR_ADDR"); v != "" {
		cfg.Addr = v
	}

	if v := os.Getenv("OPENMIRROR_CACHE_DIR"); v != "" {
		cfg.CacheDir = v
	}

	if v := os.Getenv("OPENMIRROR_REQUEST_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RequestTimeout = d
		}
	}

	if v := os.Getenv("OPENMIRROR_METADATA_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.MetadataTTL = d
		}
	}

	for name, route := range cfg.Routes {
		envKey := "OPENMIRROR_ROUTE_" + strings.ToUpper(name) + "_UPSTREAM"
		if v := os.Getenv(envKey); v != "" {
			route.Upstream = v
			cfg.Routes[name] = route
		}
	}

	return cfg
}
