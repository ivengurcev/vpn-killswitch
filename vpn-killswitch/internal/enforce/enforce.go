package enforce

import (
	"fmt"

	"vpn-killswitch/internal/bypass"
	"vpn-killswitch/internal/config"
	"vpn-killswitch/internal/lock"
	"vpn-killswitch/internal/log"
	"vpn-killswitch/internal/run"
	"vpn-killswitch/internal/vpn"
)

type BypassResolver interface {
	ResolveIPv4(domain string) ([]string, error)
}

type LockResolver interface {
	lock.DNSResolver
}

type Result struct {
	VPN          vpn.Status    `json:"vpn_status"`
	Bypass       bypass.Result `json:"bypass"`
	Lock         lock.Result   `json:"lock"`
	LockApplied  bool          `json:"lock_applied"`
	LockDisabled bool          `json:"lock_disabled"`
	Errors       []string      `json:"errors,omitempty"`
}

type Options struct {
	Config config.Config
	DryRun bool
	Reason string
}

func Run(opts Options, runner run.Runner, bypassResolver BypassResolver, lockResolver LockResolver, logger log.Logger) (Result, error) {
	var result Result

	bypassResult, err := bypass.Apply(bypass.Options{
		Domains:   opts.Config.Bypass.Domains,
		IPs:       opts.Config.Bypass.IPs,
		StatePath: opts.Config.Bypass.StatePath,
		DryRun:    opts.DryRun,
	}, runner, bypassResolver, logger)
	result.Bypass = bypassResult
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("bypass apply: %v", err))
		lockResult, lockErr := enableLock(opts, runner, lockResolver, logger)
		result.Lock = lockResult
		result.LockApplied = lockErr == nil
		if lockErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("fail-closed lock enable: %v", lockErr))
			return result, fmt.Errorf("bypass apply failed: %w; fail-closed lock enable failed: %v", err, lockErr)
		}
		return result, err
	}

	status := vpn.Detect(runner, opts.Config.VPN.ConnectionName, opts.Config.VPN.ConnectionTypes)
	result.VPN = status
	logger.Info("vpn %q: %s", opts.Config.VPN.ConnectionName, status)
	if status == vpn.Active {
		if err := disableLock(opts, runner, logger); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("lock disable: %v", err))
			return result, err
		}
		result.LockDisabled = true
		return result, nil
	}

	lockResult, err := enableLock(opts, runner, lockResolver, logger)
	result.Lock = lockResult
	result.LockApplied = err == nil
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("lock enable: %v", err))
		return result, err
	}
	return result, nil
}

func RefreshLock(opts Options, runner run.Runner, lockResolver LockResolver, logger log.Logger) (Result, error) {
	var result Result
	status := vpn.Detect(runner, opts.Config.VPN.ConnectionName, opts.Config.VPN.ConnectionTypes)
	result.VPN = status
	if status == vpn.Active {
		logger.Info("vpn %q: active; lock refresh is no-op", opts.Config.VPN.ConnectionName)
		return result, nil
	}
	lockResult, err := enableLock(opts, runner, lockResolver, logger)
	result.Lock = lockResult
	result.LockApplied = err == nil
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("lock refresh: %v", err))
		return result, err
	}
	return result, nil
}

func enableLock(opts Options, runner run.Runner, lockResolver LockResolver, logger log.Logger) (lock.Result, error) {
	return lock.Enable(lock.Options{
		Domains:      opts.Config.Killswitch.Domains,
		NFTTableName: opts.Config.Killswitch.NFTTableName,
		HostsFile:    opts.Config.Killswitch.HostsFile,
		ManageHosts:  opts.Config.Killswitch.ManageHosts,
		DryRun:       opts.DryRun,
	}, runner, lockResolver, logger)
}

func disableLock(opts Options, runner run.Runner, logger log.Logger) error {
	return lock.Disable(lock.Options{
		NFTTableName: opts.Config.Killswitch.NFTTableName,
		HostsFile:    opts.Config.Killswitch.HostsFile,
		DryRun:       opts.DryRun,
	}, runner, logger)
}
