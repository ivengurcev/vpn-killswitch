package routes

import (
	"fmt"

	"vpn-bypass/internal/gateway"
	"vpn-bypass/internal/log"
	"vpn-bypass/internal/run"
)

type IPv4Resolver interface {
	ResolveIPv4(domain string) ([]string, error)
}

type ApplyOptions struct {
	ConfigPath string
	StatePath  string
	DryRun     bool
}

type ApplyResult struct {
	OldRoutes []StateRoute `json:"old_routes"`
	NewRoutes []StateRoute `json:"new_routes"`
	Gateway   gateway.Gateway
	Failures  int `json:"failures"`
}

func Apply(opts ApplyOptions, domains []string, runner run.Runner, res IPv4Resolver, logger log.Logger) (ApplyResult, error) {
	gw, err := gateway.Detect(runner)
	if err != nil {
		return ApplyResult{}, err
	}
	logger.Info("normal gateway: %s dev %s", gw.Via, gw.Dev)

	oldRoutes, err := ReadState(opts.StatePath)
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{OldRoutes: oldRoutes, Gateway: gw}

	for _, old := range oldRoutes {
		if opts.DryRun {
			logger.Info("dry-run route delete: %s/32 via %s dev %s", old.IP, old.Gateway, old.Dev)
			continue
		}
		if err := DeleteRoute(runner, old); err != nil {
			logger.Warn("route delete failed: %s/32 via %s dev %s: %v", old.IP, old.Gateway, old.Dev, err)
			continue
		}
		logger.Verbosef("route deleted: %s/32 via %s dev %s", old.IP, old.Gateway, old.Dev)
	}

	seenRoutes := map[string]bool{}
	for _, domain := range domains {
		ips, err := res.ResolveIPv4(domain)
		if err != nil {
			logger.Warn("resolve failed: %s: %v", domain, err)
			continue
		}
		if len(ips) == 0 {
			logger.Warn("resolve returned no IPv4: %s", domain)
			continue
		}
		for _, ip := range ips {
			key := ip + "|" + domain
			if seenRoutes[key] {
				continue
			}
			seenRoutes[key] = true
			result.NewRoutes = append(result.NewRoutes, StateRoute{IP: ip, Gateway: gw.Via, Dev: gw.Dev, Domain: domain})
		}
	}

	if len(result.NewRoutes) == 0 {
		if len(domains) == 0 {
			if opts.DryRun {
				logger.Info("dry-run state write: %s (0 routes)", opts.StatePath)
				return result, nil
			}
			if err := WriteState(opts.StatePath, nil); err != nil {
				return result, err
			}
			logger.Info("no domains configured")
			return result, nil
		}
		return result, fmt.Errorf("no domains resolved to IPv4")
	}

	for _, route := range result.NewRoutes {
		if opts.DryRun {
			logger.Info("dry-run route add: %s %s via %s dev %s", route.Domain, route.IP, route.Gateway, route.Dev)
			continue
		}
		if err := AddRoute(runner, route); err != nil {
			result.Failures++
			logger.Error("route add failed: %s %s via %s dev %s: %v", route.Domain, route.IP, route.Gateway, route.Dev, err)
			continue
		}
		logger.Info("route add: %s %s via %s dev %s", route.Domain, route.IP, route.Gateway, route.Dev)
	}

	if opts.DryRun {
		logger.Info("dry-run state write: %s (%d routes)", opts.StatePath, len(result.NewRoutes))
		return result, nil
	}
	if err := WriteState(opts.StatePath, result.NewRoutes); err != nil {
		return result, err
	}
	if result.Failures > 0 {
		return result, fmt.Errorf("%d route(s) failed", result.Failures)
	}
	return result, nil
}
