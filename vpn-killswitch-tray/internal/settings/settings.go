package settings

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultCommandPath         = "/usr/local/sbin/vpn-killswitch"
	DefaultConfigPath          = "/etc/vpn-killswitch/config.toml"
	DefaultPollIntervalSeconds = 5
	DefaultLogFile             = "~/.local/state/vpn-killswitch-tray/tray.log"
)

type Settings struct {
	CommandPath                  string `toml:"command_path"`
	ConfigPath                   string `toml:"config_path"`
	PollIntervalSeconds          int    `toml:"poll_interval_seconds"`
	AutoEnforceAfterConfigChange bool   `toml:"auto_enforce_after_config_change"`
	LogFile                      string `toml:"log_file"`
}

func Defaults() Settings {
	return Settings{
		CommandPath:                  DefaultCommandPath,
		ConfigPath:                   DefaultConfigPath,
		PollIntervalSeconds:          DefaultPollIntervalSeconds,
		AutoEnforceAfterConfigChange: true,
		LogFile:                      DefaultLogFile,
	}
}

func Load(path string) (Settings, error) {
	cfg := Defaults()
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg.Normalize()
		return cfg, nil
	} else if err != nil {
		return cfg, err
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	cfg.Normalize()
	return cfg, nil
}

func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "vpn-killswitch-tray", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "vpn-killswitch-tray", "config.toml")
	}
	return filepath.Join(home, ".config", "vpn-killswitch-tray", "config.toml")
}

func (s *Settings) Normalize() {
	s.CommandPath = defaultString(s.CommandPath, DefaultCommandPath)
	s.ConfigPath = defaultString(s.ConfigPath, DefaultConfigPath)
	if s.PollIntervalSeconds <= 0 {
		s.PollIntervalSeconds = DefaultPollIntervalSeconds
	}
	s.LogFile = expandHome(defaultString(s.LogFile, DefaultLogFile))
}

func expandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
