package main

import (
	"os"

	"vpn-killswitch/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
