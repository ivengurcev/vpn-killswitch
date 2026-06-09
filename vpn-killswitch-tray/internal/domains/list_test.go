package domains

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[bypass]
domains = ["ozon.ru", "wildberries.ru"]
ips = ["93.184.216.0/24"]

[killswitch]
domains = ["example.com"]
`), 0644); err != nil {
		t.Fatal(err)
	}
	lists, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := strings.Join(lists.Bypass, ","); got != "ozon.ru,wildberries.ru" {
		t.Fatalf("bypass = %q", got)
	}
	if got := strings.Join(lists.BypassIPs, ","); got != "93.184.216.0/24" {
		t.Fatalf("bypass ips = %q", got)
	}
	if got := strings.Join(lists.Lock, ","); got != "example.com" {
		t.Fatalf("lock = %q", got)
	}
}

func TestFormatEmpty(t *testing.T) {
	if got := Format("Bypass", nil); got != "Bypass: список пуст" {
		t.Fatalf("Format() = %q", got)
	}
}

func TestParseInput(t *testing.T) {
	got := strings.Join(ParseInput("EXAMPLE.com, api.example.com\nexample.com; cdn.example.com."), ",")
	if got != "example.com,api.example.com,cdn.example.com" {
		t.Fatalf("ParseInput() = %q", got)
	}
}

func TestParseBypassInputSplitsDomainsAndIPs(t *testing.T) {
	domains, ips, err := ParseBypassInput("EXAMPLE.com\n93.184.216.99/24\n93.184.216.10-93.184.216.20\n93.184.216.34")
	if err != nil {
		t.Fatalf("ParseBypassInput() error = %v", err)
	}
	if got := strings.Join(domains, ","); got != "example.com" {
		t.Fatalf("domains = %q", got)
	}
	if got := strings.Join(ips, ","); got != "93.184.216.0/24,93.184.216.10-93.184.216.20,93.184.216.34" {
		t.Fatalf("ips = %q", got)
	}
}

func TestParseBypassInputRejectsBadIP(t *testing.T) {
	_, _, err := ParseBypassInput("2001:db8::1")
	if err == nil {
		t.Fatal("ParseBypassInput() expected IPv6 error")
	}
}

func TestParseDomainInputRejectsIP(t *testing.T) {
	_, err := ParseDomainInput("93.184.216.34")
	if err == nil {
		t.Fatal("ParseDomainInput() expected IP error")
	}
}
