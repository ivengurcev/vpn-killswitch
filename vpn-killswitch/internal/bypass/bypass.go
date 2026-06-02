package bypass

import (
	"fmt"

	"vpn-killswitch/internal/gateway"
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
			logger.Info("dry-run bypass route delete: %s/32 via %s dev %s", old.IP, old.Gateway, old.Dev)
			continue
		}
		if err := DeleteRoute(runner, old); err != nil {
			logger.Warn("bypass route delete failed: %s/32 via %s dev %s: %v", old.IP, old.Gateway, old.Dev, err)
		}
	}

	result.NewRoutes = BuildRoutes(opts.Domains, gw, resolver, logger)
	if len(opts.Domains) > 0 && len(result.NewRoutes) == 0 {
		return result, fmt.Errorf("no bypass domains resolved to IPv4")
	}

	for _, route := range result.NewRoutes {
		if opts.DryRun {
			logger.Info("dry-run bypass route add: %s %s/32 via %s dev %s", route.Domain, route.IP, route.Gateway, route.Dev)
			continue
		}
		if err := AddRoute(runner, route); err != nil {
			result.Failures++
			logger.Error("bypass route add failed: %s %s/32 via %s dev %s: %v", route.Domain, route.IP, route.Gateway, route.Dev, err)
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

func BuildRoutes(domains []string, gw gateway.Gateway, resolver IPv4Resolver, logger log.Logger) []Route {
	seen := map[string]bool{}
	var routes []Route
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
			routes = append(routes, Route{IP: ip, Gateway: gw.Via, Dev: gw.Dev, Domain: domain})
		}
	}
	sortRoutes(routes)
	return routes
}

func AddRoute(r run.Runner, route Route) error {
	_, _, err := r.Run("ip", "route", "replace", route.IP+"/32", "via", route.Gateway, "dev", route.Dev)
	return err
}

func DeleteRoute(r run.Runner, route Route) error {
	_, _, err := r.Run("ip", "route", "del", route.IP+"/32", "via", route.Gateway, "dev", route.Dev)
	return err
}
