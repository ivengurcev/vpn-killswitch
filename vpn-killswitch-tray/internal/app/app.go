package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"

	"vpn-killswitch-tray/internal/command"
	"vpn-killswitch-tray/internal/core"
	"vpn-killswitch-tray/internal/domains"
	"vpn-killswitch-tray/internal/icons"
	"vpn-killswitch-tray/internal/run"
	"vpn-killswitch-tray/internal/settings"
	"vpn-killswitch-tray/internal/state"
	"vpn-killswitch-tray/internal/tools"
	"vpn-killswitch-tray/internal/traylog"
)

type App struct {
	Settings settings.Settings
	Runner   run.Runner
	Tools    tools.Tools
	Logger   traylog.Logger

	corePath string
	client   core.Client

	statusItem  *systray.MenuItem
	refresh     *systray.MenuItem
	enforce     *systray.MenuItem
	lockEnable  *systray.MenuItem
	lockDisable *systray.MenuItem
	lockRefresh *systray.MenuItem
	domainsItem *systray.MenuItem
	test        *systray.MenuItem
	check       *systray.MenuItem
	resolve     *systray.MenuItem
	about       *systray.MenuItem
	quit        *systray.MenuItem

	mu            sync.Mutex
	actionMu      sync.Mutex
	last          state.State
	actionRunning bool
}

func Run(cfg settings.Settings) {
	app := New(cfg)
	systray.Run(app.onReady, app.onExit)
}

func New(cfg settings.Settings) *App {
	cfg.Normalize()
	runner := run.ExecRunner{}
	detected := tools.Detect()
	logger := traylog.Logger{Path: cfg.LogFile}
	return &App{
		Settings: cfg,
		Runner:   runner,
		Tools:    detected,
		Logger:   logger,
	}
}

func (a *App) onReady() {
	a.Logger.Printf("startup")
	a.corePath = a.discoverCore()
	a.client = core.Client{CommandPath: a.corePath, ConfigPath: a.Settings.ConfigPath, Runner: a.Runner}

	systray.SetTitle("vpn-killswitch")
	systray.SetTooltip("vpn-killswitch starting")
	systray.SetIcon(icons.Offline)

	a.statusItem = systray.AddMenuItem("Статус: запуск", "")
	a.statusItem.Disable()
	a.refresh = systray.AddMenuItem("Обновить статус", "Перечитать текущее состояние")
	a.enforce = systray.AddMenuItem("Применить сейчас", "vpn-killswitch enforce tray")
	systray.AddSeparator()
	lockMenu := systray.AddMenuItem("Lock", "Управление lock")
	a.lockEnable = lockMenu.AddSubMenuItem("Включить lock", "vpn-killswitch lock enable tray")
	a.lockDisable = lockMenu.AddSubMenuItem("Выключить lock", "vpn-killswitch lock disable tray")
	a.lockRefresh = lockMenu.AddSubMenuItem("Обновить lock", "vpn-killswitch lock refresh tray")
	a.domainsItem = systray.AddMenuItem("Домены...", "Редактировать списки доменов")
	systray.AddSeparator()
	diagnostics := systray.AddMenuItem("Диагностика", "Диагностика только для чтения")
	a.test = diagnostics.AddSubMenuItem("Запустить test", "vpn-killswitch test --json")
	a.check = diagnostics.AddSubMenuItem("Запустить install-check", "vpn-killswitch install-check --json")
	a.resolve = diagnostics.AddSubMenuItem("Запустить resolve", "vpn-killswitch resolve --json")
	systray.AddSeparator()
	a.about = systray.AddMenuItem("О программе", "О vpn-killswitch-tray")
	a.quit = systray.AddMenuItem("Выход", "Выйти")
	a.updateActionAvailability()

	go a.handleMenu()
	go a.poll()
	a.refreshNow()
}

func (a *App) onExit() {
	a.Logger.Printf("shutdown")
}

