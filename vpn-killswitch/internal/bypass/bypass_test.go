package bypass

import (
	"strings"
	"testing"

	"vpn-killswitch/internal/gateway"
	"vpn-killswitch/internal/log"
)

type fakeResolver map[string][]string

func (f fakeResolver) ResolveIPv4(domain string) ([]string, error) {
	return f[domain], nil
}

func join(values []string) string {
	return strings.Join(values, ",")
}

func TestBuildRoutesDeduplicatesPerDomainIP(t *testing.T) {
	gw := gateway.Gateway{Via: "192.168.3.1", Dev: "wlp3s0"}
	routes, domainRouteCount, err := BuildRoutes([]string{"a.test", "b.test"}, nil, gw, fakeResolver{
		"a.test": {"1.1.1.1", "1.1.1.1"},
		"b.test": {"2.2.2.2"},
	}, log.Logger{Quiet: true})
	if err != nil {
		t.Fatalf("BuildRoutes() error = %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d, want 2: %+v", len(routes), routes)
	}
	if domainRouteCount != 2 {
		t.Fatalf("domainRouteCount = %d, want 2", domainRouteCount)
	}
	if routes[0].Domain != "a.test" || routes[0].IP != "1.1.1.1" {
		t.Fatalf("routes[0] = %+v", routes[0])
	}
}

func TestBuildRoutesAddsExplicitIPs(t *testing.T) {
	gw := gateway.Gateway{Via: "192.168.3.1", Dev: "wlp3s0"}
	routes, domainRouteCount, err := BuildRoutes(nil, []string{
		"93.184.216.34",
		"93.184.216.0/30",
		"93.184.216.10-93.184.216.11",
	}, gw, fakeResolver{}, log.Logger{Quiet: true})
	if err != nil {
		t.Fatalf("BuildRoutes() error = %v", err)
	}
	if domainRouteCount != 0 {
		t.Fatalf("domainRouteCount = %d, want 0", domainRouteCount)
	}
	got := make([]string, 0, len(routes))
	for _, route := range routes {
		got = append(got, route.Target)
		if route.SourceType != "ip" {
			t.Fatalf("route SourceType = %q, want ip: %+v", route.SourceType, route)
		}
	}
	if join(got) != "93.184.216.0/30,93.184.216.10/31,93.184.216.34/32" {
		t.Fatalf("targets = %q", join(got))
	}
	if routes[1].Source != "93.184.216.10-93.184.216.11" {
		t.Fatalf("range source = %q", routes[1].Source)
	}
}
