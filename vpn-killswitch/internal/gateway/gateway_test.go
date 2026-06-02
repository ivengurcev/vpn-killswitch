package gateway

import "testing"

func TestParseDefaultRoutesSkipsVPNInterfaces(t *testing.T) {
	out := `default via 10.8.0.1 dev tun0
default via 192.168.3.1 dev wlp3s0 proto dhcp`
	got, err := ParseDefaultRoutes(out)
	if err != nil {
		t.Fatalf("ParseDefaultRoutes() error = %v", err)
	}
	if got.Via != "192.168.3.1" || got.Dev != "wlp3s0" {
		t.Fatalf("gateway = %+v", got)
	}
}

func TestParseDefaultRoutesNotFound(t *testing.T) {
	_, err := ParseDefaultRoutes("default via 10.8.0.1 dev wg0")
	if err == nil {
		t.Fatal("ParseDefaultRoutes() expected error")
	}
}
