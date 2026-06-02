package vpn

import (
	"strings"

	"vpn-killswitch/internal/run"
)

type Status string

const (
	Active   Status = "active"
	Inactive Status = "inactive"
	Unknown  Status = "unknown"
)

func Detect(r run.Runner, connectionName string, allowedTypes []string) Status {
	out, _, err := r.Run("nmcli", "-t", "-f", "NAME,TYPE", "connection", "show", "--active")
	if err != nil {
		return Unknown
	}
	return ParseActiveConnections(out, connectionName, allowedTypes)
}

func ParseActiveConnections(out, connectionName string, allowedTypes []string) Status {
	allowed := map[string]bool{}
	for _, typ := range allowedTypes {
		allowed[strings.ToLower(strings.TrimSpace(typ))] = true
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, typ, ok := cutNMCLIField(line)
		if !ok {
			continue
		}
		name = strings.ReplaceAll(name, `\:`, ":")
		typ = strings.ToLower(strings.TrimSpace(typ))
		if name == connectionName && allowed[typ] {
			return Active
		}
	}
	return Inactive
}

func cutNMCLIField(line string) (string, string, bool) {
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ':' {
			return line[:i], line[i+1:], true
		}
	}
	return "", "", false
}
