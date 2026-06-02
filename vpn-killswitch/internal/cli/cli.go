package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vpn-killswitch/internal/bypass"
	"vpn-killswitch/internal/config"
	"vpn-killswitch/internal/configedit"
	"vpn-killswitch/internal/diagnostics"
	"vpn-killswitch/internal/enforce"
	"vpn-killswitch/internal/lock"
	"vpn-killswitch/internal/lockdns"
	"vpn-killswitch/internal/lockfile"
	"vpn-killswitch/internal/log"
	"vpn-killswitch/internal/resolver"
	"vpn-killswitch/internal/run"
	"vpn-killswitch/internal/system"
)

const Version = "0.2.0"

const (
	ExitOK          = 0
	ExitGeneral     = 1
	ExitRoot        = 2
	ExitGateway     = 3
	ExitConfig      = 4
	ExitApply       = 5
	ExitLockAcquire = 6
	ExitInstall     = 7
	ExitUninstall   = 8
	ExitBadArgs     = 9
	ExitDiagnostics = 10
)

type options struct {
	configPath string
	dryRun     bool
	json       bool
	verbose    bool
	quiet      bool
	noCopy     bool
	noEnable   bool
	noStart    bool
	purge      bool
	destDir    string
}

func Run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return ExitBadArgs
	}
	cmd, subcmd, reason, opts, err := parse(args)
	logger := log.New(opts.verbose, opts.quiet)
	if err != nil {
		logger.Error("%v", err)
		return ExitBadArgs
	}
	if cmd == "help" {
		usage(os.Stdout)
		return ExitOK
	}
	if cmd == "version" {
		fmt.Println(Version)
		return ExitOK
	}
	if requiresRoot(cmd, subcmd) && !opts.dryRun && opts.destDir == "" && !editsLocalConfig(cmd, opts) && os.Geteuid() != 0 {
		logger.Error("%s requires root", commandName(cmd, subcmd))
		return ExitRoot
	}

	if cmd == "install" {
		err := system.Install(system.InstallOptions{DryRun: opts.dryRun, NoCopy: opts.noCopy, NoEnable: opts.noEnable, NoStart: opts.noStart, DestDir: opts.destDir}, run.ExecRunner{}, logger.Info)
		if err != nil {
			logger.Error("install: %v", err)
			return ExitInstall
		}
		return ExitOK
	}
	if cmd == "uninstall" {
		err := system.Uninstall(system.UninstallOptions{DryRun: opts.dryRun, Purge: opts.purge, DestDir: opts.destDir}, run.ExecRunner{}, logger.Info)
		if err != nil {
			logger.Error("uninstall: %v", err)
			return ExitUninstall
		}
		return ExitOK
	}

	cfg, err := config.Read(opts.configPath)
	if err != nil {
		logger.Error("read config: %v", err)
		return ExitConfig
	}
	if cmd == "config" {
		return runConfigEdit(subcmd, reason, opts, logger)
	}
	runner := run.ExecRunner{}
	bypassResolver := resolver.Getent{Runner: runner}
	lockResolver := lockdns.Resolver{Runner: runner}

	release, code := acquireIfNeeded(cfg, cmd, subcmd, opts.dryRun, logger)
	if code != ExitOK {
		return code
	}
	if release != nil {
		defer release()
	}

	switch cmd {
	case "enforce":
		result, err := enforce.Run(enforce.Options{Config: cfg, DryRun: opts.dryRun, Reason: reason}, runner, bypassResolver, lockResolver, logger)
		if opts.json {
			printJSON(result)
		}
		return codeForApplyError(err)
	case "bypass":
		if subcmd != "apply" {
			logger.Error("unknown bypass command: %s", subcmd)
			return ExitBadArgs
		}
		result, err := bypass.Apply(bypass.Options{Domains: cfg.Bypass.Domains, StatePath: cfg.Bypass.StatePath, DryRun: opts.dryRun}, runner, bypassResolver, logger)
		if opts.json {
			printJSON(result)
		}
		return codeForApplyError(err)
	case "lock":
		return runLock(subcmd, reason, opts, cfg, runner, lockResolver, logger)
	case "status":
		status := diagnostics.CollectStatus(cfg, opts.configPath, runner)
		if opts.json {
			printJSON(status)
		} else {
			printStatus(status)
		}
		return ExitOK
	case "resolve":
		result := diagnostics.Resolve(cfg, bypassResolver, lockResolver)
		if opts.json {
			printJSON(result)
		} else {
			printResolve(result)
		}
		return ExitOK
	case "test":
		result := diagnostics.RunTests(cfg, opts.configPath, runner)
		if opts.json {
			printJSON(result)
		} else {
			printTests(result)
		}
		if !result.OK {
			return ExitDiagnostics
		}
		return ExitOK
	case "install-check":
		result := diagnostics.RunInstallCheck(cfg, opts.configPath, runner)
		if opts.json {
			printJSON(result)
		} else {
			printTests(result)
		}
		if !result.OK {
			return ExitDiagnostics
		}
		return ExitOK
	default:
		logger.Error("unknown command: %s", cmd)
		return ExitBadArgs
	}
}

