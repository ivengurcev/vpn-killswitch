package configedit

import (
	"encoding/json"
	"fmt"
	"reflect"

	"vpn-killswitch/internal/config"
	"vpn-killswitch/internal/ipaddr"
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

type ApplyTrayPayload struct {
	BypassDomains []string `json:"bypass_domains"`
	LockDomains   []string `json:"lock_domains"`
	BypassIPs     []string `json:"bypass_ips,omitempty"`

	hasBypassDomains bool
	hasBypassIPs     bool
	hasLockDomains   bool
}

type ApplyTrayResult struct {
	Changed       bool          `json:"changed"`
	BypassDomains []string      `json:"bypass_domains"`
	BypassIPs     []string      `json:"bypass_ips"`
	LockDomains   []string      `json:"lock_domains"`
	Config        config.Config `json:"config"`
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

func ApplyTray(path string, payload ApplyTrayPayload, dryRun bool) (ApplyTrayResult, error) {
	cfg, err := config.Read(path)
	if err != nil {
		return ApplyTrayResult{}, err
	}
	if !payload.hasBypassDomains || !payload.hasBypassIPs || !payload.hasLockDomains {
		return ApplyTrayResult{}, fmt.Errorf("apply-tray payload must include bypass_domains, bypass_ips and lock_domains")
	}
	next := cfg
	next.Bypass.Domains = config.NormalizeDomains(payload.BypassDomains)
	normalizedIPs, err := ipaddr.NormalizeBypassEntries(payload.BypassIPs)
	if err != nil {
		return ApplyTrayResult{}, err
	}
	next.Bypass.IPs = normalizedIPs
	next.Killswitch.Domains = config.NormalizeDomains(payload.LockDomains)
	if err := next.Validate(); err != nil {
		return ApplyTrayResult{}, err
	}

	changed := !reflect.DeepEqual(cfg.Bypass.Domains, next.Bypass.Domains) ||
		!reflect.DeepEqual(cfg.Bypass.IPs, next.Bypass.IPs) ||
		!reflect.DeepEqual(cfg.Killswitch.Domains, next.Killswitch.Domains)
	if changed && !dryRun {
		if err := config.Write(path, next); err != nil {
			return ApplyTrayResult{}, err
		}
	}
	return ApplyTrayResult{
		Changed:       changed,
		BypassDomains: next.Bypass.Domains,
		BypassIPs:     next.Bypass.IPs,
		LockDomains:   next.Killswitch.Domains,
		Config:        next,
	}, nil
}

func (p *ApplyTrayPayload) UnmarshalJSON(data []byte) error {
	type payload ApplyTrayPayload
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var decoded payload
	if value, ok := raw["bypass_domains"]; ok {
		p.hasBypassDomains = true
		if err := json.Unmarshal(value, &decoded.BypassDomains); err != nil {
			return err
		}
	}
	if value, ok := raw["bypass_ips"]; ok {
		p.hasBypassIPs = true
		if err := json.Unmarshal(value, &decoded.BypassIPs); err != nil {
			return err
		}
	}
	if value, ok := raw["lock_domains"]; ok {
		p.hasLockDomains = true
		if err := json.Unmarshal(value, &decoded.LockDomains); err != nil {
			return err
		}
	}
	p.BypassDomains = decoded.BypassDomains
	p.BypassIPs = decoded.BypassIPs
	p.LockDomains = decoded.LockDomains
	return nil
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
