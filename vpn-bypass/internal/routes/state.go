package routes

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultStatePath = "/run/vpn-bypass/routes.v4"

type StateRoute struct {
	IP      string `json:"ip"`
	Gateway string `json:"gateway"`
	Dev     string `json:"dev"`
	Domain  string `json:"domain"`
}

func ReadState(path string) ([]StateRoute, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var routes []StateRoute
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		routes = append(routes, StateRoute{
			IP: fields[0], Gateway: fields[1], Dev: fields[2], Domain: fields[3],
		})
	}
	return routes, scanner.Err()
}

func WriteState(path string, routes []StateRoute) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Domain == routes[j].Domain {
			return routes[i].IP < routes[j].IP
		}
		return routes[i].Domain < routes[j].Domain
	})
	tmp, err := os.CreateTemp(filepath.Dir(path), ".routes.v4-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	for _, route := range routes {
		if _, err := fmt.Fprintf(tmp, "%s %s %s %s\n", route.IP, route.Gateway, route.Dev, route.Domain); err != nil {
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
