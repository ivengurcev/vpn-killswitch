package config

import (
	"strings"
	"testing"
)

func TestDecodeAppliesDefaultsAndNormalizes(t *testing.T) {
	cfg, err := Decode(`
[vpn]
connection_name = "Work VPN"

[bypass]
domains = ["OZON.RU.", "wildberries.ru", "ozon.ru"]
`)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if cfg.VPN.ConnectionName != "Work VPN" {
		t.Fatalf("connection name = %q", cfg.VPN.ConnectionName)
	}
	if got, want := strings.Join(cfg.VPN.ConnectionTypes, ","), "vpn,wireguard"; got != want {
		t.Fatalf("connection types = %q, want %q", got, want)
	}
	if got, want := strings.Join(cfg.Bypass.Domains, ","), "ozon.ru,wildberries.ru"; got != want {
		t.Fatalf("bypass domains = %q, want %q", got, want)
	}
	if cfg.Killswitch.ManageHosts != true {
		t.Fatalf("manage hosts default = false")
	}
	if cfg.Killswitch.NFTTableName != DefaultNFTTableName {
		t.Fatalf("nft table default = %q", cfg.Killswitch.NFTTableName)
	}
}

func TestDecodeRejectsDomainOverlap(t *testing.T) {
	_, err := Decode(`
[vpn]
connection_name = "Work VPN"

[bypass]
domains = ["example.com"]

[killswitch]
domains = ["EXAMPLE.COM."]
`)
	if err == nil {
		t.Fatal("Decode() expected overlap error")
	}
	if !strings.Contains(err.Error(), "both bypass.domains and killswitch.domains") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeRequiresAtLeastOneDomainList(t *testing.T) {
	_, err := Decode(`
[vpn]
connection_name = "Work VPN"
`)
	if err == nil {
		t.Fatal("Decode() expected empty domains error")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeRejectsBadNFTIdentifier(t *testing.T) {
	_, err := Decode(`
[vpn]
connection_name = "Work VPN"

[killswitch]
domains = ["example.com"]
nft_table_name = "bad-name"
`)
	if err == nil {
		t.Fatal("Decode() expected nft identifier error")
	}
	if !strings.Contains(err.Error(), "nft_table_name") {
		t.Fatalf("error = %v", err)
	}
}
