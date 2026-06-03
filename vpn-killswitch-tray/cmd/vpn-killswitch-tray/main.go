package main

import (
	"flag"
	"fmt"
	"os"

	"vpn-killswitch-tray/internal/app"
	"vpn-killswitch-tray/internal/settings"
)

func main() {
	var configPath string
	var version bool
	flag.StringVar(&configPath, "config", "", "path to tray config.toml")
	flag.BoolVar(&version, "version", false, "print version")
	flag.Parse()

	if version {
		fmt.Println("0.1.0")
		return
	}
	cfg, err := settings.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vpn-killswitch-tray: %v\n", err)
		os.Exit(1)
	}
	app.Run(cfg)
}
