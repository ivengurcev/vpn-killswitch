package run

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const DefaultTimeout = 3 * time.Second
const UserActionTimeout = 2 * time.Minute

type Runner interface {
	Run(name string, args ...string) (stdout string, stderr string, err error)
}

type InputRunner interface {
	Runner
	RunWithInput(input string, name string, args ...string) (stdout string, stderr string, err error)
}

type ExecRunner struct {
	Timeout time.Duration
}

func (r ExecRunner) Run(name string, args ...string) (string, string, error) {
	return r.run("", name, args...)
}

func (r ExecRunner) RunWithInput(input string, name string, args ...string) (string, string, error) {
	return r.run(input, name, args...)
}

func (r ExecRunner) run(input string, name string, args ...string) (string, string, error) {
	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	serr := strings.TrimSpace(stderr.String())
	if ctx.Err() == context.DeadlineExceeded {
		return out, serr, fmt.Errorf("%s timed out after %s", name, timeout)
	}
	if err != nil {
		if serr != "" {
			return out, serr, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, serr)
		}
		return out, serr, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, serr, nil
}
