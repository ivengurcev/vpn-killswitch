package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"vpn-bypass/internal/config"
	"vpn-bypass/internal/lockfile"
	"vpn-bypass/internal/log"
	"vpn-bypass/internal/networkmanager"
	"vpn-bypass/internal/resolver"
	"vpn-bypass/internal/routes"
	"vpn-bypass/internal/run"
	"vpn-bypass/internal/systemd"
)

const Version = "0.1.0"

const (
	ExitOK        = 0
	ExitGeneral   = 1
	ExitRoot      = 2
	ExitNoGateway = 3
	ExitConfig    = 4
	ExitApply     = 5
	ExitLock      = 6
	ExitInstall   = 7
	ExitUninstall = 8
	ExitBadArgs   = 9
)

type options struct {
	configPath string
	statePath  string
	dryRun     bool
	json       bool
	verbose    bool
	quiet      bool
	noCopy     bool
	purge      bool
}

func Run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return ExitBadArgs
	}

	cmd, opts, rest, err := parse(args)
	logger := log.New(opts.verbose, opts.quiet)
	if err != nil {
		logger.Error("%v", err)
		return ExitBadArgs
	}
	if err := validate(cmd, opts, rest); err != nil {
		logger.Error("%v", err)
		return ExitBadArgs
	}
	if requiresRoot(cmd) && !opts.dryRun && os.Geteuid() != 0 {
		logger.Error("%s requires root", cmd)
		return ExitRoot
	}

	runner := run.ExecRunner{}
	res := resolver.Resolver{Runner: runner}

	if requiresLock(cmd) && !opts.dryRun {
		l, err := lockfile.Acquire(lockfile.DefaultPath, 30*time.Second)
		if err != nil {
			logger.Error("another vpn-bypass instance is running")
			return ExitLock
		}
		defer l.Close()
	}

	switch cmd {
	case "apply":
		return runApply(opts, runner, res, logger)
	case "check":
		return runCheck(rest[0], opts, runner, res)
	case "list":
		return runList(opts)
	case "status":
		return runStatus(opts, runner)
	case "add":
		return runAdd(rest[0], opts)
	case "remove":
		return runRemove(rest[0], opts)
	case "install":
		return runInstall(opts, runner)
	case "uninstall":
		return runUninstall(opts, runner)
	case "version":
		fmt.Println(Version)
		return ExitOK
	default:
		logger.Error("unknown command: %s", cmd)
		return ExitBadArgs
	}
}

func parse(args []string) (string, options, []string, error) {
	cmd := args[0]
	flagArgs, positional, err := splitArgs(args[1:])
	if err != nil {
		return cmd, options{}, nil, err
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := options{configPath: config.DefaultPath, statePath: routes.DefaultStatePath}
	fs.StringVar(&opts.configPath, "config", opts.configPath, "path to domains config")
	fs.StringVar(&opts.statePath, "state", opts.statePath, "path to state file")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "show planned changes")
	fs.BoolVar(&opts.json, "json", false, "print JSON")
	fs.BoolVar(&opts.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&opts.quiet, "quiet", false, "quiet output")
	fs.BoolVar(&opts.noCopy, "no-copy", false, "do not copy binary during install")
	fs.BoolVar(&opts.purge, "purge", false, "remove config during uninstall")
	if err := fs.Parse(flagArgs); err != nil {
		return cmd, opts, nil, err
	}
	return cmd, opts, positional, nil
}

func splitArgs(args []string) ([]string, []string, error) {
	stringFlags := map[string]bool{"--config": true, "-config": true, "--state": true, "-state": true}
	boolFlags := map[string]bool{
		"--dry-run": true, "-dry-run": true, "--json": true, "-json": true, "--verbose": true, "-verbose": true,
		"--quiet": true, "-quiet": true, "--no-copy": true, "-no-copy": true, "--purge": true, "-purge": true,
	}
	var flagArgs, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) == 0 || arg[0] != '-' {
			positional = append(positional, arg)
			continue
		}
		if hasInlineValue(arg, stringFlags) || boolFlags[arg] {
			flagArgs = append(flagArgs, arg)
			continue
		}
		if stringFlags[arg] {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s requires a value", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
			continue
		}
		return nil, nil, fmt.Errorf("unknown flag: %s", arg)
	}
	return flagArgs, positional, nil
}

