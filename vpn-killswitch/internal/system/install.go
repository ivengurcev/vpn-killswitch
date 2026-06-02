package system

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"vpn-killswitch/internal/config"
	"vpn-killswitch/internal/run"
)

type InstallOptions struct {
	DryRun   bool
	NoCopy   bool
	NoEnable bool
	NoStart  bool
	DestDir  string
}

type UninstallOptions struct {
	DryRun  bool
	Purge   bool
	DestDir string
}

type Printer func(string, ...any)

func Install(opts InstallOptions, runner run.Runner, printf Printer) error {
	assetsOpts := DefaultAssetsOptions()
	if !opts.NoCopy {
		current, err := os.Executable()
		if err != nil {
			return err
		}
		current, _ = filepath.EvalSymlinks(current)
		dst := target(opts.DestDir, BinPath)
		if current != dst {
			if opts.DryRun {
				printf("DRY-RUN copy binary: %s -> %s\n", current, dst)
			} else if err := copyBinary(current, dst); err != nil {
				return err
			}
		}
	}

	if opts.DryRun {
		printf("DRY-RUN ensure config: %s\n", target(opts.DestDir, ConfigPath))
	} else if err := config.EnsureExample(target(opts.DestDir, ConfigPath)); err != nil {
		return err
	}

	for _, asset := range BuildAssets(assetsOpts) {
		path := target(opts.DestDir, asset.Path)
		if opts.DryRun {
			printf("DRY-RUN write file: %s mode %04o\n%s\n", path, asset.Mode, QuotePlan(asset.Data))
			continue
		}
		if err := writeFile(path, []byte(asset.Data), os.FileMode(asset.Mode)); err != nil {
			return err
		}
	}

	if opts.DestDir != "" {
		return nil
	}
	if opts.DryRun {
		printf("DRY-RUN systemctl daemon-reload\n")
		if !opts.NoEnable {
			printf("DRY-RUN systemctl enable --now vpn-killswitch.service vpn-killswitch-refresh.timer\n")
		}
		if !opts.NoStart {
			printf("DRY-RUN systemctl start vpn-killswitch.service\n")
		}
		return nil
	}
	if _, _, err := runner.Run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if !opts.NoEnable {
		if _, _, err := runner.Run("systemctl", "enable", "--now", "vpn-killswitch.service", "vpn-killswitch-refresh.timer"); err != nil {
			return err
		}
	}
	if !opts.NoStart {
		if _, _, err := runner.Run("systemctl", "start", "vpn-killswitch.service"); err != nil {
			return err
		}
	}
	return nil
}

func Uninstall(opts UninstallOptions, runner run.Runner, printf Printer) error {
	paths := []string{
		BinPath, ServicePath, RefreshSvcPath, TimerPath, SleepHookPath, DispatcherPath, TmpfilesPath, LogrotatePath,
	}
	if opts.DryRun {
		if opts.DestDir == "" {
			printf("DRY-RUN vpn-killswitch lock disable uninstall\n")
			printf("DRY-RUN remove bypass routes from state: /run/vpn-killswitch/bypass-routes.v4\n")
			printf("DRY-RUN systemctl disable --now vpn-killswitch-refresh.timer vpn-killswitch.service\n")
			printf("DRY-RUN systemctl stop vpn-killswitch-refresh.service\n")
		}
		for _, path := range paths {
			printf("DRY-RUN remove: %s\n", target(opts.DestDir, path))
		}
		if opts.Purge {
			printf("DRY-RUN remove directory: %s\n", target(opts.DestDir, "/etc/vpn-killswitch"))
			printf("DRY-RUN remove directory: %s\n", target(opts.DestDir, "/var/lib/vpn-killswitch"))
			printf("DRY-RUN remove directory: %s\n", target(opts.DestDir, "/run/vpn-killswitch"))
			printf("DRY-RUN remove file: %s\n", target(opts.DestDir, "/var/log/vpn-killswitch.log"))
		}
		if opts.DestDir == "" {
			printf("DRY-RUN systemctl daemon-reload\n")
		}
		return nil
	}

	if opts.DestDir == "" {
		_, _, _ = runner.Run(BinPath, "lock", "disable", "uninstall")
		_ = removeBypassRoutes("/run/vpn-killswitch/bypass-routes.v4", runner)
		_, _, _ = runner.Run("systemctl", "disable", "--now", "vpn-killswitch-refresh.timer", "vpn-killswitch.service")
		_, _, _ = runner.Run("systemctl", "stop", "vpn-killswitch-refresh.service")
	}
	for _, path := range paths {
		if err := removePath(target(opts.DestDir, path)); err != nil {
			return err
		}
	}
	if opts.Purge {
		for _, path := range []string{"/etc/vpn-killswitch", "/var/lib/vpn-killswitch", "/run/vpn-killswitch"} {
			if err := os.RemoveAll(target(opts.DestDir, path)); err != nil {
				return err
			}
		}
		if err := removePath(target(opts.DestDir, "/var/log/vpn-killswitch.log")); err != nil {
			return err
		}
	}
	if opts.DestDir == "" {
		_, _, err := runner.Run("systemctl", "daemon-reload")
		return err
	}
	return nil
}

func target(destDir, path string) string {
	if destDir == "" || destDir == "/" {
		return path
	}
	return filepath.Join(destDir, path)
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
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
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".vpn-killswitch-*")
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

func removePath(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func removeBypassRoutes(statePath string, runner run.Runner) error {
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		_, _, _ = runner.Run("ip", "route", "del", fields[0]+"/32", "via", fields[1], "dev", fields[2])
	}
	return nil
}
