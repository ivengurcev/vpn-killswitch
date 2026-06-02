package bypass

import (
	"testing"

	"vpn-killswitch/internal/gateway"
	"vpn-killswitch/internal/log"
)

type fakeResolver map[string][]string

func (f fakeResolver) ResolveIPv4(domain string) ([]string, error) {
	return f[domain], nil
}

func TestBuildRoutesDeduplicatesPerDomainIP(t *testing.T) {
	gw := gateway.Gateway{Via: "192.168.3.1", Dev: "wlp3s0"}
	routes := BuildRoutes([]string{"a.test", "b.test"}, gw, fakeResolver{
		"a.test": {"1.1.1.1", "1.1.1.1"},
		"b.test": {"2.2.2.2"},
	}, log.Logger{Quiet: true})
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d, want 2: %+v", len(routes), routes)
	}
	if routes[0].Domain != "a.test" || routes[0].IP != "1.1.1.1" {
		t.Fatalf("routes[0] = %+v", routes[0])
	}
}
