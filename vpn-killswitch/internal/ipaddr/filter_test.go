package ipaddr

import (
	"strings"
	"testing"
)

func TestNormalizeBypassEntries(t *testing.T) {
	entries, err := NormalizeBypassEntries([]string{
		"93.184.216.34",
		"93.184.216.99/24",
		"93.184.216.10-93.184.216.20",
		"93.184.216.34",
	})
	if err != nil {
		t.Fatalf("NormalizeBypassEntries() error = %v", err)
	}
	if got, want := strings.Join(entries, ","), "93.184.216.0/24,93.184.216.10-93.184.216.20,93.184.216.34"; got != want {
		t.Fatalf("entries = %q, want %q", got, want)
	}
}

func TestExpandBypassRange(t *testing.T) {
	prefixes, err := ExpandBypassEntries([]string{"93.184.216.10-93.184.216.20"})
	if err != nil {
		t.Fatalf("ExpandBypassEntries() error = %v", err)
	}
	if got, want := strings.Join(prefixes, ","), "93.184.216.10/31,93.184.216.12/30,93.184.216.16/30,93.184.216.20/32"; got != want {
		t.Fatalf("prefixes = %q, want %q", got, want)
	}
}

func TestParseBypassEntryRejectsIPv6(t *testing.T) {
	_, err := ParseBypassEntry("2001:db8::1")
	if err == nil {
		t.Fatal("ParseBypassEntry() expected IPv6 error")
	}
}

func TestParseBypassEntryRejectsReversedRange(t *testing.T) {
	_, err := ParseBypassEntry("93.184.216.20-93.184.216.10")
	if err == nil {
		t.Fatal("ParseBypassEntry() expected reversed range error")
	}
}
