package diagnostics

import (
	"vpn-killswitch/internal/config"
	"vpn-killswitch/internal/lockdns"
)

type BypassResolver interface {
	ResolveIPv4(domain string) ([]string, error)
}

type LockResolver interface {
	Resolve(domain string) (lockdns.Result, error)
}

type ResolveResult struct {
	Bypass []ResolveItem `json:"bypass"`
	Lock   []ResolveItem `json:"lock"`
}

type ResolveItem struct {
	Domain string   `json:"domain"`
	IPv4   []string `json:"ipv4,omitempty"`
	IPv6   []string `json:"ipv6,omitempty"`
	Error  string   `json:"error,omitempty"`
}

func Resolve(cfg config.Config, bypassResolver BypassResolver, lockResolver LockResolver) ResolveResult {
	var result ResolveResult
	for _, domain := range cfg.Bypass.Domains {
		ips, err := bypassResolver.ResolveIPv4(domain)
		item := ResolveItem{Domain: domain, IPv4: ips}
		if err != nil {
			item.Error = err.Error()
		}
		result.Bypass = append(result.Bypass, item)
	}
	for _, domain := range cfg.Killswitch.Domains {
		resolved, err := lockResolver.Resolve(domain)
		item := ResolveItem{Domain: domain, IPv4: resolved.IPv4, IPv6: resolved.IPv6}
		if err != nil {
			item.Error = err.Error()
		}
		result.Lock = append(result.Lock, item)
	}
	return result
}