func runConfigEdit(subcmd, domain string, opts options, logger log.Logger) int {
	if domain == "" {
		logger.Error("config %s requires DOMAIN", subcmd)
		return ExitBadArgs
	}
	var list configedit.List
	var add bool
	switch subcmd {
	case "add-bypass":
		list, add = configedit.Bypass, true
	case "remove-bypass":
		list, add = configedit.Bypass, false
	case "add-lock":
		list, add = configedit.Lock, true
	case "remove-lock":
		list, add = configedit.Lock, false
	default:
		logger.Error("unknown config command: %s", subcmd)
		return ExitBadArgs
	}
	if opts.dryRun {
		action := "remove"
		if add {
			action = "add"
		}
		logger.Info("dry-run config %s %s: %s", action, list, domain)
		return ExitOK
	}
	var result configedit.Result
	var err error
	if add {
		result, err = configedit.Add(opts.configPath, list, domain)
	} else {
		result, err = configedit.Remove(opts.configPath, list, domain)
	}
	if err != nil {
		logger.Error("config %s: %v", subcmd, err)
		return ExitConfig
	}
	if opts.json {
		printJSON(result)
	} else if result.Changed {
		logger.Info("config updated: %s", result.Domain)
	} else {
		logger.Info("config unchanged: %s", result.Domain)
	}
	return ExitOK
}

func runLock(subcmd, reason string, opts options, cfg config.Config, runner run.Runner, lockResolver lock.DNSResolver, logger log.Logger) int {
	switch subcmd {
	case "enable":
		result, err := lock.Enable(lock.Options{
			Domains:      cfg.Killswitch.Domains,
			NFTTableName: cfg.Killswitch.NFTTableName,
			HostsFile:    cfg.Killswitch.HostsFile,
			ManageHosts:  cfg.Killswitch.ManageHosts,
			DryRun:       opts.dryRun,
		}, runner, lockResolver, logger)
		if opts.json {
			printJSON(result)
		}
		return codeForApplyError(err)
	case "disable":
		err := lock.Disable(lock.Options{NFTTableName: cfg.Killswitch.NFTTableName, HostsFile: cfg.Killswitch.HostsFile, DryRun: opts.dryRun}, runner, logger)
		return codeForApplyError(err)
	case "refresh":
		result, err := enforce.RefreshLock(enforce.Options{Config: cfg, DryRun: opts.dryRun, Reason: reason}, runner, lockResolver, logger)
		if opts.json {
			printJSON(result)
		}
		return codeForApplyError(err)
	default:
		logger.Error("unknown lock command: %s", subcmd)
		return ExitBadArgs
	}
}

func parse(args []string) (cmd string, subcmd string, reason string, opts options, err error) {
	cmd = args[0]
	rest := args[1:]
	if cmd == "bypass" || cmd == "lock" || cmd == "config" {
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
			return cmd, "", "", opts, fmt.Errorf("%s requires a subcommand", cmd)
		}
		subcmd = rest[0]
		rest = rest[1:]
	}
	flagArgs, positional, err := splitArgs(rest)
	if err != nil {
		return cmd, subcmd, "", opts, err
	}
	fs := flag.NewFlagSet(commandName(cmd, subcmd), flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts.configPath = config.DefaultPath
	fs.StringVar(&opts.configPath, "config", opts.configPath, "path to config.toml")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "show planned changes")
	fs.BoolVar(&opts.json, "json", false, "print JSON")
	fs.BoolVar(&opts.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&opts.quiet, "quiet", false, "quiet output")
	fs.BoolVar(&opts.noCopy, "no-copy", false, "do not copy the current binary during install")
	fs.BoolVar(&opts.noEnable, "no-enable", false, "do not enable systemd units during install")
	fs.BoolVar(&opts.noStart, "no-start", false, "do not start initial service during install")
	fs.BoolVar(&opts.purge, "purge", false, "remove config/state/log during uninstall")
	fs.StringVar(&opts.destDir, "destdir", "", "stage install/uninstall under DIR")
	if err := fs.Parse(flagArgs); err != nil {
		return cmd, subcmd, "", opts, err
	}
	positional = append(positional, fs.Args()...)
	switch cmd {
	case "enforce":
		if len(positional) > 1 {
			return cmd, subcmd, "", opts, fmt.Errorf("enforce accepts at most one reason")
		}
	case "lock":
		if subcmd == "enable" || subcmd == "disable" || subcmd == "refresh" {
			if len(positional) > 1 {
				return cmd, subcmd, "", opts, fmt.Errorf("lock %s accepts at most one reason", subcmd)
			}
		} else if len(positional) > 0 {
			return cmd, subcmd, "", opts, fmt.Errorf("unexpected argument: %s", positional[0])
		}
	case "config":
		if len(positional) != 1 {
			return cmd, subcmd, "", opts, fmt.Errorf("config %s requires exactly one domain", subcmd)
		}
	default:
		if len(positional) > 0 {
			return cmd, subcmd, "", opts, fmt.Errorf("%s does not accept positional arguments", commandName(cmd, subcmd))
		}
	}
	if len(positional) == 1 {
		reason = positional[0]
	}
	return cmd, subcmd, reason, opts, nil
}

