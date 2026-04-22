// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/config"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// App represents the main application view.
type App struct {
	*ui.App
	version       string
	Content       *PageStack
	command       *Command
	client        *client.Client
	cancelFn      context.CancelFunc
	cmdHistory    *model.History
	filterHistory *model.History
	showHeader    bool
	showLogo      bool
	filterMode    bool // true when in filter mode (/) vs command mode (:)
}

// NewApp returns a new application.
func NewApp(cfg *config.Config) *App {
	ctxName := ""
	if cfg.J9s != nil {
		ctxName = cfg.J9s.CurrentContext
	}

	a := App{
		App:           ui.NewApp(cfg, ctxName),
		cmdHistory:    model.NewHistory(model.MaxHistory),
		filterHistory: model.NewHistory(model.MaxHistory),
		Content:       NewPageStack(),
		showHeader:    true,
		showLogo:      true,
	}

	return &a
}

// Init initializes the application.
func (a *App) Init(version string, refreshRate int) error {
	a.version = version

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFn = cancel

	// Initialize base UI components first
	a.App.Init()

	if err := a.Content.Init(ctx); err != nil {
		return err
	}
	a.Content.SetApplication(a.App.Application)
	a.Content.AddListener(a.Crumbs())
	a.Content.AddListener(a.Menu())

	a.SetInputCapture(a.keyboard)
	a.bindKeys()

	// Initialize Jenkins client
	if err := a.initClient(); err != nil {
		slog.Warn("Failed to initialize Jenkins client", "error", err)
	}

	a.command = NewCommand(a)
	if err := a.command.Init(); err != nil {
		return err
	}

	// Set up prompt with autocomplete and done handler
	a.Prompt().SetSuggestionFn(a.command.Suggest)
	a.Prompt().SetDoneFunc(a.promptDone)
	a.Prompt().SetChangedFunc(a.promptChanged)

	a.layout(ctx)
	a.initSignals()

	return nil
}

func (a *App) initClient() error {
	ctx, err := a.Config().ActiveContext()
	if err != nil {
		return err
	}

	c, err := client.NewClient(ctx)
	if err != nil {
		return err
	}

	// Fetch CSRF crumb
	if err := c.FetchCrumb(context.Background()); err != nil {
		slog.Warn("Failed to fetch CSRF crumb", "error", err)
	}

	// Check connection
	if err := c.CheckConnection(context.Background()); err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}

	a.client = c
	slog.Info("Jenkins connection established")
	return nil
}

// Client returns the Jenkins client.
func (a *App) Client() *client.Client {
	return a.client
}

// SetFilterMode sets the filter mode flag.
func (a *App) SetFilterMode(b bool) {
	a.filterMode = b
}

// Run starts the application.
func (a *App) Run() error {
	// Check for bookmark and navigate to it, otherwise default to jobs
	bookmark := a.Config().GetBookmark()
	if bookmark != "" {
		// Navigate to bookmark with proper parent view stack
		a.command.navigateToBookmarkWithStack(bookmark)
	} else {
		a.gotoResource("jobs")
	}
	return a.Application.Run()
}

// Stop stops the application.
func (a *App) Stop() {
	if a.cancelFn != nil {
		a.cancelFn()
	}
	a.BailOut()
}

func (a *App) layout(ctx context.Context) {
	// Build header: Logo | Info | Menu
	header := tview.NewFlex().SetDirection(tview.FlexColumn)
	header.AddItem(a.buildInfoPanel(), 40, 1, false)
	header.AddItem(a.Menu(), 0, 1, false)
	header.AddItem(a.Logo(), 26, 1, false)

	// Main layout (top to bottom):
	// - Header (or status indicator if headless)
	// - Content (table)
	// - Crumbs
	// - Flash
	main := tview.NewFlex().SetDirection(tview.FlexRow)
	if a.showHeader {
		main.AddItem(header, 7, 1, false)
	} else {
		main.AddItem(a.buildInfoPanel(), 1, 1, false)
	}
	main.AddItem(a.Content, 0, 1, true)
	main.AddItem(a.Crumbs(), 1, 1, false)
	main.AddItem(a.Flash(), 1, 1, false)

	a.Main.AddPage("main", main, true, true)
	a.SetRoot(a.Main, true)
}

