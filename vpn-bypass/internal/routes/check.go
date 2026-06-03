package routes

import (
	"fmt"

	"vpn-bypass/internal/gateway"
	"vpn-bypass/internal/resolver"
	"vpn-bypass/internal/run"
)

type CheckIP struct {
	IP     string    `json:"ip"`
	Route  RouteInfo `json:"route"`
	Status string    `json:"status"`
	Error  string    `json:"error,omitempty"`
}

type CheckResult struct {
	Domain  string    `json:"domain"`
	IPs     []CheckIP `json:"ips"`
	Summary string    `json:"summary"`
}

func Check(domain string, runner run.Runner, res resolver.Resolver) CheckResult {
	result := CheckResult{Domain: domain}
	gw, gwErr := gateway.Detect(runner)
	ips, err := res.ResolveIPv4(domain)
	if err != nil || len(ips) == 0 {
		result.Summary = "unresolved"
		return result
	}
	if gwErr != nil {
		result.Summary = "error"
		for _, ip := range ips {
			result.IPs = append(result.IPs, CheckIP{IP: ip, Status: "error", Error: gwErr.Error()})
		}
		return result
	}

	counts := map[string]int{}
	for _, ip := range ips {
		route, err := GetRoute(runner, ip)
		item := CheckIP{IP: ip, Route: route}
		if err != nil {
			item.Status = "error"
			item.Error = err.Error()
		} else if IsBypassed(route, gw) {
			item.Status = "bypassed"
		} else {
			item.Status = "vpn"
		}
		counts[item.Status]++
		result.IPs = append(result.IPs, item)
	}
	result.Summary = summarize(counts, len(result.IPs))
	return result
}

func summarize(counts map[string]int, total int) string {
	if total == 0 {
		return "unresolved"
	}
	if counts["error"] > 0 {
		return "error"
	}
	if counts["bypassed"] == total {
		return "bypassed"
	}
	if counts["vpn"] == total {
		return "vpn"
	}
	return "partial"
}

func FormatRoute(info RouteInfo) string {
	if info.Via != "" && info.Dev != "" {
		return fmt.Sprintf("via %s dev %s", info.Via, info.Dev)
	}
	if info.Dev != "" {
		return fmt.Sprintf("dev %s", info.Dev)
	}
	if info.Raw != "" {
		return info.Raw
	}
	return "unknown"
}
