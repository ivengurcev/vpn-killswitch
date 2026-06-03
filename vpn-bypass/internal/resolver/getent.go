package resolver

import (
	"net"
	"strings"

	"vpn-bypass/internal/run"
)

type Resolver struct {
	Runner run.Runner
}

func (r Resolver) ResolveIPv4(domain string) ([]string, error) {
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
		ip := net.ParseIP(fields[0])
		if ip == nil || ip.To4() == nil {
			continue
		}
		v4 := ip.To4().String()
		if !seen[v4] {
			seen[v4] = true
			ips = append(ips, v4)
		}
	}
	return ips, nil
}
