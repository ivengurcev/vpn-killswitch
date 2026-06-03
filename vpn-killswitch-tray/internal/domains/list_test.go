package domains

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[bypass]
domains = ["ozon.ru", "wildberries.ru"]

[killswitch]
domains = ["example.com"]
`), 0644); err != nil {
		t.Fatal(err)
	}
	lists, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := strings.Join(lists.Bypass, ","); got != "ozon.ru,wildberries.ru" {
		t.Fatalf("bypass = %q", got)
	}
	if got := strings.Join(lists.Lock, ","); got != "example.com" {
		t.Fatalf("lock = %q", got)
	}
}

func TestFormatEmpty(t *testing.T) {
	if got := Format("Bypass", nil); got != "Bypass: список пуст" {
		t.Fatalf("Format() = %q", got)
	}
}

func TestParseInput(t *testing.T) {
	got := strings.Join(ParseInput("EXAMPLE.com, api.example.com\nexample.com; cdn.example.com."), ",")
	if got != "example.com,api.example.com,cdn.example.com" {
		t.Fatalf("ParseInput() = %q", got)
	}
}
