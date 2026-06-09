package domains

import (
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

var inputSplit = regexp.MustCompile(`[\s,;]+`)

type Lists struct {
	Bypass    []string
	BypassIPs []string
	Lock      []string
}

type configFile struct {
	Bypass struct {
		Domains []string `toml:"domains"`
		IPs     []string `toml:"ips"`
	} `toml:"bypass"`
	Killswitch struct {
		Domains []string `toml:"domains"`
	} `toml:"killswitch"`
}

func Load(path string) (Lists, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Lists{}, err
	}
	var cfg configFile
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Lists{}, err
	}
	return Lists{Bypass: cfg.Bypass.Domains, BypassIPs: cfg.Bypass.IPs, Lock: cfg.Killswitch.Domains}, nil
}

func Format(title string, values []string) string {
	if len(values) == 0 {
		return fmt.Sprintf("%s: список пуст", title)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d\n\n", title, len(values))
	for _, value := range values {
		fmt.Fprintf(&b, "%s\n", value)
	}
	return strings.TrimRight(b.String(), "\n")
}

func Short(title string, values []string) string {
	if len(values) == 0 {
		return fmt.Sprintf("%s: список пуст", title)
	}
	text := fmt.Sprintf("%s: %s", title, strings.Join(values, ", "))
	if len(text) > 220 {
		return fmt.Sprintf("%s: %d доменов", title, len(values))
	}
	return text
}

func ParseInput(input string) []string {
	values, _ := ParseDomainInput(input)
	return values
}

func ParseDomainInput(input string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, part := range inputSplit.Split(input, -1) {
		domain := strings.Trim(strings.ToLower(strings.TrimSpace(part)), ".")
		if domain == "" || seen[domain] {
			continue
		}
		if looksLikeIPEntry(domain) {
			return nil, fmt.Errorf("lock принимает только домены: %s", domain)
		}
		seen[domain] = true
		out = append(out, domain)
	}
	return out, nil
}

func ParseBypassInput(input string) ([]string, []string, error) {
	seenDomains := map[string]bool{}
	seenIPs := map[string]bool{}
	var domains []string
	var ips []string
	for _, part := range inputSplit.Split(input, -1) {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if looksLikeIPEntry(token) {
			normalized, err := NormalizeIPEntry(token)
			if err != nil {
				return nil, nil, err
			}
			if !seenIPs[normalized] {
				seenIPs[normalized] = true
				ips = append(ips, normalized)
			}
			continue
		}
		domain := strings.Trim(strings.ToLower(token), ".")
		if domain == "" || seenDomains[domain] {
			continue
		}
		seenDomains[domain] = true
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	sort.Strings(ips)
	return domains, ips, nil
}

func NormalizeIPEntry(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "-") {
		parts := strings.Split(raw, "-")
		if len(parts) != 2 {
			return "", fmt.Errorf("некорректный IPv4-диапазон: %s", raw)
		}
		start, err := parseIPv4(parts[0])
		if err != nil {
			return "", fmt.Errorf("некорректное начало IPv4-диапазона: %s", strings.TrimSpace(parts[0]))
		}
		end, err := parseIPv4(parts[1])
		if err != nil {
			return "", fmt.Errorf("некорректный конец IPv4-диапазона: %s", strings.TrimSpace(parts[1]))
		}
		if start.Compare(end) > 0 {
			return "", fmt.Errorf("некорректный IPv4-диапазон: начало больше конца")
		}
		return start.String() + "-" + end.String(), nil
	}
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || !prefix.Addr().Is4() {
			return "", fmt.Errorf("некорректный IPv4 CIDR: %s", raw)
		}
		return prefix.Masked().String(), nil
	}
	addr, err := parseIPv4(raw)
	if err != nil {
		return "", fmt.Errorf("некорректный IPv4-адрес: %s", raw)
	}
	return addr.String(), nil
}

func Contains(values []string, domain string) bool {
	for _, value := range values {
		if value == domain {
			return true
		}
	}
	return false
}

func All(lists Lists) []string {
	out := append([]string{}, lists.Bypass...)
	out = append(out, lists.BypassIPs...)
	out = append(out, lists.Lock...)
	return out
}

func BypassEntries(lists Lists) []string {
	out := append([]string{}, lists.Bypass...)
	out = append(out, lists.BypassIPs...)
	return out
}

func looksLikeIPEntry(raw string) bool {
	if strings.ContainsAny(raw, ":/") || isIPv4Literal(raw) {
		return true
	}
	if strings.Contains(raw, "-") {
		parts := strings.Split(raw, "-")
		if len(parts) == 2 {
			return isIPv4Literal(parts[0]) || isIPv4Literal(parts[1])
		}
	}
	return false
}

func isIPv4Literal(raw string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	return err == nil && addr.Is4()
}

func parseIPv4(raw string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || !addr.Is4() {
		return netip.Addr{}, fmt.Errorf("invalid IPv4")
	}
	return addr, nil
}
