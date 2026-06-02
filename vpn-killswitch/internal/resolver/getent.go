package resolver

import (
	"net/netip"
	"strings"

	"vpn-killswitch/internal/run"
)

type Getent struct {
	Runner run.Runner
}

func (r Getent) ResolveIPv4(domain string) ([]string, error) {
	out, _, err := r.Runner.Run("getent", "ahostsv4", domain)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ips []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		addr, err := netip.ParseAddr(fields[0])
		if err != nil || !addr.Is4() {
			continue
		}
		ip := addr.String()
		if !seen[ip] {
			seen[ip] = true
			ips = append(ips, ip)
		}
	}
	return ips, nil
}
