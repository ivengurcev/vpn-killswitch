package system

import (
	"strings"
	"testing"
)

func TestBuildAssetsContainsExpectedHooks(t *testing.T) {
	assets := BuildAssets(DefaultAssetsOptions())
	byPath := map[string]FileAsset{}
	for _, asset := range assets {
		byPath[asset.Path] = asset
	}
	if !strings.Contains(byPath[ServicePath].Data, "vpn-killswitch enforce systemd") {
		t.Fatalf("service missing enforce command:\n%s", byPath[ServicePath].Data)
	}
	if !strings.Contains(byPath[RefreshSvcPath].Data, "vpn-killswitch lock refresh timer") {
		t.Fatalf("refresh service missing lock refresh:\n%s", byPath[RefreshSvcPath].Data)
	}
	if !strings.Contains(byPath[DispatcherPath].Data, "networkmanager:${action}:${interface}") {
		t.Fatalf("dispatcher missing reason:\n%s", byPath[DispatcherPath].Data)
	}
	if byPath[DispatcherPath].Mode != 0755 || byPath[SleepHookPath].Mode != 0755 {
		t.Fatalf("hooks must be executable")
	}
}
