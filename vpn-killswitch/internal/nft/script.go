package nft

import (
	"fmt"
	"sort"
	"strings"
)

const (
	DefaultIPv4Set      = "blocked_ipv4"
	DefaultIPv6Set      = "blocked_ipv6"
	DefaultOutputChain  = "vpn_killswitch_output"
	DefaultForwardChain = "vpn_killswitch_forward"
)

type Spec struct {
	TableName    string
	IPv4         []string
	IPv6         []string
	IPv4Set      string
	IPv6Set      string
	OutputChain  string
	ForwardChain string
}

func BuildScript(spec Spec) string {
	spec = spec.withDefaults()
	ipv4 := sorted(spec.IPv4)
	ipv6 := sorted(spec.IPv6)
	return fmt.Sprintf(`table inet %s {
  set %s {
    type ipv4_addr
    flags interval
    elements = { %s }
  }

  set %s {
    type ipv6_addr
    flags interval
    elements = { %s }
  }

  chain %s {
    type filter hook output priority 0; policy accept;
    ip daddr @%s reject
    ip6 daddr @%s reject
  }

  chain %s {
    type filter hook forward priority 0; policy accept;
    ip daddr @%s reject
    ip6 daddr @%s reject
  }
}
`, spec.TableName, spec.IPv4Set, strings.Join(ipv4, ", "), spec.IPv6Set, strings.Join(ipv6, ", "), spec.OutputChain, spec.IPv4Set, spec.IPv6Set, spec.ForwardChain, spec.IPv4Set, spec.IPv6Set)
}

func DeleteTableArgs(tableName string) []string {
	return []string{"delete", "table", "inet", tableName}
}

func (s Spec) withDefaults() Spec {
	if s.IPv4Set == "" {
		s.IPv4Set = DefaultIPv4Set
	}
	if s.IPv6Set == "" {
		s.IPv6Set = DefaultIPv6Set
	}
	if s.OutputChain == "" {
		s.OutputChain = DefaultOutputChain
	}
	if s.ForwardChain == "" {
		s.ForwardChain = DefaultForwardChain
	}
	return s
}

func sorted(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}
