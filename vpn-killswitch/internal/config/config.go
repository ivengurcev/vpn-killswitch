package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"vpn-killswitch/internal/ipaddr"
)

const (
	DefaultPath = "/etc/vpn-killswitch/config.toml"

	DefaultBypassStatePath = "/run/vpn-killswitch/bypass-routes.v4"
	DefaultStateDir        = "/var/lib/vpn-killswitch"
	DefaultRunDir          = "/run/vpn-killswitch"
	DefaultHostsFile       = "/etc/hosts"
	DefaultNFTTableName    = "vpn_killswitch_lock"
	DefaultLogFile         = "/var/log/vpn-killswitch.log"
	DefaultLockFile        = "/run/vpn-killswitch/lock"
)

type Config struct {
	VPN        VPNConfig        `toml:"vpn" json:"vpn"`
	Bypass     BypassConfig     `toml:"bypass" json:"bypass"`
	Killswitch KillswitchConfig `toml:"killswitch" json:"killswitch"`
}

type VPNConfig struct {
	ConnectionName  string   `toml:"connection_name" json:"connection_name"`
	ConnectionTypes []string `toml:"connection_types" json:"connection_types"`
}

type BypassConfig struct {
	Domains   []string `toml:"domains" json:"domains"`
	IPs       []string `toml:"ips" json:"ips"`
	StatePath string   `toml:"state_path" json:"state_path"`
}

type KillswitchConfig struct {
	Domains      []string `toml:"domains" json:"domains"`
	ManageHosts  bool     `toml:"manage_hosts" json:"manage_hosts"`
	HostsFile    string   `toml:"hosts_file" json:"hosts_file"`
	NFTTableName string   `toml:"nft_table_name" json:"nft_table_name"`
	StateDir     string   `toml:"state_dir" json:"state_dir"`
	RunDir       string   `toml:"run_dir" json:"run_dir"`
}

func Defaults() Config {
	return Config{
		VPN: VPNConfig{
			ConnectionTypes: []string{"vpn", "wireguard"},
		},
		Bypass: BypassConfig{
			StatePath: DefaultBypassStatePath,
		},
		Killswitch: KillswitchConfig{
			ManageHosts:  true,
			HostsFile:    DefaultHostsFile,
			NFTTableName: DefaultNFTTableName,
			StateDir:     DefaultStateDir,
			RunDir:       DefaultRunDir,
		},
	}
}

func Read(path string) (Config, error) {
	cfg := Defaults()
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Decode(data string) (Config, error) {
	cfg := Defaults()
	if _, err := toml.Decode(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) Normalize() {
	c.VPN.ConnectionName = strings.TrimSpace(c.VPN.ConnectionName)
	c.VPN.ConnectionTypes = normalizeWords(c.VPN.ConnectionTypes)
	c.Bypass.Domains = NormalizeDomains(c.Bypass.Domains)
	if ips, err := ipaddr.NormalizeBypassEntries(c.Bypass.IPs); err == nil {
		c.Bypass.IPs = ips
	}
	c.Killswitch.Domains = NormalizeDomains(c.Killswitch.Domains)
	c.Bypass.StatePath = defaultString(c.Bypass.StatePath, DefaultBypassStatePath)
	c.Killswitch.HostsFile = defaultString(c.Killswitch.HostsFile, DefaultHostsFile)
	c.Killswitch.NFTTableName = defaultString(c.Killswitch.NFTTableName, DefaultNFTTableName)
	c.Killswitch.StateDir = defaultString(c.Killswitch.StateDir, DefaultStateDir)
	c.Killswitch.RunDir = defaultString(c.Killswitch.RunDir, DefaultRunDir)
}

func (c Config) Validate() error {
	var errs []string
	if c.VPN.ConnectionName == "" {
		errs = append(errs, "vpn.connection_name is required")
	}
	if len(c.VPN.ConnectionTypes) == 0 {
		errs = append(errs, "vpn.connection_types must not be empty")
	}
	if len(c.Bypass.Domains) == 0 && len(c.Bypass.IPs) == 0 && len(c.Killswitch.Domains) == 0 {
		errs = append(errs, "at least one of bypass.domains, bypass.ips or killswitch.domains must be non-empty")
	}
	if c.Bypass.StatePath == "" {
		errs = append(errs, "bypass.state_path is required")
	}
	if c.Killswitch.HostsFile == "" {
		errs = append(errs, "killswitch.hosts_file is required")
	}
	if c.Killswitch.StateDir == "" {
		errs = append(errs, "killswitch.state_dir is required")
	}
	if c.Killswitch.RunDir == "" {
		errs = append(errs, "killswitch.run_dir is required")
	}
	if !isNFTIdentifier(c.Killswitch.NFTTableName) {
		errs = append(errs, "killswitch.nft_table_name must be a simple nftables identifier")
	}
	if dup := firstOverlap(c.Bypass.Domains, c.Killswitch.Domains); dup != "" {
		errs = append(errs, fmt.Sprintf("domain %q is present in both bypass.domains and killswitch.domains", dup))
	}
	if bad := firstInvalidDomain(append([]string{}, append(c.Bypass.Domains, c.Killswitch.Domains...)...)); bad != "" {
		errs = append(errs, fmt.Sprintf("invalid domain: %s", bad))
	}
	if _, err := ipaddr.NormalizeBypassEntries(c.Bypass.IPs); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func EnsureExample(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(ExampleTOML), 0644)
}

func Write(path string, cfg Config) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

const ExampleTOML = `# vpn-killswitch example configuration.

[vpn]
connection_name = "VPN connection name"
connection_types = ["vpn", "wireguard"]

[bypass]
domains = ["ozon.ru", "wildberries.ru"]
ips = ["93.184.216.34", "93.184.216.0/24", "93.184.216.10-93.184.216.20"]
state_path = "/run/vpn-killswitch/bypass-routes.v4"

[killswitch]
domains = ["example.com", "api.example.com"]
manage_hosts = true
hosts_file = "/etc/hosts"
nft_table_name = "vpn_killswitch_lock"
state_dir = "/var/lib/vpn-killswitch"
run_dir = "/run/vpn-killswitch"
`

var (
	nftIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	domainRe   = regexp.MustCompile(`^[a-z0-9]([a-z0-9_-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9_-]*[a-z0-9])?)*$`)
)

func NormalizeDomains(domains []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range domains {
		for _, token := range splitDomainTokens(raw) {
			domain := strings.Trim(strings.ToLower(strings.TrimSpace(token)), ".")
			if domain == "" || seen[domain] {
				continue
			}
			seen[domain] = true
			out = append(out, domain)
		}
	}
	sort.Strings(out)
	return out
}

func splitDomainTokens(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	return strings.Fields(raw)
}

func normalizeWords(words []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" || seen[word] {
			continue
		}
		seen[word] = true
		out = append(out, word)
	}
	return out
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func isNFTIdentifier(value string) bool {
	return nftIdentRe.MatchString(value)
}

func firstOverlap(a, b []string) string {
	seen := map[string]bool{}
	for _, item := range a {
		seen[item] = true
	}
	for _, item := range b {
		if seen[item] {
			return item
		}
	}
	return ""
}

func firstInvalidDomain(domains []string) string {
	for _, domain := range domains {
		if !domainRe.MatchString(domain) || strings.Contains(domain, "..") {
			return domain
		}
	}
	return ""
}
