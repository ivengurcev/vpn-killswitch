package routes

import "testing"

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(name string, args ...string) (string, string, error) {
	r.name = name
	r.args = append([]string{}, args...)
	return "", "", nil
}

func TestParseRouteGet(t *testing.T) {
	got := ParseRouteGet("185.73.193.68 via 192.168.3.1 dev wlp3s0 src 192.168.3.20 uid 1000")
	if got.Via != "192.168.3.1" || got.Dev != "wlp3s0" {
		t.Fatalf("route = %+v", got)
	}
}

func TestDeleteRouteUsesExactStateParameters(t *testing.T) {
	runner := &recordingRunner{}
	err := DeleteRoute(runner, StateRoute{IP: "185.73.193.68", Gateway: "192.168.3.1", Dev: "wlp3s0"})
	if err != nil {
		t.Fatalf("DeleteRoute() error = %v", err)
	}
	want := []string{"route", "del", "185.73.193.68/32", "via", "192.168.3.1", "dev", "wlp3s0"}
	if runner.name != "ip" {
		t.Fatalf("name = %s", runner.name)
	}
	if len(runner.args) != len(want) {
		t.Fatalf("args = %+v", runner.args)
	}
	for i := range want {
		if runner.args[i] != want[i] {
			t.Fatalf("args = %+v", runner.args)
		}
	}
}
