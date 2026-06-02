package hosts

import (
	"strings"
	"testing"
)

func TestAddBlockReplacesExistingBlock(t *testing.T) {
	content := "127.0.0.1 localhost\n\n" + DefaultBegin + "\n0.0.0.0 old.test\n" + DefaultEnd + "\n"
	got := AddBlock(content, []string{"example.com"}, DefaultBegin, DefaultEnd)
	if strings.Contains(got, "old.test") {
		t.Fatalf("old block was not removed:\n%s", got)
	}
	if !strings.Contains(got, "0.0.0.0 example.com") || !strings.Contains(got, ":: example.com") {
		t.Fatalf("new block missing:\n%s", got)
	}
	if !strings.Contains(got, "127.0.0.1 localhost") {
		t.Fatalf("unmanaged content missing:\n%s", got)
	}
}

func TestStripBlock(t *testing.T) {
	content := "a\n" + DefaultBegin + "\nb\n" + DefaultEnd + "\nc\n"
	got := StripBlock(content, DefaultBegin, DefaultEnd)
	if got != "a\nc\n" {
		t.Fatalf("StripBlock() = %q", got)
	}
}
