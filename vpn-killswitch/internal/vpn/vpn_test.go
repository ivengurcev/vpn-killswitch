package vpn

import "testing"

func TestParseActiveConnections(t *testing.T) {
	out := "Home WiFi:802-11-wireless\nWork\\: VPN:vpn\n"
	got := ParseActiveConnections(out, "Work: VPN", []string{"vpn", "wireguard"})
	if got != Active {
		t.Fatalf("status = %s, want %s", got, Active)
	}
}

func TestParseActiveConnectionsInactive(t *testing.T) {
	got := ParseActiveConnections("Home:802-11-wireless\nOther:vpn", "Work VPN", []string{"vpn"})
	if got != Inactive {
		t.Fatalf("status = %s, want %s", got, Inactive)
	}
}
