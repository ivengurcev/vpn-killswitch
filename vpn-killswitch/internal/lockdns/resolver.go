package lockdns

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"vpn-killswitch/internal/ipaddr"
	"vpn-killswitch/internal/run"
)

const DefaultTimeout = 10 * time.Second

type Resolver struct {
	Runner        run.Runner
	ResolvConf    string
	Timeout       time.Duration
	ResolvectlDNS []string
}

type Result struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

func (r Resolver) Resolve(domain string) (Result, error) {
	servers, err := r.DNSServers()
	if err != nil {
		return Result{}, err
	}
	if len(servers) == 0 {
		return Result{}, fmt.Errorf("no DNS servers found")
	}
	var lastErr error
	for _, server := range servers {
		result, err := r.resolveWithServer(domain, server)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return Result{}, lastErr
}

func (r Resolver) DNSServers() ([]string, error) {
	path := r.ResolvConf
	if path == "" {
		path = "/etc/resolv.conf"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	servers := ParseResolvConf(string(data))
	if hasLocalStub(servers) {
		if len(r.ResolvectlDNS) > 0 {
			return normalizeServers(r.ResolvectlDNS), nil
		}
		if r.Runner == nil {
			return servers, nil
		}
		out, _, err := r.Runner.Run("resolvectl", "dns")
		if err != nil {
			return servers, nil
		}
		fromResolved := ParseResolvectlDNS(out)
		if len(fromResolved) > 0 {
			return fromResolved, nil
		}
	}
	return servers, nil
}

func (r Resolver) resolveWithServer(domain, server string) (Result, error) {
	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	dialer := net.Dialer{Timeout: timeout}
	resolver := net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ips, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return Result{}, err
	}
	seen4 := map[string]bool{}
	seen6 := map[string]bool{}
	var result Result
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if !ok || !ipaddr.IsUsablePublic(addr.String()) {
			continue
		}
		if addr.Is4() {
			if !seen4[addr.String()] {
				seen4[addr.String()] = true
				result.IPv4 = append(result.IPv4, addr.String())
			}
			continue
		}
		if addr.Is6() && !seen6[addr.String()] {
			seen6[addr.String()] = true
			result.IPv6 = append(result.IPv6, addr.String())
		}
	}
	return result, nil
}

func ParseResolvConf(data string) []string {
	var servers []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			if addr, err := netip.ParseAddr(fields[1]); err == nil {
				servers = append(servers, addr.String())
			}
		}
	}
	return unique(servers)
}

func ParseResolvectlDNS(out string) []string {
	var servers []string
	for _, field := range strings.Fields(out) {
		field = strings.TrimSuffix(field, ":")
		if addr, err := netip.ParseAddr(field); err == nil {
			servers = append(servers, addr.String())
		}
	}
	return unique(servers)
}

func hasLocalStub(servers []string) bool {
	for _, server := range servers {
		addr, err := netip.ParseAddr(server)
		if err == nil && addr.IsLoopback() {
			return true
		}
	}
	return false
}

func normalizeServers(servers []string) []string {
	var out []string
	for _, server := range servers {
		addr, err := netip.ParseAddr(strings.TrimSpace(server))
		if err == nil {
			out = append(out, addr.String())
		}
	}
	return unique(out)
}

func unique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
