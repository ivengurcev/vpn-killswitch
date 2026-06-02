package enforce

import (
	"errors"
	"strings"
	"testing"

	"vpn-killswitch/internal/config"
	"vpn-killswitch/internal/lockdns"
	"vpn-killswitch/internal/log"
)

type fakeRunner struct {
	output map[string]string
	errs   map[string]error
	calls  []string
}

func (f *fakeRunner) Run(name string, args ...string) (string, string, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err := f.errs[key]; err != nil {
		return "", "", err
	}
	return f.output[key], "", nil
}

type fakeBypassResolver struct{}

func (fakeBypassResolver) ResolveIPv4(domain string) ([]string, error) {
	return []string{"1.1.1.1"}, nil
}

type fakeLockResolver struct{}

func (fakeLockResolver) Resolve(domain string) (lockdns.Result, error) {
	return lockdns.Result{IPv4: []string{"2.2.2.2"}}, nil
}

func TestRunEnablesLockWhenVPNInactive(t *testing.T) {
	runner := &fakeRunner{output: map[string]string{
		"ip -4 route show default":                       "default via 192.168.3.1 dev wlp3s0",
		"nmcli -t -f NAME,TYPE connection show --active": "Home:802-11-wireless",
	}, errs: map[string]error{}}
	cfg := testConfig()
	_, err := Run(Options{Config: cfg, DryRun: true}, runner, fakeBypassResolver{}, fakeLockResolver{}, log.Logger{Quiet: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunFailClosedWhenGatewayMissing(t *testing.T) {
	runner := &fakeRunner{output: map[string]string{}, errs: map[string]error{
		"ip -4 route show default": errors.New("no route"),
	}}
	cfg := testConfig()
	result, err := Run(Options{Config: cfg, DryRun: true}, runner, fakeBypassResolver{}, fakeLockResolver{}, log.Logger{Quiet: true})
	if err == nil {
		t.Fatal("Run() expected gateway error")
	}
	if !result.LockApplied {
		t.Fatalf("fail-closed lock was not applied: %+v", result)
	}
}

func testConfig() config.Config {
	cfg := config.Defaults()
	cfg.VPN.ConnectionName = "Work VPN"
	cfg.Bypass.Domains = []string{"bypass.test"}
	cfg.Killswitch.Domains = []string{"lock.test"}
	return cfg
}