func (a *App) buildInfoPanel() tview.Primitive {
	info := tview.NewTextView()
	info.SetDynamicColors(true)
	info.SetBackgroundColor(tcell.ColorDefault)

	ctx, _ := a.Config().ActiveContext()
	ctxName := "N/A"
	url := "N/A"
	if ctx != nil {
		ctxName = ctx.Name
		url = ctx.URL
	}

	info.SetText(fmt.Sprintf(
		"[aqua::b]Context:[white::-] %s\n[aqua::b]URL:[white::-] %s\n[aqua::b]Version:[white::-] %s",
		ctxName, url, a.version,
	))

	return info
}

func (a *App) initSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		a.Stop()
	}()
}

func (a *App) bindKeys() {
	a.Actions().Bulk(ui.KeyMap{
		tcell.KeyCtrlC: ui.NewKeyAction("Quit", a.quitCmd, true),
		tcell.KeyEsc:   ui.NewKeyAction("Back/Clear", a.escapeCmd, true),
		ui.KeyColon:    ui.NewSharedKeyAction("Command", a.cmdCmd, false),
		ui.KeySlash:    ui.NewSharedKeyAction("Filter", a.filterCmd, false),
	})
}

func (a *App) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	if a.InCmdMode() {
		return evt
	}

	key := evt.Key()
	if key == tcell.KeyRune {
		key = tcell.Key(evt.Rune())
	}

	if action, ok := a.Actions().Get(key); ok {
		return action.Action(evt)
	}

	return evt
}

func (a *App) quitCmd(*tcell.EventKey) *tcell.EventKey {
	a.Stop()
	return nil
}

func (a *App) escapeCmd(*tcell.EventKey) *tcell.EventKey {
	if a.InCmdMode() {
		a.ResetPrompt()
		return nil
	}
	if a.Content.CanPop() {
		a.Content.Pop()
	}
	return nil
}

func (a *App) cmdCmd(*tcell.EventKey) *tcell.EventKey {
	a.filterMode = false
	a.Prompt().SetIcon(':')
	a.ActivateCmd(true)
	a.TogglePrompt(true)
	return nil
}

func (a *App) filterCmd(*tcell.EventKey) *tcell.EventKey {
	a.filterMode = true
	a.Prompt().SetIcon('/')
	a.ActivateCmd(true)
	a.TogglePrompt(true)
	return nil
}

func (a *App) gotoResource(res string) {
	a.command.Run(res)
}

func (a *App) promptDone(key tcell.Key) {
	text := a.Prompt().GetText()
	wasFilterMode := a.filterMode
	a.ResetPrompt()
	a.TogglePrompt(false)

	// Return focus to content
	if top := a.Content.Top(); top != nil {
		a.SetFocus(top.(tview.Primitive))
	}

	if key == tcell.KeyEscape {
		// Clear filter on escape
		if wasFilterMode {
			a.applyFilter("")
		}
		return
	}

	if text == "" {
		// Empty filter clears it
		if wasFilterMode {
			a.applyFilter("")
		}
		return
	}

	// Execute command or apply filter
	if key == tcell.KeyEnter {
		if wasFilterMode {
			a.applyFilter(text)
		} else {
			a.command.Run(text)
		}
	}
}

// promptChanged is called when the prompt text changes (for filter-as-you-type).
func (a *App) promptChanged(text string) {
	// Only apply filter-as-you-type in filter mode
	if a.filterMode {
		a.applyFilter(text)
	}
}

// applyFilter applies a filter to the current view if it supports filtering.
func (a *App) applyFilter(filter string) {
	top := a.Content.Top()
	if top == nil {
		return
	}

	// Check if the view supports filtering
	if filterable, ok := top.(interface{ SetFilter(string) }); ok {
		filterable.SetFilter(filter)
	}
}

// CmdHistory returns the command history.
func (a *App) CmdHistory() *model.History {
	return a.cmdHistory
}

// FilterHistory returns the filter history.
func (a *App) FilterHistory() *model.History {
	return a.filterHistory
}

// SwitchContext switches to a different Jenkins context.
func (a *App) SwitchContext(name string) error {
	if err := a.Config().SetActiveContext(name); err != nil {
		return err
	}
	if err := a.Config().Save(config.AppConfigFile); err != nil {
		slog.Warn("Failed to save config", "error", err)
	}
	return a.initClient()
}
