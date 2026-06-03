package state

import (
	"fmt"
	"strings"

	"vpn-killswitch-tray/internal/status"
)

type Kind string

const (
	Offline Kind = "offline"
	OK      Kind = "ok"
	Locked  Kind = "locked"
	Warning Kind = "warning"
	Error   Kind = "error"
)

type State struct {
	Kind    Kind
	Summary string
	Detail  string
}

func OfflineState(reason string) State {
	return State{Kind: Offline, Summary: "vpn-killswitch offline", Detail: reason}
}

func Normalize(s status.Status) State {
	lockActive := s.Lock.NFTActive || s.Lock.HostsBlock
	if len(s.Errors) > 0 || s.Lock.Error != "" {
		return State{
			Kind:    Error,
			Summary: fmt.Sprintf("VPN: %s | Lock: %s | Errors: %d", vpnStatus(s), lockText(lockActive), len(s.Errors)+boolCount(s.Lock.Error != "")),
			Detail:  strings.Join(append(append([]string{}, s.Errors...), s.Lock.Error), "; "),
		}
	}
	switch s.VPN.Status {
	case "active":
		if lockActive {
			return State{Kind: Warning, Summary: fmt.Sprintf("VPN: active | Lock: on | Bypass routes: %d", s.Bypass.RouteCount), Detail: "lock is active while VPN is active"}
		}
		return State{Kind: OK, Summary: fmt.Sprintf("VPN: active | Lock: off | Bypass routes: %d", s.Bypass.RouteCount)}
	case "inactive", "unknown":
		if lockActive {
			return State{Kind: Locked, Summary: fmt.Sprintf("VPN: %s | Lock: on | Bypass routes: %d", s.VPN.Status, s.Bypass.RouteCount)}
		}
		return State{Kind: Warning, Summary: fmt.Sprintf("VPN: %s | Lock: off | Bypass routes: %d", s.VPN.Status, s.Bypass.RouteCount), Detail: "lock is not active while VPN is inactive/unknown"}
	default:
		return State{Kind: Warning, Summary: fmt.Sprintf("VPN: %s | Lock: %s | Bypass routes: %d", vpnStatus(s), lockText(lockActive), s.Bypass.RouteCount), Detail: "unexpected VPN status"}
	}
}

func vpnStatus(s status.Status) string {
	if s.VPN.Status == "" {
		return "unknown"
	}
	return s.VPN.Status
}

func lockText(active bool) string {
	if active {
		return "on"
	}
	return "off"
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
