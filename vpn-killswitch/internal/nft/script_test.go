package nft

import (
	"strings"
	"testing"
)

func TestBuildScript(t *testing.T) {
	script := BuildScript(Spec{
		TableName: "vpn_killswitch_lock",
		IPv4:      []string{"2.2.2.2", "1.1.1.1"},
		IPv6:      []string{"2001:db8::1"},
	})
	for _, want := range []string{
		"table inet vpn_killswitch_lock",
		"set blocked_ipv4",
		"elements = { 1.1.1.1, 2.2.2.2 }",
		"chain vpn_killswitch_output",
		"ip daddr @blocked_ipv4 reject",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}
