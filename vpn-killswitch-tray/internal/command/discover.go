package command

import (
	"os"
	"os/exec"
)

const DefaultCorePath = "/usr/local/sbin/vpn-killswitch"

func Discover(configured string) (string, error) {
	if configured != "" {
		if isExecutable(configured) {
			return configured, nil
		}
		if path, err := exec.LookPath(configured); err == nil {
			return path, nil
		}
	}
	if isExecutable(DefaultCorePath) {
		return DefaultCorePath, nil
	}
	return exec.LookPath("vpn-killswitch")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}
