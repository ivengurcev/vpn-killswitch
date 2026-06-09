package ipaddr

import (
	"fmt"
	"math/bits"
	"net/netip"
	"sort"
	"strings"
)

func IsUsablePublic(raw string) bool {
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return false
	}
	return !(addr.IsUnspecified() ||
		addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast())
}

func NormalizeBypassEntries(entries []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, raw := range entries {
		for _, token := range splitTokens(raw) {
			entry, err := ParseBypassEntry(token)
			if err != nil {
				return nil, err
			}
			if seen[entry.Value] {
				continue
			}
			seen[entry.Value] = true
			out = append(out, entry.Value)
		}
	}
	sort.Strings(out)
	return out, nil
}

func ExpandBypassEntries(entries []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, raw := range entries {
		entry, err := ParseBypassEntry(raw)
		if err != nil {
			return nil, err
		}
		for _, prefix := range entry.Prefixes {
			if seen[prefix] {
				continue
			}
			seen[prefix] = true
			out = append(out, prefix)
		}
	}
	sort.Strings(out)
	return out, nil
}

type BypassEntry struct {
	Value    string
	Prefixes []string
}

func ParseBypassEntry(raw string) (BypassEntry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return BypassEntry{}, fmt.Errorf("empty bypass IP entry")
	}
	if strings.Contains(raw, "-") {
		return parseRange(raw)
	}
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || !prefix.Addr().Is4() {
			return BypassEntry{}, fmt.Errorf("invalid IPv4 CIDR: %s", raw)
		}
		prefix = prefix.Masked()
		return BypassEntry{Value: prefix.String(), Prefixes: []string{prefix.String()}}, nil
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil || !addr.Is4() {
		return BypassEntry{}, fmt.Errorf("invalid IPv4 address: %s", raw)
	}
	return BypassEntry{Value: addr.String(), Prefixes: []string{addr.String() + "/32"}}, nil
}

func parseRange(raw string) (BypassEntry, error) {
	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return BypassEntry{}, fmt.Errorf("invalid IPv4 range: %s", raw)
	}
	start, err := parseIPv4Addr(parts[0])
	if err != nil {
		return BypassEntry{}, fmt.Errorf("invalid IPv4 range start: %s", strings.TrimSpace(parts[0]))
	}
	end, err := parseIPv4Addr(parts[1])
	if err != nil {
		return BypassEntry{}, fmt.Errorf("invalid IPv4 range end: %s", strings.TrimSpace(parts[1]))
	}
	startInt := addrToUint32(start)
	endInt := addrToUint32(end)
	if startInt > endInt {
		return BypassEntry{}, fmt.Errorf("invalid IPv4 range: start is greater than end")
	}
	return BypassEntry{
		Value:    start.String() + "-" + end.String(),
		Prefixes: rangeToPrefixes(startInt, endInt),
	}, nil
}

func parseIPv4Addr(raw string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || !addr.Is4() {
		return netip.Addr{}, fmt.Errorf("invalid IPv4 address")
	}
	return addr, nil
}

func rangeToPrefixes(start, end uint32) []string {
	var prefixes []string
	current := uint64(start)
	last := uint64(end)
	for current <= last {
		remaining := last - current + 1
		trailing := 32
		if current != 0 {
			trailing = bits.TrailingZeros32(uint32(current))
		}
		maxByAlignment := uint64(1) << trailing
		maxByRemaining := uint64(1) << (bits.Len64(remaining) - 1)
		size := maxByAlignment
		if maxByRemaining < size {
			size = maxByRemaining
		}
		prefixLen := 32 - bits.TrailingZeros64(size)
		addr := uint32ToAddr(uint32(current))
		prefixes = append(prefixes, netip.PrefixFrom(addr, prefixLen).String())
		current += size
	}
	return prefixes
}

func addrToUint32(addr netip.Addr) uint32 {
	bytes := addr.As4()
	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}

func uint32ToAddr(value uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(value >> 24),
		byte(value >> 16),
		byte(value >> 8),
		byte(value),
	})
}

func splitTokens(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	return strings.Fields(raw)
}
