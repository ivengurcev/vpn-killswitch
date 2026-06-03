package status

import "testing"

func TestParseStatus(t *testing.T) {
	got, err := Parse([]byte(`{
  "vpn": {"name": "Work VPN", "types": ["vpn"], "status": "active"},
  "gateway": {"via": "192.168.3.1", "dev": "wlp3s0"},
  "bypass": {"domain_count": 2, "route_count": 3, "state_path": "/run/state"},
  "lock": {"domain_count": 1, "nft_active": false, "hosts_block": false},
  "config": {"path": "/etc/vpn-killswitch/config.toml"},
  "errors": []
}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.VPN.Status != "active" || got.Bypass.RouteCount != 3 {
		t.Fatalf("status = %+v", got)
	}
}
