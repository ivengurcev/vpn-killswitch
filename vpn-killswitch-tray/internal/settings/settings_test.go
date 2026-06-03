package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CommandPath != DefaultCommandPath {
		t.Fatalf("CommandPath = %q", cfg.CommandPath)
	}
	if cfg.PollIntervalSeconds != DefaultPollIntervalSeconds {
		t.Fatalf("PollIntervalSeconds = %d", cfg.PollIntervalSeconds)
	}
	if !cfg.AutoEnforceAfterConfigChange {
		t.Fatalf("bool defaults not enabled: %+v", cfg)
	}
}

func TestLoadOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(path, []byte(`
command_path = "/tmp/vpn-killswitch"
poll_interval_seconds = 10
`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CommandPath != "/tmp/vpn-killswitch" || cfg.PollIntervalSeconds != 10 {
		t.Fatalf("settings = %+v", cfg)
	}
}
