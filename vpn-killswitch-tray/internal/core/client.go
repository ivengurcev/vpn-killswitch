package core

import (
	"fmt"
	"strings"

	"vpn-killswitch-tray/internal/run"
	"vpn-killswitch-tray/internal/state"
	"vpn-killswitch-tray/internal/status"
)

type Client struct {
	CommandPath string
	ConfigPath  string
	Runner      run.Runner
}

type Snapshot struct {
	Status status.Status `json:"status"`
	State  state.State   `json:"state"`
}

type ActionResult struct {
	Stdout    string
	Stderr    string
	Cancelled bool
}

func (c Client) Status() (Snapshot, error) {
	if c.CommandPath == "" {
		return Snapshot{State: state.OfflineState("vpn-killswitch command path is empty")}, fmt.Errorf("vpn-killswitch command path is empty")
	}
	runner := c.Runner
	if runner == nil {
		runner = run.ExecRunner{}
	}
	args := []string{"status", "--json"}
	if c.ConfigPath != "" {
		args = append(args, "--config", c.ConfigPath)
	}
	out, _, err := runner.Run(c.CommandPath, args...)
	if err != nil {
		return Snapshot{State: state.OfflineState(err.Error())}, err
	}
	parsed, err := status.Parse([]byte(out))
	if err != nil {
		return Snapshot{State: state.OfflineState(err.Error())}, err
	}
	return Snapshot{Status: parsed, State: state.Normalize(parsed)}, nil
}

func (c Client) Enforce(actor string) (ActionResult, error) {
	return c.runPkexec("enforce", actor)
}

func (c Client) LockEnable(actor string) (ActionResult, error) {
	return c.runPkexec("lock", "enable", actor)
}

func (c Client) LockDisable(actor string) (ActionResult, error) {
	return c.runPkexec("lock", "disable", actor)
}

func (c Client) LockRefresh(actor string) (ActionResult, error) {
	return c.runPkexec("lock", "refresh", actor)
}

func (c Client) AddBypassDomain(domain string) (ActionResult, error) {
	return c.runPkexec("config", "add-bypass", domain)
}

func (c Client) AddLockDomain(domain string) (ActionResult, error) {
	return c.runPkexec("config", "add-lock", domain)
}

func (c Client) RemoveBypassDomain(domain string) (ActionResult, error) {
	return c.runPkexec("config", "remove-bypass", domain)
}

func (c Client) RemoveLockDomain(domain string) (ActionResult, error) {
	return c.runPkexec("config", "remove-lock", domain)
}

func (c Client) runPkexec(args ...string) (ActionResult, error) {
	if c.CommandPath == "" {
		return ActionResult{}, fmt.Errorf("vpn-killswitch command path is empty")
	}
	runner := c.Runner
	if runner == nil {
		runner = run.ExecRunner{}
	}
	fullArgs := []string{c.CommandPath}
	fullArgs = append(fullArgs, args...)
	if c.ConfigPath != "" {
		fullArgs = append(fullArgs, "--config", c.ConfigPath)
	}
	out, stderr, err := runner.Run("pkexec", fullArgs...)
	result := ActionResult{Stdout: out, Stderr: stderr, Cancelled: isPkexecCancelled(stderr, err)}
	if err != nil {
		return result, err
	}
	return result, nil
}

func isPkexecCancelled(stderr string, err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(stderr + " " + err.Error())
	return strings.Contains(text, "dismissed") ||
		strings.Contains(text, "cancelled") ||
		strings.Contains(text, "canceled") ||
		strings.Contains(text, "authentication failed")
}