func hasInlineValue(arg string, stringFlags map[string]bool) bool {
	for name := range stringFlags {
		if len(arg) > len(name)+1 && arg[:len(name)+1] == name+"=" {
			return true
		}
	}
	return false
}

func validate(cmd string, opts options, rest []string) error {
	readOnly := map[string]bool{"status": true, "check": true, "list": true, "version": true}
	if readOnly[cmd] && opts.dryRun {
		return fmt.Errorf("--dry-run is not supported for %s", cmd)
	}
	switch cmd {
	case "apply", "status", "list", "install", "uninstall", "version":
		if len(rest) != 0 {
			return fmt.Errorf("%s does not accept positional arguments", cmd)
		}
	case "check", "add", "remove":
		if len(rest) != 1 {
			return fmt.Errorf("%s requires exactly one domain", cmd)
		}
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
	if opts.noCopy && cmd != "install" {
		return fmt.Errorf("--no-copy is only supported for install")
	}
	if opts.purge && cmd != "uninstall" {
		return fmt.Errorf("--purge is only supported for uninstall")
	}
	return nil
}

func requiresRoot(cmd string) bool {
	switch cmd {
	case "apply", "install", "uninstall", "add", "remove":
		return true
	default:
		return false
	}
}

func requiresLock(cmd string) bool {
	switch cmd {
	case "apply", "uninstall", "add", "remove":
		return true
	default:
		return false
	}
}

func runApply(opts options, runner run.Runner, res resolver.Resolver, logger log.Logger) int {
	domains, err := config.ReadDomains(opts.configPath)
	if err != nil {
		logger.Error("read config: %v", err)
		return ExitConfig
	}
	result, err := routes.Apply(routes.ApplyOptions{ConfigPath: opts.configPath, StatePath: opts.statePath, DryRun: opts.dryRun}, domains, runner, res, logger)
	if opts.json {
		printJSON(result)
	}
	if err != nil {
		if err.Error() == "normal gateway not found" {
			logger.Error("%v", err)
			return ExitNoGateway
		}
		if result.Failures > 0 {
			return ExitApply
		}
		logger.Error("%v", err)
		return ExitGeneral
	}
	return ExitOK
}

func runCheck(domain string, opts options, runner run.Runner, res resolver.Resolver) int {
	result := routes.Check(domain, runner, res)
	if opts.json {
		printJSON(result)
		return ExitOK
	}
	fmt.Printf("Domain: %s\n\n", result.Domain)
	for _, item := range result.IPs {
		fmt.Printf("%s\n", item.IP)
		if item.Error != "" {
			fmt.Printf("    Error: %s\n", item.Error)
		}
		fmt.Printf("    Route: %s\n", routes.FormatRoute(item.Route))
		fmt.Printf("    Status: %s\n\n", item.Status)
	}
	fmt.Printf("Summary: %s\n", result.Summary)
	return ExitOK
}

func runList(opts options) int {
	domains, err := config.ReadDomains(opts.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR read config: %v\n", err)
		return ExitConfig
	}
	if opts.json {
		printJSON(domains)
		return ExitOK
	}
	for _, domain := range domains {
		fmt.Println(domain)
	}
	return ExitOK
}

func runStatus(opts options, runner run.Runner) int {
	domains, err := config.ReadDomains(opts.configPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ERROR read config: %v\n", err)
		return ExitConfig
	}
	status := routes.BuildStatus(opts.configPath, opts.statePath, domains, runner)
	if opts.json {
		printJSON(status)
		return ExitOK
	}
	fmt.Printf("Config: %s\n", status.ConfigPath)
	fmt.Printf("State: %s\n\n", status.StatePath)
	fmt.Printf("Domains in config: %d\n", status.DomainsInConfig)
	fmt.Printf("Routes in state: %d\n", status.RoutesInState)
	fmt.Printf("Active bypass routes: %d\n\n", status.ActiveRoutes)
	if status.Gateway != nil {
		fmt.Printf("Normal gateway: %s dev %s\n\n", status.Gateway.Via, status.Gateway.Dev)
	} else {
		fmt.Printf("Normal gateway: not found\n\n")
	}
	fmt.Printf("Systemd service: %s\n", status.Service)
	fmt.Printf("Systemd timer: %s\n", status.Timer)
	fmt.Printf("NetworkManager hook: %s\n", status.Hook)
	return ExitOK
}

func runAdd(domain string, opts options) int {
	changed, err := config.AddDomain(opts.configPath, domain, opts.dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR add domain: %v\n", err)
		return ExitGeneral
	}
	if opts.dryRun {
		fmt.Printf("DRY-RUN domain would be added: %s\n", domain)
		return ExitOK
	}
	if changed {
		fmt.Printf("Domain added: %s\n\n", domain)
	} else {
		fmt.Printf("Domain already exists: %s\n\n", domain)
	}
	printApplyHint()
	return ExitOK
}

func runRemove(domain string, opts options) int {
	changed, err := config.RemoveDomain(opts.configPath, domain, opts.dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR remove domain: %v\n", err)
		return ExitGeneral
	}
	if opts.dryRun {
		fmt.Printf("DRY-RUN domain would be removed: %s\n", domain)
		return ExitOK
	}
	if changed {
		fmt.Printf("Domain removed: %s\n\n", domain)
	} else {
		fmt.Printf("Domain not found: %s\n\n", domain)
	}
	printApplyHint()
	return ExitOK
}

func runInstall(opts options, runner run.Runner) int {
	printf := func(format string, args ...any) { fmt.Printf(format, args...) }
	if !opts.dryRun {
		l, err := lockfile.Acquire(lockfile.DefaultPath, 30*time.Second)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR another vpn-bypass instance is running")
			return ExitLock
		}
		if code := installFiles(opts, runner, printf); code != ExitOK {
			_ = l.Close()
			return code
		}
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR release lock: %v\n", err)
			return ExitInstall
		}
	} else if code := installFiles(opts, runner, printf); code != ExitOK {
		return code
	}
	if err := systemd.Activate(opts.dryRun, runner, printf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR activate systemd units: %v\n", err)
		return ExitInstall
	}
	return ExitOK
}

