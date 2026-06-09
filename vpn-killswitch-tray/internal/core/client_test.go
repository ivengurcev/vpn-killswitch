package core

import (
	"strings"
	"testing"
)

type fakeRunner struct {
	out    string
	errOut string
	err    error
	args   []string
	input  string
}

func (f *fakeRunner) Run(name string, args ...string) (string, string, error) {
	f.args = append([]string{name}, args...)
	return f.out, f.errOut, f.err
}

func (f *fakeRunner) RunWithInput(input string, name string, args ...string) (string, string, error) {
	f.input = input
	return f.Run(name, args...)
}

func TestClientStatus(t *testing.T) {
	runner := &fakeRunner{out: `{
  "vpn": {"status": "active"},
  "gateway": {},
  "bypass": {"route_count": 3},
  "lock": {"nft_active": false, "hosts_block": false},
  "config": {},
  "errors": []
}`}
	snapshot, err := (Client{CommandPath: "/usr/local/sbin/vpn-killswitch", ConfigPath: "/etc/config.toml", Runner: runner}).Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if snapshot.State.Kind != "ok" {
		t.Fatalf("state = %+v", snapshot.State)
	}
	if got := strings.Join(runner.args, " "); got != "/usr/local/sbin/vpn-killswitch status --json --config /etc/config.toml" {
		t.Fatalf("args = %s", got)
	}
}

func TestClientAddBypassDomainUsesPkexec(t *testing.T) {
	runner := &fakeRunner{out: "unchanged"}
	result, err := (Client{CommandPath: "/usr/local/sbin/vpn-killswitch", ConfigPath: "/etc/config.toml", Runner: runner}).AddBypassDomain("example.com")
	if err != nil {
		t.Fatalf("AddBypassDomain() error = %v", err)
	}
	if result.Stdout != "unchanged" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	if got := strings.Join(runner.args, " "); got != "pkexec /usr/local/sbin/vpn-killswitch config add-bypass example.com --config /etc/config.toml" {
		t.Fatalf("args = %s", got)
	}
}

func TestClientLockEnableUsesActor(t *testing.T) {
	runner := &fakeRunner{}
	_, err := (Client{CommandPath: "/usr/local/sbin/vpn-killswitch", Runner: runner}).LockEnable("tray")
	if err != nil {
		t.Fatalf("LockEnable() error = %v", err)
	}
	if got := strings.Join(runner.args, " "); got != "pkexec /usr/local/sbin/vpn-killswitch lock enable tray" {
		t.Fatalf("args = %s", got)
	}
}

func TestClientApplyTrayUsesSinglePkexecWithJSONInput(t *testing.T) {
	runner := &fakeRunner{out: `{"changed":true}`}
	result, err := (Client{CommandPath: "/usr/local/sbin/vpn-killswitch", ConfigPath: "/etc/config.toml", Runner: runner}).ApplyTray(ApplyTrayPayload{
		BypassDomains: []string{"ozon.ru"},
		BypassIPs:     []string{"93.184.216.0/24"},
		LockDomains:   []string{"lock.test"},
	}, true)
	if err != nil {
		t.Fatalf("ApplyTray() error = %v", err)
	}
	if result.Stdout == "" {
		t.Fatal("ApplyTray() stdout is empty")
	}
	if got := strings.Join(runner.args, " "); got != "pkexec /usr/local/sbin/vpn-killswitch config apply-tray --enforce --config /etc/config.toml" {
		t.Fatalf("args = %s", got)
	}
	for _, want := range []string{`"bypass_domains":["ozon.ru"]`, `"bypass_ips":["93.184.216.0/24"]`, `"lock_domains":["lock.test"]`} {
		if !strings.Contains(runner.input, want) {
			t.Fatalf("input = %s, want %s", runner.input, want)
		}
	}
}

func TestClientPkexecCancelled(t *testing.T) {
	runner := &fakeRunner{errOut: "Error executing command as another user: Request dismissed", err: errFake{}}
	result, err := (Client{CommandPath: "/usr/local/sbin/vpn-killswitch", Runner: runner}).Enforce("tray")
	if err == nil {
		t.Fatalf("Enforce() expected error")
	}
	if !result.Cancelled {
		t.Fatalf("Cancelled = false")
	}
}

type errFake struct{}

func (errFake) Error() string {
	return "exit status 126"
}
