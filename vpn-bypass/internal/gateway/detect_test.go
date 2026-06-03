package gateway

import "testing"

func TestParseDefaultRoutesSkipsVPNInterfaces(t *testing.T) {
	input := `default via 10.110.193.1 dev tun0 metric 50
default via 192.168.3.1 dev wlp3s0 metric 600`

	got, err := ParseDefaultRoutes(input)
	if err != nil {
		t.Fatalf("ParseDefaultRoutes() error = %v", err)
	}
	if got.Via != "192.168.3.1" || got.Dev != "wlp3s0" {
		t.Fatalf("gateway = %+v", got)
	}
}

func TestParseDefaultRoutesRejectsOnlyVPNInterfaces(t *testing.T) {
	_, err := ParseDefaultRoutes(`default via 10.110.193.1 dev tun0 metric 50`)
	if err == nil {
		t.Fatal("expected error")
	}
}
