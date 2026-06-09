package bypass

import (
	"fmt"

	"vpn-killswitch/internal/gateway"
	"vpn-killswitch/internal/ipaddr"
	"vpn-killswitch/internal/log"
	"vpn-killswitch/internal/run"
)

type IPv4Resolver interface {
	ResolveIPv4(domain string) ([]string, error)
}

type Result struct {
	OldRoutes []Route         `json:"old_routes"`
	NewRoutes []Route         `json:"new_routes"`
	Gateway   gateway.Gateway `json:"gateway"`
	Failures  int             `json:"failures"`
}

type Options struct {
	Domains   []string
	IPs       []string
	StatePath string
	DryRun    bool
}

func Apply(opts Options, runner run.Runner, resolver IPv4Resolver, logger log.Logger) (Result, error) {
	gw, err := gateway.Detect(runner)
	if err != nil {
		return Result{}, err
	}
	logger.Info("bypass gateway: %s dev %s", gw.Via, gw.Dev)

	oldRoutes, err := ReadState(opts.StatePath)
	if err != nil {
		return Result{}, err
	}
	result := Result{OldRoutes: oldRoutes, Gateway: gw}

	for _, old := range oldRoutes {
		if opts.DryRun {
			logger.Info("dry-run bypass route delete: %s via %s dev %s", routeTarget(old), old.Gateway, old.Dev)
			continue
		}
		if err := DeleteRoute(runner, old); err != nil {
			logger.Warn("bypass route delete failed: %s via %s dev %s: %v", routeTarget(old), old.Gateway, old.Dev, err)
		}
	}

	routes, domainRouteCount, err := BuildRoutes(opts.Domains, opts.IPs, gw, resolver, logger)
	if err != nil {
		return result, err
	}
	result.NewRoutes = routes
	if len(opts.Domains) > 0 && domainRouteCount == 0 {
		return result, fmt.Errorf("no bypass domains resolved to IPv4")
	}

	for _, route := range result.NewRoutes {
		if opts.DryRun {
			logger.Info("dry-run bypass route add: %s %s via %s dev %s", routeSource(route), routeTarget(route), route.Gateway, route.Dev)
			continue
		}
		if err := AddRoute(runner, route); err != nil {
			result.Failures++
			logger.Error("bypass route add failed: %s %s via %s dev %s: %v", routeSource(route), routeTarget(route), route.Gateway, route.Dev, err)
		}
	}

	if opts.DryRun {
		logger.Info("dry-run bypass state write: %s (%d routes)", opts.StatePath, len(result.NewRoutes))
		return result, nil
	}
	if err := WriteState(opts.StatePath, result.NewRoutes); err != nil {
		return result, err
	}
	if result.Failures > 0 {
		return result, fmt.Errorf("%d bypass route(s) failed", result.Failures)
	}
	return result, nil
}

func BuildRoutes(domains []string, ips []string, gw gateway.Gateway, resolver IPv4Resolver, logger log.Logger) ([]Route, int, error) {
	seen := map[string]bool{}
	var routes []Route
	domainRouteCount := 0
	for _, domain := range domains {
		ips, err := resolver.ResolveIPv4(domain)
		if err != nil {
			logger.Warn("bypass resolve failed: %s: %v", domain, err)
			continue
		}
		for _, ip := range ips {
			key := domain + "|" + ip
			if seen[key] {
				continue
			}
			seen[key] = true
			routes = append(routes, Route{Target: ip + "/32", IP: ip, Gateway: gw.Via, Dev: gw.Dev, Domain: domain, SourceType: "domain", Source: domain})
			domainRouteCount++
		}
	}
	entries, err := ipaddr.NormalizeBypassEntries(ips)
	if err != nil {
		return nil, 0, err
	}
	for _, raw := range entries {
		entry, err := ipaddr.ParseBypassEntry(raw)
		if err != nil {
			return nil, 0, err
		}
		for _, prefix := range entry.Prefixes {
			key := "ip|" + prefix
			if seen[key] {
				continue
			}
			seen[key] = true
			routes = append(routes, Route{Target: prefix, IP: targetIP(prefix), Gateway: gw.Via, Dev: gw.Dev, SourceType: "ip", Source: entry.Value})
		}
	}
	sortRoutes(routes)
	return routes, domainRouteCount, nil
}

func AddRoute(r run.Runner, route Route) error {
	_, _, err := r.Run("ip", "route", "replace", routeTarget(route), "via", route.Gateway, "dev", route.Dev)
	return err
}

func DeleteRoute(r run.Runner, route Route) error {
	_, _, err := r.Run("ip", "route", "del", routeTarget(route), "via", route.Gateway, "dev", route.Dev)
	return err
}
