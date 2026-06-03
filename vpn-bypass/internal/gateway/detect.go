package gateway

import (
	"fmt"
	"path/filepath"
	"strings"

	"vpn-bypass/internal/run"
)

type Gateway struct {
	Via string
	Dev string
}

var excludedPatterns = []string{
	"lo", "tun*", "tap*", "wg*", "wt*", "docker*", "br-*", "virbr*", "veth*",
}

func Detect(r run.Runner) (Gateway, error) {
	out, _, err := r.Run("ip", "-4", "route", "show", "default")
	if err != nil {
		return Gateway{}, err
	}
	return ParseDefaultRoutes(out)
}

func ParseDefaultRoutes(out string) (Gateway, error) {
	for _, line := range strings.Split(out, "\n") {
		gw, ok := parseDefaultRouteLine(line)
		if !ok {
			continue
		}
		if isExcluded(gw.Dev) {
			continue
		}
		return gw, nil
	}
	return Gateway{}, fmt.Errorf("normal gateway not found")
}

func parseDefaultRouteLine(line string) (Gateway, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] != "default" {
		return Gateway{}, false
	}
	var gw Gateway
	for i := 1; i < len(fields)-1; i++ {
		switch fields[i] {
		case "via":
			gw.Via = fields[i+1]
		case "dev":
			gw.Dev = fields[i+1]
		}
	}
	return gw, gw.Via != "" && gw.Dev != ""
}

func isExcluded(dev string) bool {
	for _, pattern := range excludedPatterns {
		if ok, _ := filepath.Match(pattern, dev); ok {
			return true
		}
	}
	return false
}
