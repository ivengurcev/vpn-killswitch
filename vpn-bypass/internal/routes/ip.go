package routes

import (
	"strings"

	"vpn-bypass/internal/gateway"
	"vpn-bypass/internal/run"
)

type RouteInfo struct {
	Raw string `json:"raw"`
	Via string `json:"via,omitempty"`
	Dev string `json:"dev,omitempty"`
}

func AddRoute(r run.Runner, route StateRoute) error {
	_, _, err := r.Run("ip", "route", "replace", route.IP+"/32", "via", route.Gateway, "dev", route.Dev)
	return err
}

func DeleteRoute(r run.Runner, route StateRoute) error {
	_, _, err := r.Run("ip", "route", "del", route.IP+"/32", "via", route.Gateway, "dev", route.Dev)
	return err
}

func GetRoute(r run.Runner, ip string) (RouteInfo, error) {
	out, _, err := r.Run("ip", "-4", "route", "get", ip)
	if err != nil {
		return RouteInfo{}, err
	}
	return ParseRouteGet(out), nil
}

func ParseRouteGet(out string) RouteInfo {
	line := strings.TrimSpace(strings.Split(out, "\n")[0])
	fields := strings.Fields(line)
	info := RouteInfo{Raw: line}
	for i := 0; i < len(fields)-1; i++ {
		switch fields[i] {
		case "via":
			info.Via = fields[i+1]
		case "dev":
			info.Dev = fields[i+1]
		}
	}
	return info
}

func IsBypassed(info RouteInfo, gw gateway.Gateway) bool {
	return info.Via == gw.Via && info.Dev == gw.Dev
}
