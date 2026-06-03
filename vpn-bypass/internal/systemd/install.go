package systemd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vpn-bypass/internal/config"
	"vpn-bypass/internal/run"
)

const (
	BinPath     = "/usr/local/bin/vpn-bypass"
	ServicePath = "/etc/systemd/system/vpn-bypass-routes.service"
	TimerPath   = "/etc/systemd/system/vpn-bypass-routes.timer"
)

const ServiceContent = `[Unit]
Description=Update routes for domains bypassing OpenVPN
Wants=network-online.target
After=network-online.target NetworkManager.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/vpn-bypass apply
`

const TimerContent = `[Unit]
Description=Periodically update routes for domains bypassing OpenVPN

[Timer]
OnBootSec=30
OnUnitActiveSec=10min
Unit=vpn-bypass-routes.service

[Install]
WantedBy=timers.target
`

type InstallOptions struct {
	ConfigPath string
	DryRun     bool
	NoCopy     bool
}

func Install(opts InstallOptions, runner run.Runner, printf func(string, ...any)) error {
	if !opts.NoCopy {
		current, err := os.Executable()
		if err != nil {
			return err
		}
		current, _ = filepath.EvalSymlinks(current)
		if current != BinPath {
			if opts.DryRun {
				printf("DRY-RUN copy binary: %s -> %s\n", current, BinPath)
			} else if err := copyBinary(current, BinPath); err != nil {
				return err
			}
		}
	}
	if opts.DryRun {
		printf("DRY-RUN ensure config: %s\n", opts.ConfigPath)
		printf("DRY-RUN write file: %s\n%s", ServicePath, ServiceContent)
		printf("DRY-RUN write file: %s\n%s", TimerPath, TimerContent)
		return nil
	}
	if err := config.Ensure(opts.ConfigPath); err != nil {
		return err
	}
	if err := os.WriteFile(ServicePath, []byte(ServiceContent), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(TimerPath, []byte(TimerContent), 0644); err != nil {
		return err
	}
	return nil
}

func Activate(dryRun bool, runner run.Runner, printf func(string, ...any)) error {
	if dryRun {
		printf("DRY-RUN systemctl daemon-reload\n")
		printf("DRY-RUN systemctl enable --now vpn-bypass-routes.timer\n")
		printf("DRY-RUN systemctl start vpn-bypass-routes.service\n")
		return nil
	}
	if _, _, err := runner.Run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, _, err := runner.Run("systemctl", "enable", "--now", "vpn-bypass-routes.timer"); err != nil {
		return err
	}
	if _, _, err := runner.Run("systemctl", "start", "vpn-bypass-routes.service"); err != nil {
		return err
	}
	return nil
}

func copyBinary(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".vpn-bypass-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return err
	}
	_ = os.Chown(tmpPath, 0, 0)
	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func Uninstall(dryRun, purge bool, runner run.Runner, printf func(string, ...any)) error {
	if dryRun {
		printf("DRY-RUN systemctl disable --now vpn-bypass-routes.timer\n")
		printf("DRY-RUN remove: %s\n", ServicePath)
		printf("DRY-RUN remove: %s\n", TimerPath)
		printf("DRY-RUN systemctl daemon-reload\n")
		if purge {
			printf("DRY-RUN remove directory: /etc/vpn-bypass\n")
		}
		return nil
	}
	_, _, _ = runner.Run("systemctl", "disable", "--now", "vpn-bypass-routes.timer")
	for _, path := range []string{ServicePath, TimerPath} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	if purge {
		if err := os.RemoveAll("/etc/vpn-bypass"); err != nil {
			return err
		}
	}
	_, _, err := runner.Run("systemctl", "daemon-reload")
	return err
}
