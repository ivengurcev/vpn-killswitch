package routes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadWriteState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.v4")
	want := []StateRoute{
		{IP: "185.73.194.82", Gateway: "192.168.3.1", Dev: "wlp3s0", Domain: "ozon.ru"},
		{IP: "185.73.193.68", Gateway: "192.168.3.1", Dev: "wlp3s0", Domain: "ozon.ru"},
	}
	if err := WriteState(path, want); err != nil {
		t.Fatalf("WriteState() error = %v", err)
	}
	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d", len(got))
	}
	if got[0].IP != "185.73.193.68" || got[1].IP != "185.73.194.82" {
		t.Fatalf("state was not sorted/read correctly: %+v", got)
	}
}

func TestReadStateMissingFile(t *testing.T) {
	got, err := ReadState(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v", got)
	}
}

func TestReadStateSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.v4")
	if err := os.WriteFile(path, []byte("bad\n1.1.1.1 192.168.1.1 eth0 example.com\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if len(got) != 1 || got[0].Domain != "example.com" {
		t.Fatalf("got = %+v", got)
	}
}
