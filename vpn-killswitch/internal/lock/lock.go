package lock

import (
	"fmt"
	"os"

	"vpn-killswitch/internal/hosts"
	"vpn-killswitch/internal/lockdns"
	"vpn-killswitch/internal/log"
	"vpn-killswitch/internal/nft"
	"vpn-killswitch/internal/run"
)

type DNSResolver interface {
	Resolve(domain string) (lockdns.Result, error)
}

type Options struct {
	Domains      []string
	NFTTableName string
	HostsFile    string
	ManageHosts  bool
	DryRun       bool
}

type Result struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

func Enable(opts Options, runner run.Runner, resolver DNSResolver, logger log.Logger) (Result, error) {
	result := ResolveDomains(opts.Domains, resolver, logger)
	if opts.DryRun {
		logger.Info("dry-run lock nft apply: table inet %s (%d IPv4, %d IPv6)", opts.NFTTableName, len(result.IPv4), len(result.IPv6))
	} else if err := ApplyNFT(opts.NFTTableName, result, runner); err != nil {
		return result, err
	}

	manager := hosts.Manager{Path: opts.HostsFile}
	if opts.ManageHosts {
		if opts.DryRun {
			logger.Info("dry-run hosts write managed block: %s (%d domains)", opts.HostsFile, len(opts.Domains))
		} else if err := manager.Enable(opts.Domains); err != nil {
			return result, err
		}
	} else {
		if opts.DryRun {
			logger.Info("dry-run hosts remove managed block: %s", opts.HostsFile)
		} else if err := manager.Disable(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func Disable(opts Options, runner run.Runner, logger log.Logger) error {
	if opts.DryRun {
		logger.Info("dry-run lock nft delete: table inet %s", opts.NFTTableName)
		logger.Info("dry-run hosts remove managed block: %s", opts.HostsFile)
		return nil
	}
	_, _, _ = runner.Run("nft", nft.DeleteTableArgs(opts.NFTTableName)...)
	if err := (hosts.Manager{Path: opts.HostsFile}).Disable(); err != nil {
		return err
	}
	return nil
}

func ResolveDomains(domains []string, resolver DNSResolver, logger log.Logger) Result {
	seen4 := map[string]bool{}
	seen6 := map[string]bool{}
	var result Result
	for _, domain := range domains {
		resolved, err := resolver.Resolve(domain)
		if err != nil {
			logger.Warn("lock DNS resolve failed: %s: %v", domain, err)
			continue
		}
		for _, ip := range resolved.IPv4 {
			if !seen4[ip] {
				seen4[ip] = true
				result.IPv4 = append(result.IPv4, ip)
			}
		}
		for _, ip := range resolved.IPv6 {
			if !seen6[ip] {
				seen6[ip] = true
				result.IPv6 = append(result.IPv6, ip)
			}
		}
	}
	return result
}

func ApplyNFT(tableName string, result Result, runner run.Runner) error {
	if tableName == "" {
		return fmt.Errorf("nft table name is empty")
	}
	script := nft.BuildScript(nft.Spec{TableName: tableName, IPv4: result.IPv4, IPv6: result.IPv6})
	tmp, err := os.CreateTemp("", "vpn-killswitch-nft-*.nft")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_, _, _ = runner.Run("nft", nft.DeleteTableArgs(tableName)...)
	_, _, err = runner.Run("nft", "-f", tmpPath)
	return err
}
