package routes

import (
	"os"

	"vpn-bypass/internal/gateway"
	"vpn-bypass/internal/run"
)

type Status struct {
	ConfigPath      string           `json:"config_path"`
	StatePath       string           `json:"state_path"`
	DomainsInConfig int              `json:"domains_in_config"`
	RoutesInState   int              `json:"routes_in_state"`
	ActiveRoutes    int              `json:"active_bypass_routes"`
	Gateway         *gateway.Gateway `json:"normal_gateway,omitempty"`
	Service         string           `json:"systemd_service"`
	Timer           string           `json:"systemd_timer"`
	Hook            string           `json:"networkmanager_hook"`
}

func BuildStatus(configPath, statePath string, domains []string, runner run.Runner) Status {
	state, _ := ReadState(statePath)
	status := Status{
		ConfigPath:      configPath,
		StatePath:       statePath,
		DomainsInConfig: len(domains),
		RoutesInState:   len(state),
		Service:         installed("/etc/systemd/system/vpn-bypass-routes.service"),
		Timer:           timerStatus(runner),
		Hook:            installed("/etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes"),
	}
	if gw, err := gateway.Detect(runner); err == nil {
		status.Gateway = &gw
	}
	for _, route := range state {
		if info, err := GetRoute(runner, route.IP); err == nil && info.Via == route.Gateway && info.Dev == route.Dev {
			status.ActiveRoutes++
		}
	}
	return status
}

func installed(path string) string {
	if _, err := os.Stat(path); err == nil {
		return "installed"
	}
	return "missing"
}

func timerStatus(runner run.Runner) string {
	if _, err := os.Stat("/etc/systemd/system/vpn-bypass-routes.timer"); err != nil {
		return "missing"
	}
	out, _, err := runner.Run("systemctl", "is-active", "vpn-bypass-routes.timer")
	if err != nil {
		return "installed"
	}
	if out == "" {
		return "installed"
	}
	return out
}
