package bypass

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadStateSupportsLegacyFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.v4")
	if err := os.WriteFile(path, []byte("185.73.193.68 192.168.3.1 wlp3s0 ozon.ru\n"), 0644); err != nil {
		t.Fatal(err)
	}
	routes, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("len(routes) = %d, want 1", len(routes))
	}
	route := routes[0]
	if route.Target != "185.73.193.68/32" || route.SourceType != "domain" || route.Source != "ozon.ru" {
		t.Fatalf("route = %+v", route)
	}
}

func TestWriteAndReadStateUsesSourceFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.v4")
	err := WriteState(path, []Route{
		{Target: "93.184.216.0/24", Gateway: "192.168.3.1", Dev: "wlp3s0", SourceType: "ip", Source: "93.184.216.0/24"},
	})
	if err != nil {
		t.Fatalf("WriteState() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "93.184.216.0/24 192.168.3.1 wlp3s0 ip 93.184.216.0/24\n"; got != want {
		t.Fatalf("state = %q, want %q", got, want)
	}
	routes, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if len(routes) != 1 || routes[0].SourceType != "ip" || routes[0].Source != "93.184.216.0/24" {
		t.Fatalf("routes = %+v", routes)
	}
}
