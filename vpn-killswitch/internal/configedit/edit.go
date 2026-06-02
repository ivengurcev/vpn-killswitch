package configedit

import (
	"fmt"

	"vpn-killswitch/internal/config"
)

type List string

const (
	Bypass List = "bypass"
	Lock   List = "lock"
)

type Result struct {
	Changed bool          `json:"changed"`
	Domain  string        `json:"domain"`
	Config  config.Config `json:"config"`
}

func Add(path string, list List, domain string) (Result, error) {
	cfg, err := config.Read(path)
	if err != nil {
		return Result{}, err
	}
	domains := config.NormalizeDomains([]string{domain})
	if len(domains) != 1 {
		return Result{}, fmt.Errorf("invalid domain: %s", domain)
	}
	domain = domains[0]
	if contains(cfg.Bypass.Domains, domain) || contains(cfg.Killswitch.Domains, domain) {
		return Result{Changed: false, Domain: domain, Config: cfg}, nil
	}
	switch list {
	case Bypass:
		cfg.Bypass.Domains = append(cfg.Bypass.Domains, domain)
	case Lock:
		cfg.Killswitch.Domains = append(cfg.Killswitch.Domains, domain)
	default:
		return Result{}, fmt.Errorf("unknown list: %s", list)
	}
	if err := config.Write(path, cfg); err != nil {
		return Result{}, err
	}
	return Result{Changed: true, Domain: domain, Config: cfg}, nil
}

func Remove(path string, list List, domain string) (Result, error) {
	cfg, err := config.Read(path)
	if err != nil {
		return Result{}, err
	}
	domains := config.NormalizeDomains([]string{domain})
	if len(domains) != 1 {
		return Result{}, fmt.Errorf("invalid domain: %s", domain)
	}
	domain = domains[0]
	var changed bool
	switch list {
	case Bypass:
		cfg.Bypass.Domains, changed = remove(cfg.Bypass.Domains, domain)
	case Lock:
		cfg.Killswitch.Domains, changed = remove(cfg.Killswitch.Domains, domain)
	default:
		return Result{}, fmt.Errorf("unknown list: %s", list)
	}
	if changed {
		if err := config.Write(path, cfg); err != nil {
			return Result{}, err
		}
	}
	return Result{Changed: changed, Domain: domain, Config: cfg}, nil
}

func contains(domains []string, domain string) bool {
	for _, current := range domains {
		if current == domain {
			return true
		}
	}
	return false
}

func remove(domains []string, domain string) ([]string, bool) {
	var out []string
	changed := false
	for _, current := range domains {
		if current == domain {
			changed = true
			continue
		}
		out = append(out, current)
	}
	return out, changed
}
