package state

import (
	"testing"

	"vpn-killswitch-tray/internal/status"
)

func TestNormalizeOK(t *testing.T) {
	got := Normalize(status.Status{
		VPN:    status.VPNStatus{Status: "active"},
		Bypass: status.BypassStatus{RouteCount: 3},
	})
	if got.Kind != OK {
		t.Fatalf("Kind = %s, want %s: %+v", got.Kind, OK, got)
	}
}

func TestNormalizeLocked(t *testing.T) {
	got := Normalize(status.Status{
		VPN:  status.VPNStatus{Status: "inactive"},
		Lock: status.LockStatus{NFTActive: true},
	})
	if got.Kind != Locked {
		t.Fatalf("Kind = %s, want %s: %+v", got.Kind, Locked, got)
	}
}

func TestNormalizeWarningWhenLockMissing(t *testing.T) {
	got := Normalize(status.Status{VPN: status.VPNStatus{Status: "unknown"}})
	if got.Kind != Warning {
		t.Fatalf("Kind = %s, want %s: %+v", got.Kind, Warning, got)
	}
}

func TestNormalizeError(t *testing.T) {
	got := Normalize(status.Status{Errors: []string{"config broken"}})
	if got.Kind != Error {
		t.Fatalf("Kind = %s, want %s: %+v", got.Kind, Error, got)
	}
}