func (a *App) discoverCore() string {
	path, err := command.Discover(a.Settings.CommandPath)
	if err != nil {
		a.Logger.Printf("vpn-killswitch not found: %v", err)
		return ""
	}
	a.Logger.Printf("using vpn-killswitch: %s", path)
	return path
}

func (a *App) handleMenu() {
	for {
		select {
		case <-a.refresh.ClickedCh:
			a.refreshNow()
		case <-a.enforce.ClickedCh:
			go a.runAction("Применение правил", "Правила применены", func(c core.Client) (core.ActionResult, error) {
				return c.Enforce("tray")
			})
		case <-a.lockEnable.ClickedCh:
			go a.runAction("Включение lock", "Lock включён", func(c core.Client) (core.ActionResult, error) {
				return c.LockEnable("tray")
			})
		case <-a.lockDisable.ClickedCh:
			go a.runAction("Выключение lock", "Lock выключен", func(c core.Client) (core.ActionResult, error) {
				return c.LockDisable("tray")
			})
		case <-a.lockRefresh.ClickedCh:
			go a.runAction("Обновление lock", "Lock обновлён", func(c core.Client) (core.ActionResult, error) {
				return c.LockRefresh("tray")
			})
		case <-a.domainsItem.ClickedCh:
			go a.runEditDomainsAction()
		case <-a.test.ClickedCh:
			a.runDiagnostic("test")
		case <-a.check.ClickedCh:
			a.runDiagnostic("install-check")
		case <-a.resolve.ClickedCh:
			a.runDiagnostic("resolve")
		case <-a.about.ClickedCh:
			a.Logger.Printf("about opened")
		case <-a.quit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (a *App) poll() {
	interval := time.Duration(a.Settings.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		a.refreshNow()
	}
}

func (a *App) refreshNow() {
	a.mu.Lock()
	defer a.mu.Unlock()

	var next state.State
	if a.corePath == "" {
		next = state.OfflineState("vpn-killswitch not found")
	} else {
		snapshot, err := a.client.Status()
		if err != nil {
			next = state.OfflineState(err.Error())
			a.Logger.Printf("status error: %v", err)
		} else {
			next = snapshot.State
		}
	}
	a.applyState(next)
}

func (a *App) applyState(next state.State) {
	a.last = next
	a.statusItem.SetTitle("Статус: " + next.Summary)
	systray.SetTooltip(next.Summary)
	switch next.Kind {
	case state.OK:
		systray.SetIcon(icons.OK)
	case state.Locked:
		systray.SetIcon(icons.Locked)
	case state.Warning, state.Error:
		systray.SetIcon(icons.Warning)
	default:
		systray.SetIcon(icons.Offline)
	}
	a.updateActionAvailability()
}

func (a *App) runDiagnostic(name string) {
	if a.corePath == "" {
		a.refreshNow()
		return
	}
	args := []string{name, "--json"}
	if a.Settings.ConfigPath != "" {
		args = append(args, "--config", a.Settings.ConfigPath)
	}
	out, _, err := a.Runner.Run(a.corePath, args...)
	if err != nil {
		a.Logger.Printf("%s error: %v", name, err)
		return
	}
	summary := diagnosticSummary(name, out)
	a.Logger.Printf("%s: %s", name, summary)
	a.refreshNow()
}

func diagnosticSummary(name, out string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		return strings.TrimSpace(out)
	}
	if ok, exists := data["ok"].(bool); exists {
		if ok {
			return "OK"
		}
		return "FAIL"
	}
	if name == "resolve" {
		return "resolve completed"
	}
	return fmt.Sprintf("%s completed", name)
}

type domainChanges struct {
	AddBypass    []string
	AddBypassIP  []string
	RemoveBypass []string
	RemoveIP     []string
	AddLock      []string
	RemoveLock   []string
}

func (c domainChanges) Empty() bool {
	return len(c.AddBypass) == 0 &&
		len(c.AddBypassIP) == 0 &&
		len(c.RemoveBypass) == 0 &&
		len(c.RemoveIP) == 0 &&
		len(c.AddLock) == 0 &&
		len(c.RemoveLock) == 0
}

func (a *App) runEditDomainsAction() {
	oldLists, err := domains.Load(a.Settings.ConfigPath)
	if err != nil {
		a.logMessage("Домены", fmt.Sprintf("Не удалось прочитать конфиг: %v", err))
		return
	}
	nextLists, ok := a.promptEditDomains(oldLists)
	if !ok {
		return
	}
	changes, err := diffDomainLists(oldLists, nextLists)
	if err != nil {
		a.logMessage("Домены", err.Error())
		return
	}
	if changes.Empty() {
		a.logMessage("Домены", "Изменений нет")
		return
	}
	a.runAction("Домены", "Домены сохранены", func(c core.Client) (core.ActionResult, error) {
		result, err := c.ApplyTray(core.ApplyTrayPayload{
			BypassDomains: nextLists.Bypass,
			BypassIPs:     nextLists.BypassIPs,
			LockDomains:   nextLists.Lock,
		}, a.Settings.AutoEnforceAfterConfigChange)
		if err != nil {
			return result, err
		}
		result.Stdout = fmt.Sprintf(
			"Списки сохранены: +bypass=%d +ip=%d -bypass=%d -ip=%d +lock=%d -lock=%d",
			len(changes.AddBypass),
			len(changes.AddBypassIP),
			len(changes.RemoveBypass),
			len(changes.RemoveIP),
			len(changes.AddLock),
			len(changes.RemoveLock),
		)
		return result, nil
	})
}

func diffDomainLists(oldLists, nextLists domains.Lists) (domainChanges, error) {
	if dup := firstOverlap(nextLists.Bypass, nextLists.Lock); dup != "" {
		return domainChanges{}, fmt.Errorf("домен %s не может быть одновременно в bypass и lock", dup)
	}
	return domainChanges{
		AddBypass:    missingFrom(nextLists.Bypass, oldLists.Bypass),
		AddBypassIP:  missingFrom(nextLists.BypassIPs, oldLists.BypassIPs),
		RemoveBypass: missingFrom(oldLists.Bypass, nextLists.Bypass),
		RemoveIP:     missingFrom(oldLists.BypassIPs, nextLists.BypassIPs),
		AddLock:      missingFrom(nextLists.Lock, oldLists.Lock),
		RemoveLock:   missingFrom(oldLists.Lock, nextLists.Lock),
	}, nil
}

func (a *App) runAction(title, success string, fn func(core.Client) (core.ActionResult, error)) {
	if a.corePath == "" {
		a.refreshNow()
		return
	}
	if !a.Tools.Pkexec {
		a.logMessage(title, "pkexec не найден")
		return
	}
	if !a.beginAction() {
		a.logMessage(title, "Другое действие уже выполняется")
		return
	}
	defer func() {
		a.endAction()
		a.refreshNow()
	}()

	client := a.actionClient()
	result, err := fn(client)
	if err != nil {
		if result.Cancelled {
			a.Logger.Printf("%s cancelled", title)
			return
		}
		a.Logger.Printf("%s error: %v", title, err)
		return
	}
	message := success
	if result.Stdout != "" {
		message = result.Stdout
	}
	a.Logger.Printf("%s: %s", title, message)
}

func (a *App) promptEditDomains(lists domains.Lists) (domains.Lists, bool) {
	if !a.Tools.Yad {
		a.logMessage("Домены", "Для редактируемых таблиц нужен yad")
		return domains.Lists{}, false
	}
	const sep = "\t"
	out, _, err := a.dialogRunner().Run(
		"yad",
		"--form",
		"--title", "Домены",
		"--width", "820",
		"--height", "620",
		"--text", "Редактируй списки. Bypass принимает домены, IPv4, CIDR и диапазоны IPv4. Lock принимает только домены.",
		"--field", "Bypass:TXT",
		"--field", "Lock:TXT",
		"--separator", sep,
		"--button", "Сохранить:0",
		"--button", "Отмена:1",
		strings.Join(domains.BypassEntries(lists), "\n"),
		strings.Join(lists.Lock, "\n"),
	)
	if err != nil {
		a.Logger.Printf("domains dialog cancelled or failed: %v", err)
		return domains.Lists{}, false
	}
	lists, err = parseDomainEditorOutputStrict(out, sep)
	if err != nil {
		a.logMessage("Домены", err.Error())
		return domains.Lists{}, false
	}
	return lists, true
}

func parseDomainEditorOutput(out, sep string) domains.Lists {
	lists, _ := parseDomainEditorOutputStrict(out, sep)
	return lists
}

func parseDomainEditorOutputStrict(out, sep string) (domains.Lists, error) {
	out = strings.TrimRight(out, "\r\n")
	fields := strings.SplitN(out, sep, 3)
	if len(fields) < 2 && sep != "|" {
		fields = strings.SplitN(out, "|", 3)
	}
	var lists domains.Lists
	if len(fields) > 0 {
		bypassDomains, bypassIPs, err := domains.ParseBypassInput(fields[0])
		if err != nil {
			return domains.Lists{}, err
		}
		lists.Bypass = bypassDomains
		lists.BypassIPs = bypassIPs
	}
	if len(fields) > 1 {
		lockDomains, err := domains.ParseDomainInput(fields[1])
		if err != nil {
			return domains.Lists{}, err
		}
		lists.Lock = lockDomains
	}
	return lists, nil
}

func firstOverlap(left, right []string) string {
	for _, value := range left {
		if domains.Contains(right, value) {
			return value
		}
	}
	return ""
}

func missingFrom(left, right []string) []string {
	var out []string
	for _, value := range left {
		if !domains.Contains(right, value) {
			out = append(out, value)
		}
	}
	return out
}

func (a *App) showText(title, text, fallbackNotification string) {
	if a.Tools.Zenity {
		_, _, err := a.dialogRunner().Run("zenity", "--info", "--no-markup", "--title", title, "--width", "520", "--height", "420", "--text", text)
		if err != nil {
			a.Logger.Printf("%s dialog error: %v", title, err)
		}
		return
	}
	if a.Tools.KDialog {
		_, _, err := a.dialogRunner().Run("kdialog", "--title", title, "--msgbox", text)
		if err != nil {
			a.Logger.Printf("%s dialog error: %v", title, err)
		}
		return
	}
	a.logMessage(title, fallbackNotification)
}

func (a *App) beginAction() bool {
	a.actionMu.Lock()
	if a.actionRunning {
		a.actionMu.Unlock()
		return false
	}
	a.actionRunning = true
	a.actionMu.Unlock()
	a.updateActionAvailability()
	return true
}

func (a *App) endAction() {
	a.actionMu.Lock()
	a.actionRunning = false
	a.actionMu.Unlock()
	a.updateActionAvailability()
}

func (a *App) updateActionAvailability() {
	disabled := a.corePath == "" || a.isActionRunning() || !a.Tools.Pkexec
	for _, item := range []*systray.MenuItem{a.enforce, a.lockEnable, a.lockDisable, a.lockRefresh} {
		if item == nil {
			continue
		}
		if disabled {
			item.Disable()
		} else {
			item.Enable()
		}
	}
	domainsDisabled := disabled || !a.Tools.Yad
	if a.domainsItem != nil {
		if domainsDisabled {
			a.domainsItem.Disable()
		} else {
			a.domainsItem.Enable()
		}
	}
}

func (a *App) isActionRunning() bool {
	a.actionMu.Lock()
	defer a.actionMu.Unlock()
	return a.actionRunning
}

func (a *App) logMessage(title, message string) {
	a.Logger.Printf("%s: %s", title, message)
}

func (a *App) actionClient() core.Client {
	return core.Client{CommandPath: a.corePath, ConfigPath: a.Settings.ConfigPath, Runner: run.ExecRunner{Timeout: run.UserActionTimeout}}
}

func (a *App) dialogRunner() run.Runner {
	return run.ExecRunner{Timeout: run.UserActionTimeout}
}
