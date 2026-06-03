package main

import (
	"os"

	"vpn-bypass/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
