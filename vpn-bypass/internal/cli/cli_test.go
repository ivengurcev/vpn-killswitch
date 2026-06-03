package cli

import "testing"

func TestSplitArgsAllowsFlagsAfterPositional(t *testing.T) {
	flags, positional, err := splitArgs([]string{"ozon.ru", "--dry-run", "--config", "/tmp/domains.txt"})
	if err != nil {
		t.Fatalf("splitArgs() error = %v", err)
	}
	if len(positional) != 1 || positional[0] != "ozon.ru" {
		t.Fatalf("positional = %+v", positional)
	}
	if len(flags) != 3 {
		t.Fatalf("flags = %+v", flags)
	}
}

func TestValidateRejectsDryRunForReadOnly(t *testing.T) {
	err := validate("status", options{dryRun: true}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
