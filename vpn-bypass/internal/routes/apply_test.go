package routes

import (
	"errors"
	"io"
	"path/filepath"
	"testing"

	"vpn-bypass/internal/log"
)

type fakeRunner struct {
	outputs map[string]string
}

func (r fakeRunner) Run(name string, args ...string) (string, string, error) {
	key := name
	for _, arg := range args {
		key += " " + arg
	}
	out, ok := r.outputs[key]
	if !ok {
		return "", "", errors.New("unexpected command: " + key)
	}
	return out, "", nil
}

func TestApplyWithNoDomainsIsNoopSuccess(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "routes.v4")
	runner := fakeRunner{outputs: map[string]string{
		"ip -4 route show default": "default via 192.168.3.1 dev wlp3s0 metric 600",
	}}
	logger := log.Logger{Out: io.Discard, Err: io.Discard}

	result, err := Apply(
		ApplyOptions{StatePath: statePath},
		nil,
		runner,
		testResolver{},
		logger,
	)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(result.NewRoutes) != 0 {
		t.Fatalf("NewRoutes = %+v", result.NewRoutes)
	}
	state, err := ReadState(statePath)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if len(state) != 0 {
		t.Fatalf("state = %+v", state)
	}
}

type testResolver struct{}

func (testResolver) ResolveIPv4(string) ([]string, error) {
	return nil, errors.New("resolver should not be called")
}
