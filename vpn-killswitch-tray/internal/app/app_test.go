package app

import (
	"strings"
	"testing"

	"vpn-killswitch-tray/internal/domains"
)

func TestParseDomainEditorOutput(t *testing.T) {
	got := parseDomainEditorOutput("OZON.RU\nwildberries.ru\n93.184.216.99/24\tExample.com\napi.example.com\t", "\t")
	if strings.Join(got.Bypass, ",") != "ozon.ru,wildberries.ru" {
		t.Fatalf("bypass = %#v", got.Bypass)
	}
	if strings.Join(got.BypassIPs, ",") != "93.184.216.0/24" {
		t.Fatalf("bypass ips = %#v", got.BypassIPs)
	}
	if strings.Join(got.Lock, ",") != "example.com,api.example.com" {
		t.Fatalf("lock = %#v", got.Lock)
	}
}

func TestParseDomainEditorOutputRejectsLockIP(t *testing.T) {
	_, err := parseDomainEditorOutputStrict("example.com\t93.184.216.34\t", "\t")
	if err == nil {
		t.Fatal("parseDomainEditorOutputStrict() expected lock IP error")
	}
}

func TestDiagnosticTextIncludesStdoutOnFailure(t *testing.T) {
	text := diagnosticText("test", "FAIL", `{"ok":false}`, errFake{})
	if !strings.Contains(text, "test: FAIL") || !strings.Contains(text, `{"ok":false}`) || !strings.Contains(text, "Ошибка запуска") {
		t.Fatalf("diagnostic text = %q", text)
	}
}

type errFake struct{}

func (errFake) Error() string {
	return "exit status 10"
}

func TestDiffDomainLists(t *testing.T) {
	oldLists := domains.Lists{
		Bypass:    []string{"old-bypass.test", "stay-bypass.test"},
		BypassIPs: []string{"93.184.216.0/24"},
		Lock:      []string{"old-lock.test", "stay-lock.test"},
	}
	nextLists := domains.Lists{
		Bypass:    []string{"stay-bypass.test", "new-bypass.test"},
		BypassIPs: []string{"93.184.216.34"},
		Lock:      []string{"stay-lock.test", "new-lock.test"},
	}
	changes, err := diffDomainLists(oldLists, nextLists)
	if err != nil {
		t.Fatalf("diffDomainLists() error = %v", err)
	}
	if strings.Join(changes.RemoveBypass, ",") != "old-bypass.test" {
		t.Fatalf("RemoveBypass = %#v", changes.RemoveBypass)
	}
	if strings.Join(changes.AddBypass, ",") != "new-bypass.test" {
		t.Fatalf("AddBypass = %#v", changes.AddBypass)
	}
	if strings.Join(changes.RemoveIP, ",") != "93.184.216.0/24" {
		t.Fatalf("RemoveIP = %#v", changes.RemoveIP)
	}
	if strings.Join(changes.AddBypassIP, ",") != "93.184.216.34" {
		t.Fatalf("AddBypassIP = %#v", changes.AddBypassIP)
	}
	if strings.Join(changes.RemoveLock, ",") != "old-lock.test" {
		t.Fatalf("RemoveLock = %#v", changes.RemoveLock)
	}
	if strings.Join(changes.AddLock, ",") != "new-lock.test" {
		t.Fatalf("AddLock = %#v", changes.AddLock)
	}
}

func TestDiffDomainListsRejectsOverlap(t *testing.T) {
	_, err := diffDomainLists(domains.Lists{}, domains.Lists{
		Bypass: []string{"example.com"},
		Lock:   []string{"example.com"},
	})
	if err == nil {
		t.Fatal("diffDomainLists() expected overlap error")
	}
}
