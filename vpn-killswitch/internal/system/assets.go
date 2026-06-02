package system

import (
	"fmt"
	"strings"
)

const (
	BinPath        = "/usr/local/sbin/vpn-killswitch"
	ConfigPath     = "/etc/vpn-killswitch/config.toml"
	ServicePath    = "/etc/systemd/system/vpn-killswitch.service"
	RefreshSvcPath = "/etc/systemd/system/vpn-killswitch-refresh.service"
	TimerPath      = "/etc/systemd/system/vpn-killswitch-refresh.timer"
	SleepHookPath  = "/etc/systemd/system-sleep/vpn-killswitch"
	DispatcherPath = "/etc/NetworkManager/dispatcher.d/90-vpn-killswitch"
	TmpfilesPath   = "/usr/lib/tmpfiles.d/vpn-killswitch.conf"
	LogrotatePath  = "/etc/logrotate.d/vpn-killswitch"
)

type AssetsOptions struct {
	BinPath         string
	ConfigPath      string
	RunDir          string
	LogFile         string
	RefreshInterval string
	DebounceSeconds int
}

type FileAsset struct {
	Path string
	Mode int
	Data string
}

func DefaultAssetsOptions() AssetsOptions {
	return AssetsOptions{
		BinPath:         BinPath,
		ConfigPath:      ConfigPath,
		RunDir:          "/run/vpn-killswitch",
		LogFile:         "/var/log/vpn-killswitch.log",
		RefreshInterval: "5min",
		DebounceSeconds: 2,
	}
}

func BuildAssets(opts AssetsOptions) []FileAsset {
	if opts.BinPath == "" {
		opts.BinPath = BinPath
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = ConfigPath
	}
	if opts.RunDir == "" {
		opts.RunDir = "/run/vpn-killswitch"
	}
	if opts.LogFile == "" {
		opts.LogFile = "/var/log/vpn-killswitch.log"
	}
	if opts.RefreshInterval == "" {
		opts.RefreshInterval = "5min"
	}
	if opts.DebounceSeconds <= 0 {
		opts.DebounceSeconds = 2
	}
	return []FileAsset{
		{Path: ServicePath, Mode: 0644, Data: service(opts)},
		{Path: RefreshSvcPath, Mode: 0644, Data: refreshService(opts)},
		{Path: TimerPath, Mode: 0644, Data: refreshTimer(opts)},
		{Path: SleepHookPath, Mode: 0755, Data: sleepHook(opts)},
		{Path: DispatcherPath, Mode: 0755, Data: dispatcher(opts)},
		{Path: TmpfilesPath, Mode: 0644, Data: tmpfiles(opts)},
		{Path: LogrotatePath, Mode: 0644, Data: logrotate(opts)},
	}
}

func service(opts AssetsOptions) string {
	return fmt.Sprintf(`[Unit]
Description=VPN Killswitch reconciler
Documentation=man:nft(8) man:nmcli(1)
Wants=NetworkManager.service
After=NetworkManager.service nftables.service network-online.target

[Service]
Type=oneshot
ExecStart=%s enforce systemd --config %s

[Install]
WantedBy=multi-user.target
`, opts.BinPath, opts.ConfigPath)
}

func refreshService(opts AssetsOptions) string {
	return fmt.Sprintf(`[Unit]
Description=VPN Killswitch periodic lock refresh
Documentation=man:nft(8) man:nmcli(1)
After=NetworkManager.service nftables.service network-online.target

[Service]
Type=oneshot
ExecStart=%s lock refresh timer --config %s
`, opts.BinPath, opts.ConfigPath)
}

func refreshTimer(opts AssetsOptions) string {
	return fmt.Sprintf(`[Unit]
Description=VPN Killswitch periodic lock refresh timer

[Timer]
OnBootSec=30
OnUnitActiveSec=%s
Unit=vpn-killswitch-refresh.service

[Install]
WantedBy=timers.target
`, opts.RefreshInterval)
}

func dispatcher(opts AssetsOptions) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -u

PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

interface="${1:-unknown}"
action="${2:-unknown}"
run_dir=%q
debounce_seconds=%d

run_enforce() {
  %s enforce "networkmanager:${action}:${interface}" --config %s &
}

run_with_debounce() {
  local marker="${run_dir%%/}/dispatcher.marker"
  local now mtime age

  if [ -e "$marker" ]; then
    now="$(date +%%s 2>/dev/null || printf '%%s\n' 0)"
    mtime="$(stat -c %%Y "$marker" 2>/dev/null || printf '%%s\n' 0)"
    age=$((now - mtime))
    if [ "$age" -ge 0 ] && [ "$age" -lt "$debounce_seconds" ]; then
      return 0
    fi
  fi

  mkdir -p "$run_dir" 2>/dev/null
  : >"$marker" 2>/dev/null || true
  run_enforce
}

case "$action" in
  vpn-up|vpn-down)
    run_enforce
    ;;
  up|down|connectivity-change|dhcp4-change|dhcp6-change)
    run_with_debounce
    ;;
esac

exit 0
`, opts.RunDir, opts.DebounceSeconds, opts.BinPath, opts.ConfigPath)
}

func sleepHook(opts AssetsOptions) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -u

PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

case "${1:-}/${2:-}" in
  post/*)
    %s enforce "systemd-sleep:${2:-unknown}" --config %s &
    ;;
esac

exit 0
`, opts.BinPath, opts.ConfigPath)
}

func tmpfiles(opts AssetsOptions) string {
	return fmt.Sprintf("d %s 0755 root root -\n", opts.RunDir)
}

func logrotate(opts AssetsOptions) string {
	return fmt.Sprintf(`%s {
    weekly
    rotate 4
    missingok
    notifempty
    compress
    create 0644 root root
}
`, opts.LogFile)
}

func QuotePlan(data string) string {
	return strings.TrimRight(data, "\n")
}
