package system

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type noopRunner struct{}

func (noopRunner) Run(name string, args ...string) (string, string, error) {
	return "", "", nil
}

func TestInstallDryRunPrintsPlan(t *testing.T) {
	var out bytes.Buffer
	err := Install(InstallOptions{DryRun: true, NoCopy: true}, noopRunner{}, func(format string, args ...any) {
		_, _ = out.WriteString(sprintf(format, args...))
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "vpn-killswitch.service") || !strings.Contains(text, "NetworkManager/dispatcher.d/90-vpn-killswitch") {
		t.Fatalf("dry-run plan missing expected assets:\n%s", text)
	}
}

func TestInstallDestDirWritesAssets(t *testing.T) {
	dir := t.TempDir()
	err := Install(InstallOptions{DestDir: dir, NoCopy: true, NoEnable: true, NoStart: true}, noopRunner{}, func(string, ...any) {})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	for _, path := range []string{ConfigPath, ServicePath, DispatcherPath, SleepHookPath} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("expected staged file %s: %v", path, err)
		}
	}
}

func TestUninstallDryRunMentionsBypassCleanup(t *testing.T) {
	var out bytes.Buffer
	err := Uninstall(UninstallOptions{DryRun: true}, noopRunner{}, func(format string, args ...any) {
		_, _ = out.WriteString(sprintf(format, args...))
	})
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if !strings.Contains(out.String(), "remove bypass routes from state") {
		t.Fatalf("uninstall dry-run missing bypass cleanup:\n%s", out.String())
	}
}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
