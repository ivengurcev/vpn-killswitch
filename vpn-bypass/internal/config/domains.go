package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultPath = "/etc/vpn-bypass/domains.txt"

func ReadDomains(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := map[string]bool{}
	var domains []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		line = strings.ToLower(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		domains = append(domains, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return domains, nil
}

func Ensure(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return f.Close()
}

func AddDomain(path, domain string, dryRun bool) (bool, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return false, fmt.Errorf("empty domain")
	}
	domains, err := readDomainsIfExists(path)
	if err != nil {
		return false, err
	}
	for _, current := range domains {
		if current == domain {
			return false, nil
		}
	}
	if dryRun {
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if len(domains) > 0 {
		if _, err := fmt.Fprintln(f); err != nil {
			return false, err
		}
	}
	_, err = fmt.Fprintln(f, domain)
	return true, err
}

func RemoveDomain(path, domain string, dryRun bool) (bool, error) {
	domain = normalizeDomain(domain)
	domains, err := ReadDomains(path)
	if err != nil {
		return false, err
	}
	var next []string
	removed := false
	for _, current := range domains {
		if current == domain {
			removed = true
			continue
		}
		next = append(next, current)
	}
	if !removed || dryRun {
		return removed, nil
	}
	sort.Strings(next)
	data := strings.Join(next, "\n")
	if data != "" {
		data += "\n"
	}
	return true, os.WriteFile(path, []byte(data), 0644)
}

func readDomainsIfExists(path string) ([]string, error) {
	domains, err := ReadDomains(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return domains, err
}

func normalizeDomain(domain string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(domain)), ".")
}
