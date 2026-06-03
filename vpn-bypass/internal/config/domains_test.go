package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDomainsIgnoresCommentsAndDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "domains.txt")
	data := "# comment\nOZON.ru\n\nwww.ozon.ru # inline\nozon.ru\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDomains(path)
	if err != nil {
		t.Fatalf("ReadDomains() error = %v", err)
	}
	if len(got) != 2 || got[0] != "ozon.ru" || got[1] != "www.ozon.ru" {
		t.Fatalf("got = %+v", got)
	}
}