func splitArgs(args []string) ([]string, []string, error) {
	stringFlags := map[string]bool{"--config": true, "-config": true, "--destdir": true, "-destdir": true}
	boolFlags := map[string]bool{
		"--dry-run": true, "-dry-run": true, "--json": true, "-json": true, "--verbose": true, "-verbose": true, "--quiet": true, "-quiet": true,
		"--no-copy": true, "-no-copy": true, "--no-enable": true, "-no-enable": true, "--no-start": true, "-no-start": true, "--purge": true, "-purge": true,
	}
	var flagArgs, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" || !strings.HasPrefix(arg, "-") {
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
		if strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func requiresRoot(cmd, subcmd string) bool {
	switch cmd {
	case "enforce":
		return true
	case "install", "uninstall":
		return true
	case "config":
		return true
	case "bypass":
		return subcmd == "apply"
	case "lock":
		return subcmd == "enable" || subcmd == "disable" || subcmd == "refresh"
	default:
		return false
	}
}

func editsLocalConfig(cmd string, opts options) bool {
	return cmd == "config" && opts.configPath != config.DefaultPath
}

func acquireIfNeeded(cfg config.Config, cmd, subcmd string, dryRun bool, logger log.Logger) (func(), int) {
	if dryRun || !requiresRoot(cmd, subcmd) {
		return nil, ExitOK
	}
	path := filepath.Join(cfg.Killswitch.RunDir, "lock")
	l, err := lockfile.Acquire(path, 10*time.Second)
	if err != nil {
		logger.Error("acquire lock: %v", err)
		return nil, ExitLockAcquire
	}
	return func() { _ = l.Close() }, ExitOK
}

func codeForApplyError(err error) int {
	if err == nil {
		return ExitOK
	}
	if strings.Contains(err.Error(), "gateway") {
		return ExitGateway
	}
	return ExitApply
}

func commandName(cmd, subcmd string) string {
	if subcmd == "" {
		return cmd
	}
	return cmd + " " + subcmd
}

func printJSON(value any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

func printStatus(status diagnostics.Status) {
	fmt.Printf("VPN: %s (%s)\n", status.VPN.Name, status.VPN.Status)
	if status.Gateway.Error != "" {
		fmt.Printf("Gateway: error: %s\n", status.Gateway.Error)
	} else {
		fmt.Printf("Gateway: %s dev %s\n", status.Gateway.Via, status.Gateway.Dev)
	}
	fmt.Printf("Bypass: %d domains, %d routes\n", status.Bypass.DomainCount, status.Bypass.RouteCount)
	fmt.Printf("Lock: %d domains, nft=%t, hosts=%t\n", status.Lock.DomainCount, status.Lock.NFTActive, status.Lock.HostsBlock)
	fmt.Printf("Config: %s\n", status.Config.Path)
	if len(status.Errors) > 0 {
		fmt.Println("Errors:")
		for _, err := range status.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}
}

func printResolve(result diagnostics.ResolveResult) {
	fmt.Println("Bypass:")
	for _, item := range result.Bypass {
		fmt.Printf("  %s IPv4=%s", item.Domain, strings.Join(item.IPv4, ","))
		if item.Error != "" {
			fmt.Printf(" error=%s", item.Error)
		}
		fmt.Println()
	}
	fmt.Println("Lock:")
	for _, item := range result.Lock {
		fmt.Printf("  %s IPv4=%s IPv6=%s", item.Domain, strings.Join(item.IPv4, ","), strings.Join(item.IPv6, ","))
		if item.Error != "" {
			fmt.Printf(" error=%s", item.Error)
		}
		fmt.Println()
	}
}

func printTests(result diagnostics.TestResult) {
	for _, check := range result.Checks {
		fmt.Printf("%s %s", strings.ToUpper(check.Status), check.Name)
		if check.Message != "" {
			fmt.Printf(": %s", check.Message)
		}
		fmt.Println()
	}
	if result.OK {
		fmt.Println("Summary: OK")
	} else {
		fmt.Println("Summary: FAIL")
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `Usage: vpn-killswitch <command> [flags]

Commands:
  enforce [reason]
  bypass apply
  lock enable [reason]
  lock disable [reason]
  lock refresh [reason]
  status
  resolve
  test
  install-check
  install
  uninstall
  config add-bypass DOMAIN
  config remove-bypass DOMAIN
  config add-lock DOMAIN
  config remove-lock DOMAIN
  version
  help

Flags:
  --config PATH
  --dry-run
  --json
  --verbose
  --quiet

Install/uninstall flags:
  --no-copy
  --no-enable
  --no-start
  --purge
  --destdir DIR`)
}
