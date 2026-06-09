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

func TestApplyTrayReplacesLists(t *testing.T) {
	path := writeTestConfig(t, `
[vpn]
connection_name = "Work VPN"

[bypass]
domains = ["old.example"]

[killswitch]
domains = ["old-lock.example"]
`)
	result, err := ApplyTray(path, ApplyTrayPayload{
		BypassDomains:    []string{"OZON.RU.", "wildberries.ru", "ozon.ru"},
		BypassIPs:        []string{"93.184.216.99/24", "93.184.216.34"},
		LockDomains:      []string{"LOCK.TEST."},
		hasBypassDomains: true,
		hasBypassIPs:     true,
		hasLockDomains:   true,
	}, false)
	if err != nil {
		t.Fatalf("ApplyTray() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("ApplyTray() did not report change")
	}
	cfg, err := config.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := join(cfg.Bypass.Domains), "ozon.ru,wildberries.ru"; got != want {
		t.Fatalf("bypass domains = %q, want %q", got, want)
	}
	if got, want := join(cfg.Bypass.IPs), "93.184.216.0/24,93.184.216.34"; got != want {
		t.Fatalf("bypass ips = %q, want %q", got, want)
	}
	if got, want := join(cfg.Killswitch.Domains), "lock.test"; got != want {
		t.Fatalf("lock domains = %q, want %q", got, want)
	}
}

func TestApplyTrayDryRunDoesNotWrite(t *testing.T) {
	path := writeTestConfig(t, `
[vpn]
connection_name = "Work VPN"

[bypass]
domains = ["old.example"]
`)
	result, err := ApplyTray(path, ApplyTrayPayload{
		BypassDomains:    []string{"new.example"},
		hasBypassDomains: true,
		hasBypassIPs:     true,
		hasLockDomains:   true,
	}, true)
	if err != nil {
		t.Fatalf("ApplyTray() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("ApplyTray() did not report planned change")
	}
	cfg, err := config.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := join(cfg.Bypass.Domains), "old.example"; got != want {
		t.Fatalf("bypass domains after dry-run = %q, want %q", got, want)
	}
}

func TestApplyTrayRejectsBadBypassIPs(t *testing.T) {
	path := writeTestConfig(t, `
[vpn]
connection_name = "Work VPN"

[bypass]
domains = ["old.example"]
`)
	_, err := ApplyTray(path, ApplyTrayPayload{
		BypassDomains:    []string{"old.example"},
		BypassIPs:        []string{"2001:db8::1"},
		hasBypassDomains: true,
		hasBypassIPs:     true,
		hasLockDomains:   true,
	}, false)
	if err == nil {
		t.Fatal("ApplyTray() expected bypass IP error")
	}
}

func TestApplyTrayRejectsIncompletePayload(t *testing.T) {
	path := writeTestConfig(t, `
[vpn]
connection_name = "Work VPN"

[bypass]
domains = ["old.example"]
`)
	_, err := ApplyTray(path, ApplyTrayPayload{
		BypassDomains:    []string{"new.example"},
		hasBypassDomains: true,
	}, false)
	if err == nil {
		t.Fatal("ApplyTray() expected incomplete payload error")
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

func join(values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ","
		}
		out += value
	}
	return out
}
