package lockdns

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseResolvConf(t *testing.T) {
	got := ParseResolvConf(`
# comment
nameserver 127.0.0.53
nameserver 1.1.1.1
options edns0
`)
	want := []string{"127.0.0.53", "1.1.1.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseResolvConf() = %#v, want %#v", got, want)
	}
}

func TestParseResolvectlDNS(t *testing.T) {
	got := ParseResolvectlDNS(`
Global: 1.1.1.1 2606:4700:4700::1111
Link 2 (wlp3s0): 192.168.3.1
`)
	want := []string{"1.1.1.1", "2606:4700:4700::1111", "192.168.3.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseResolvectlDNS() = %#v, want %#v", got, want)
	}
}

func TestDNSServersUsesResolvectlWhenResolvConfHasStub(t *testing.T) {
	dir := t.TempDir()
	resolvConf := filepath.Join(dir, "resolv.conf")
	if err := os.WriteFile(resolvConf, []byte("nameserver 127.0.0.53\n"), 0644); err != nil {
		t.Fatal(err)
	}
	resolver := Resolver{
		ResolvConf:    resolvConf,
		ResolvectlDNS: []string{"8.8.8.8", "8.8.4.4"},
	}
	got, err := resolver.DNSServers()
	if err != nil {
		t.Fatalf("DNSServers() error = %v", err)
	}
	want := []string{"8.8.8.8", "8.8.4.4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DNSServers() = %#v, want %#v", got, want)
	}
}
