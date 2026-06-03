package networkmanager

import (
	"os"
)

const HookPath = "/etc/NetworkManager/dispatcher.d/90-vpn-bypass-routes"

const HookContent = `#!/bin/sh

ACTION="$2"

case "$ACTION" in
    vpn-up|vpn-down)
        /usr/bin/systemctl start vpn-bypass-routes.service
        ;;
esac

exit 0
`

func Install(dryRun bool, printf func(string, ...any)) error {
	if dryRun {
		printf("DRY-RUN write file: %s\n%s", HookPath, HookContent)
		printf("DRY-RUN chmod +x %s\n", HookPath)
		return nil
	}
	if err := os.WriteFile(HookPath, []byte(HookContent), 0755); err != nil {
		return err
	}
	return os.Chmod(HookPath, 0755)
}

func Uninstall(dryRun bool, printf func(string, ...any)) error {
	if dryRun {
		printf("DRY-RUN remove: %s\n", HookPath)
		return nil
	}
	if err := os.Remove(HookPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
