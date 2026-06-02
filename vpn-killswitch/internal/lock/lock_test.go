package lock

import (
	"testing"

	"vpn-killswitch/internal/lockdns"
	"vpn-killswitch/internal/log"
)

type fakeDNS map[string]lockdns.Result

func (f fakeDNS) Resolve(domain string) (lockdns.Result, error) {
	return f[domain], nil
}

func TestResolveDomainsDeduplicates(t *testing.T) {
	got := ResolveDomains([]string{"a.test", "b.test"}, fakeDNS{
		"a.test": {IPv4: []string{"1.1.1.1"}, IPv6: []string{"2001:db8::1"}},
		"b.test": {IPv4: []string{"1.1.1.1", "2.2.2.2"}, IPv6: []string{"2001:db8::1"}},
	}, log.Logger{Quiet: true})
	if len(got.IPv4) != 2 {
		t.Fatalf("IPv4 = %#v", got.IPv4)
	}
	if len(got.IPv6) != 1 {
		t.Fatalf("IPv6 = %#v", got.IPv6)
	}
}
