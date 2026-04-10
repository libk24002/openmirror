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

	pypiFiles, ok := cfg.Routes["pypiFiles"]
	if !ok {
		t.Fatalf("pypiFiles route missing")
	}
	if pypiFiles.Upstream != "https://files.pythonhosted.org" {
		t.Fatalf("pypiFiles upstream = %q, want %q", pypiFiles.Upstream, "https://files.pythonhosted.org")
	}
}

func TestLoadFromEnvSupportsPyPIFilesUpstreamOverride(t *testing.T) {
	t.Setenv("OPENMIRROR_ROUTE_PYPIFILES_UPSTREAM", "https://files.example.com")

	cfg := LoadFromEnv()

	if got := cfg.Routes["pypiFiles"].Upstream; got != "https://files.example.com" {
		t.Fatalf("pypiFiles upstream = %q, want %q", got, "https://files.example.com")
	}
}
