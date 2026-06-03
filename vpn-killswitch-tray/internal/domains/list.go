package domains

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

var inputSplit = regexp.MustCompile(`[\s,;]+`)

type Lists struct {
	Bypass []string
	Lock   []string
}

type configFile struct {
	Bypass struct {
		Domains []string `toml:"domains"`
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
	return Lists{Bypass: cfg.Bypass.Domains, Lock: cfg.Killswitch.Domains}, nil
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
	seen := map[string]bool{}
	var out []string
	for _, part := range inputSplit.Split(input, -1) {
		domain := strings.Trim(strings.ToLower(strings.TrimSpace(part)), ".")
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		out = append(out, domain)
	}
	return out
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
	out = append(out, lists.Lock...)
	return out
}
