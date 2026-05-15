// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/config"
	"github.com/rplevka/j9s/internal/model"
)

// App represents the base application.
type App struct {
	*tview.Application
	config  *config.Config
	context string
	Main    *tview.Pages
	flash   *Flash
	menu    *Menu
	crumbs  *Crumbs
	prompt  *Prompt
	logo    *Logo
	cmdBuff *model.FishBuff
	actions *KeyActions
	views   map[string]tview.Primitive
}

// NewApp returns a new application.
func NewApp(cfg *config.Config, context string) *App {
	a := App{
		Application: tview.NewApplication(),
		config:      cfg,
		context:     context,
		Main:        tview.NewPages(),
		cmdBuff:     model.NewFishBuff(':', model.CommandBuffer),
		actions:     NewKeyActions(),
		views:       make(map[string]tview.Primitive),
	}

	return &a
}

// Init initializes the application.
func (a *App) Init() {
	a.flash = NewFlash(a)
	a.menu = NewMenu()
	a.crumbs = NewCrumbs()
	a.prompt = NewPrompt(a)
	a.logo = NewLogo()

	a.views["flash"] = a.flash
	a.views["menu"] = a.menu
	a.views["crumbs"] = a.crumbs
	a.views["prompt"] = a.prompt
	a.views["logo"] = a.logo
}

// Config returns the application configuration.
func (a *App) Config() *config.Config {
	return a.config
}

// Context returns the current context name.
func (a *App) Context() string {
	return a.context
}

// SetContext sets the current context.
func (a *App) SetContext(ctx string) {
	a.context = ctx
}

// Flash returns the flash view.
func (a *App) Flash() *Flash {
	return a.flash
}

// Menu returns the menu view.
func (a *App) Menu() *Menu {
	return a.menu
}

// Crumbs returns the crumbs view.
func (a *App) Crumbs() *Crumbs {
	return a.crumbs
}

// Prompt returns the prompt view.
func (a *App) Prompt() *Prompt {
	return a.prompt
}

// Logo returns the logo view.
func (a *App) Logo() *Logo {
	return a.logo
}

// CmdBuff returns the command buffer.
func (a *App) CmdBuff() *model.FishBuff {
	return a.cmdBuff
}

// Views returns the views map.
func (a *App) Views() map[string]tview.Primitive {
	return a.views
}

// Actions returns the key actions.
func (a *App) Actions() *KeyActions {
	return a.actions
}

// QueueUpdate queues an update.
func (a *App) QueueUpdate(f func()) {
	a.Application.QueueUpdate(f)
}

// QueueUpdateDraw queues an update and draw.
func (a *App) QueueUpdateDraw(f func()) {
	a.Application.QueueUpdateDraw(f)
}

// IsReadOnly returns true if the application is in read-only mode.
func (a *App) IsReadOnly() bool {
	return a.config.J9s.ReadOnly
}

// InCmdMode returns true if in command mode.
func (a *App) InCmdMode() bool {
	return a.prompt.InCmdMode()
}

// ResetPrompt resets the prompt.
func (a *App) ResetPrompt() {
	a.prompt.Reset()
}

// ActivateCmd activates command mode.
func (a *App) ActivateCmd(b bool) {
	a.prompt.SetActive(b)
}

// GetCmd returns the current command.
func (a *App) GetCmd() string {
	return a.cmdBuff.GetText()
}

// HasCmd returns true if there is a command.
func (a *App) HasCmd() bool {
	return !a.cmdBuff.Empty()
}

// ClearCmd clears the command.
func (a *App) ClearCmd() {
	a.cmdBuff.Clear()
}

// BailOut exits the application.
func (a *App) BailOut() {
	a.Stop()
}

// TogglePrompt shows or hides the command prompt in the layout.
func (a *App) TogglePrompt(show bool) {
	flex, ok := a.Main.GetPrimitive("main").(*tview.Flex)
	if !ok {
		return
	}

	// Prompt should be inserted at index 1 (after header, before content)
	if show {
		// Check if prompt is not already there
		if item := flex.ItemAt(1); item != a.prompt {
			flex.AddItemAtIndex(1, a.prompt, 3, 1, false)
		}
		a.Application.SetFocus(a.prompt)
	} else {
		// Remove prompt if it's at index 1
		if flex.ItemAt(1) == a.prompt {
			flex.RemoveItemAtIndex(1)
		}
	}
}

// PrevCmd returns the previous command from history.
func (a *App) PrevCmd(h *model.History) string {
	return h.Previous()
}

// NextCmd returns the next command from history.
func (a *App) NextCmd(h *model.History) string {
	return h.Next()
}

// Cow returns a cow view for errors.
func (a *App) Cow() *Cow {
	return NewCow(a)
}

// StatusIndicator represents a status indicator.
type StatusIndicator struct {
	*tview.TextView
}

// NewStatusIndicator returns a new status indicator.
func NewStatusIndicator(app *App) *StatusIndicator {
	s := StatusIndicator{
		TextView: tview.NewTextView(),
	}
	s.SetDynamicColors(true)
	s.SetTextAlign(tview.AlignRight)
	s.SetBackgroundColor(tcell.ColorDefault)

	return &s
}

// SetStatus sets the status text.
func (s *StatusIndicator) SetStatus(status string) {
	s.SetText(status)
}
