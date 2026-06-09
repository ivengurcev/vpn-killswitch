package bypass

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Route struct {
	Target     string `json:"target"`
	IP         string `json:"ip,omitempty"`
	Gateway    string `json:"gateway"`
	Dev        string `json:"dev"`
	Domain     string `json:"domain,omitempty"`
	SourceType string `json:"source_type"`
	Source     string `json:"source"`
}

func ReadState(path string) ([]Route, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var routes []Route
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		if len(fields) >= 5 {
			routes = append(routes, Route{
				Target:     fields[0],
				IP:         targetIP(fields[0]),
				Gateway:    fields[1],
				Dev:        fields[2],
				SourceType: fields[3],
				Source:     fields[4],
				Domain:     domainForSource(fields[3], fields[4]),
			})
			continue
		}
		routes = append(routes, Route{
			Target:     legacyTarget(fields[0]),
			IP:         fields[0],
			Gateway:    fields[1],
			Dev:        fields[2],
			Domain:     fields[3],
			SourceType: "domain",
			Source:     fields[3],
		})
	}
	return routes, scanner.Err()
}

func WriteState(path string, routes []Route) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	sortRoutes(routes)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bypass-routes-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	for _, route := range routes {
		if _, err := fmt.Fprintf(tmp, "%s %s %s %s %s\n", routeTarget(route), route.Gateway, route.Dev, routeSourceType(route), routeSource(route)); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return err
		}
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

func sortRoutes(routes []Route) {
	sort.Slice(routes, func(i, j int) bool {
		if routeSource(routes[i]) == routeSource(routes[j]) {
			return routeTarget(routes[i]) < routeTarget(routes[j])
		}
		return routeSource(routes[i]) < routeSource(routes[j])
	})
}

func routeTarget(route Route) string {
	if route.Target != "" {
		return route.Target
	}
	return legacyTarget(route.IP)
}

func routeSourceType(route Route) string {
	if route.SourceType != "" {
		return route.SourceType
	}
	return "domain"
}

func routeSource(route Route) string {
	if route.Source != "" {
		return route.Source
	}
	return route.Domain
}

func legacyTarget(ip string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	return ip + "/32"
}

func targetIP(target string) string {
	if i := strings.Index(target, "/"); i >= 0 {
		return target[:i]
	}
	return target
}

func domainForSource(sourceType, source string) string {
	if sourceType == "domain" {
		return source
	}
	return ""
}
