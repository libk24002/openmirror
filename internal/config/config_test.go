package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	cfg := LoadFromEnv()

	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, ":8080")
	}

	docker, ok := cfg.Routes["docker"]
	if !ok {
		t.Fatalf("docker route missing")
	}

	if docker.Upstream == "" {
		t.Fatalf("docker upstream is empty")
	}

	if cfg.MetadataTTL != 10*time.Minute {
		t.Fatalf("MetadataTTL = %s, want %s", cfg.MetadataTTL, 10*time.Minute)
	}
}