func installFiles(opts options, runner run.Runner, printf func(string, ...any)) int {
	if err := systemd.Install(systemd.InstallOptions{ConfigPath: opts.configPath, DryRun: opts.dryRun, NoCopy: opts.noCopy}, runner, printf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR install: %v\n", err)
		return ExitInstall
	}
	if err := networkmanager.Install(opts.dryRun, printf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR install NetworkManager hook: %v\n", err)
		return ExitInstall
	}
	return ExitOK
}

func runUninstall(opts options, runner run.Runner) int {
	printf := func(format string, args ...any) { fmt.Printf(format, args...) }
	if err := systemd.Uninstall(opts.dryRun, opts.purge, runner, printf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR uninstall: %v\n", err)
		return ExitUninstall
	}
	if err := networkmanager.Uninstall(opts.dryRun, printf); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR uninstall NetworkManager hook: %v\n", err)
		return ExitUninstall
	}
	return ExitOK
}

func printApplyHint() {
	fmt.Println("Routes are not applied automatically.")
	fmt.Println("Run:")
	fmt.Println()
	fmt.Println("    sudo vpn-bypass apply")
	fmt.Println()
	fmt.Println("Changes will also be applied automatically by systemd timer.")
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func usage(w io.Writer) {
	cmds := []string{"apply", "check <domain>", "list", "status", "add <domain>", "remove <domain>", "install", "uninstall", "version"}
	sort.Strings(cmds)
	fmt.Fprintln(w, "Usage: vpn-bypass <command> [flags]")
	for _, cmd := range cmds {
		fmt.Fprintf(w, "  %s\n", cmd)
	}
}
