package diagnostics

import (
	"os"
	"os/exec"

	"vpn-killswitch/internal/bypass"
	"vpn-killswitch/internal/config"
	"vpn-killswitch/internal/gateway"
	"vpn-killswitch/internal/hosts"
	"vpn-killswitch/internal/run"
	"vpn-killswitch/internal/system"
	"vpn-killswitch/internal/vpn"
)

type Status struct {
	VPN     VPNStatus     `json:"vpn"`
	Gateway GatewayStatus `json:"gateway"`
	Bypass  BypassStatus  `json:"bypass"`
	Lock    LockStatus    `json:"lock"`
	Config  ConfigStatus  `json:"config"`
	Errors  []string      `json:"errors"`
}

type VPNStatus struct {
	Name   string     `json:"name"`
	Types  []string   `json:"types"`
	Status vpn.Status `json:"status"`
}

type GatewayStatus struct {
	Via   string `json:"via,omitempty"`
	Dev   string `json:"dev,omitempty"`
	Error string `json:"error,omitempty"`
}

type BypassStatus struct {
	DomainCount int    `json:"domain_count"`
	RouteCount  int    `json:"route_count"`
	StatePath   string `json:"state_path"`
}

type LockStatus struct {
	DomainCount int    `json:"domain_count"`
	NFTActive   bool   `json:"nft_active"`
	HostsBlock  bool   `json:"hosts_block"`
	Error       string `json:"error,omitempty"`
}

type ConfigStatus struct {
	Path string `json:"path"`
}

func CollectStatus(cfg config.Config, configPath string, runner run.Runner) Status {
	var status Status
	status.VPN = VPNStatus{
		Name:   cfg.VPN.ConnectionName,
		Types:  cfg.VPN.ConnectionTypes,
		Status: vpn.Detect(runner, cfg.VPN.ConnectionName, cfg.VPN.ConnectionTypes),
	}
	status.Config.Path = configPath

	gw, err := gateway.Detect(runner)
	if err != nil {
		status.Gateway.Error = err.Error()
		status.Errors = append(status.Errors, "gateway: "+err.Error())
	} else {
		status.Gateway.Via = gw.Via
		status.Gateway.Dev = gw.Dev
	}

	routes, err := bypass.ReadState(cfg.Bypass.StatePath)
	if err != nil {
		status.Errors = append(status.Errors, "bypass state: "+err.Error())
	}
	status.Bypass = BypassStatus{
		DomainCount: len(cfg.Bypass.Domains),
		RouteCount:  len(routes),
		StatePath:   cfg.Bypass.StatePath,
	}

	status.Lock.DomainCount = len(cfg.Killswitch.Domains)
	if _, _, err := runner.Run("nft", "list", "table", "inet", cfg.Killswitch.NFTTableName); err == nil {
		status.Lock.NFTActive = true
	}
	block, err := hosts.Manager{Path: cfg.Killswitch.HostsFile}.HasBlock()
	if err != nil {
		status.Lock.Error = err.Error()
		status.Errors = append(status.Errors, "hosts: "+err.Error())
	} else {
		status.Lock.HostsBlock = block
	}
	return status
}

type TestResult struct {
	OK     bool        `json:"ok"`
	Checks []TestCheck `json:"checks"`
	Errors []string    `json:"errors"`
}

type TestCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func RunTests(cfg config.Config, configPath string, runner run.Runner) TestResult {
	var result TestResult
	result.OK = true
	add := func(name string, ok bool, message string) {
		status := "ok"
		if !ok {
			status = "fail"
			result.OK = false
			result.Errors = append(result.Errors, name+": "+message)
		}
		result.Checks = append(result.Checks, TestCheck{Name: name, Status: status, Message: message})
	}

	for _, cmd := range []string{"ip", "nmcli", "nft", "getent", "systemctl"} {
		_, err := exec.LookPath(cmd)
		add("dependency "+cmd, err == nil, errString(err))
	}
	add("config "+configPath, cfg.Validate() == nil, errString(cfg.Validate()))

	status := CollectStatus(cfg, configPath, runner)
	if status.VPN.Status == vpn.Active {
		add("vpn active lock off", !status.Lock.NFTActive && !status.Lock.HostsBlock, "lock is active while VPN is active")
	} else {
		add("vpn inactive lock on", status.Lock.NFTActive || status.Lock.HostsBlock, "lock is not active while VPN is inactive/unknown")
	}
	return result
}

func RunInstallCheck(cfg config.Config, configPath string, runner run.Runner) TestResult {
	result := RunTests(cfg, configPath, runner)
	add := func(name string, ok bool, message string) {
		status := "ok"
		if !ok {
			status = "fail"
			result.OK = false
			result.Errors = append(result.Errors, name+": "+message)
		}
		result.Checks = append(result.Checks, TestCheck{Name: name, Status: status, Message: message})
	}
	for _, path := range []string{
		system.BinPath,
		configPath,
		system.ServicePath,
		system.RefreshSvcPath,
		system.TimerPath,
		system.SleepHookPath,
		system.DispatcherPath,
		system.TmpfilesPath,
		system.LogrotatePath,
	} {
		_, err := os.Stat(path)
		add("file "+path, err == nil, errString(err))
	}
	if _, _, err := runner.Run("systemctl", "is-enabled", "vpn-killswitch.service"); err != nil {
		add("systemd service enabled", false, err.Error())
	} else {
		add("systemd service enabled", true, "")
	}
	if _, _, err := runner.Run("systemctl", "is-enabled", "vpn-killswitch-refresh.timer"); err != nil {
		add("systemd timer enabled", false, err.Error())
	} else {
		add("systemd timer enabled", true, "")
	}
	if _, _, err := runner.Run("systemctl", "is-active", "vpn-killswitch-refresh.timer"); err != nil {
		add("systemd timer active", false, err.Error())
	} else {
		add("systemd timer active", true, "")
	}
	return result
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
