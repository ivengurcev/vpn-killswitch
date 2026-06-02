package configedit

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-killswitch/internal/config"
)

func TestAddRejectsDomainAlreadyInOtherList(t *testing.T) {
	path := writeTestConfig(t, `
[vpn]
connection_name = "Work VPN"

[bypass]
domains = ["example.com"]
`)
	result, err := Add(path, Lock, "example.com")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if result.Changed {
		t.Fatal("Add() changed config for duplicate cross-list domain")
	}
}

func TestAddAndRemoveBypass(t *testing.T) {
	path := writeTestConfig(t, `
[vpn]
connection_name = "Work VPN"

[killswitch]
domains = ["lock.test"]
`)
	result, err := Add(path, Bypass, "OZON.RU.")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Add() did not change config")
	}
	cfg, err := config.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Bypass.Domains) != 1 || cfg.Bypass.Domains[0] != "ozon.ru" {
		t.Fatalf("bypass domains = %#v", cfg.Bypass.Domains)
	}
	result, err = Remove(path, Bypass, "ozon.ru")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("Remove() did not change config")
	}
}

func writeTestConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
