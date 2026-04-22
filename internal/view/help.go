// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"fmt"
	"sort"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

const (
	helpTitle    = "Help"
	helpTitleFmt = " [aqua::b]%s "
)

// HelpView presents a help viewer.
type HelpView struct {
	*tview.Table
	app         *App
	actions     *ui.KeyActions
	parentHints model.MenuHints
}

// NewHelpView returns a new help viewer.
func NewHelpView(app *App, parentActions *ui.KeyActions) *HelpView {
	h := &HelpView{
		Table:       tview.NewTable(),
		app:         app,
		actions:     ui.NewKeyActions(),
		parentHints: parentActions.Hints(),
	}

	h.SetBackgroundColor(tcell.ColorDefault)
	h.SetBorder(true)
	h.SetTitle(fmt.Sprintf(helpTitleFmt, helpTitle))
	h.SetTitleColor(tcell.ColorAqua)
	h.SetBorderColor(tcell.ColorAqua)
	h.SetSelectable(false, false)
	h.SetBorderPadding(0, 0, 1, 1)

	h.bindKeys()
	h.build()

	return h
}

// Name returns the view name.
func (h *HelpView) Name() string {
	return helpTitle
}

// Hints returns the view hints.
func (h *HelpView) Hints() model.MenuHints {
	return h.actions.Hints()
}

func (h *HelpView) bindKeys() {
	h.actions.Bulk(ui.KeyMap{
		tcell.KeyEscape: ui.NewKeyAction("Back", h.backCmd, false),
		ui.KeyQ:         ui.NewKeyAction("Back", h.backCmd, false),
		ui.KeyQuestion:  ui.NewKeyAction("Back", h.backCmd, false),
		tcell.KeyEnter:  ui.NewKeyAction("Back", h.backCmd, false),
	})

	h.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		if key == tcell.KeyRune {
			key = tcell.Key(evt.Rune())
		}
		if action, ok := h.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

func (h *HelpView) backCmd(*tcell.EventKey) *tcell.EventKey {
	if h.app.Content.CanPop() {
		h.app.Content.Pop()
	}
	return nil
}

func (h *HelpView) build() {
	h.Clear()

	// Collect all hints
	resourceHints := h.parentHints
	generalHints := h.showGeneral()
	navHints := h.showNav()

	// Sort hints
	sort.Sort(resourceHints)
	sort.Sort(generalHints)
	sort.Sort(navHints)

	// Calculate max widths for alignment
	maxKey := 0
	for _, hh := range []model.MenuHints{resourceHints, generalHints, navHints} {
		for _, hint := range hh {
			if len(hint.Mnemonic) > maxKey {
				maxKey = len(hint.Mnemonic)
			}
		}
	}
	maxKey += 4 // Add padding for <>

	// Build sections
	col := 0
	h.addSection(col, "RESOURCE", resourceHints, maxKey)
	col += 2
	h.addSection(col, "GENERAL", generalHints, maxKey)
	col += 2
	h.addSection(col, "NAVIGATION", navHints, maxKey)
}

func (h *HelpView) addSection(col int, title string, hints model.MenuHints, maxKey int) {
	// Add section header
	headerCell := tview.NewTableCell(fmt.Sprintf("[yellow::b]%s[-::-]", title))
	headerCell.SetBackgroundColor(tcell.ColorDefault)
	headerCell.SetSelectable(false)
	h.SetCell(0, col, headerCell)
	h.SetCell(0, col+1, tview.NewTableCell(""))

	// Add hints
	row := 1
	for _, hint := range hints {
		if !hint.Visible {
			continue
		}

		// Key cell
		keyCell := tview.NewTableCell(fmt.Sprintf("[aqua::b]<%s>[-::-]", hint.Mnemonic))
		keyCell.SetBackgroundColor(tcell.ColorDefault)
		keyCell.SetSelectable(false)
		h.SetCell(row, col, keyCell)

		// Description cell
		descCell := tview.NewTableCell(fmt.Sprintf("[white:-:-]%s", hint.Description))
		descCell.SetBackgroundColor(tcell.ColorDefault)
		descCell.SetSelectable(false)
		h.SetCell(row, col+1, descCell)

		row++
	}
}

func (h *HelpView) showGeneral() model.MenuHints {
	return model.MenuHints{
		{Mnemonic: ":", Description: "Command mode", Visible: true},
		{Mnemonic: "/", Description: "Filter mode", Visible: true},
		{Mnemonic: "Esc", Description: "Back/Clear", Visible: true},
		{Mnemonic: "Ctrl-c", Description: "Quit", Visible: true},
		{Mnemonic: "?", Description: "Help", Visible: true},
		{Mnemonic: "q", Description: "Back", Visible: true},
		{Mnemonic: "u", Description: "Copy Jenkins URL", Visible: true},
		{Mnemonic: "Shift-N", Description: "Sort by Name/Number", Visible: true},
		{Mnemonic: "Shift-S", Description: "Sort by Status", Visible: true},
		{Mnemonic: "Shift-A", Description: "Sort by Age", Visible: true},
		{Mnemonic: "Shift-R", Description: "Sort by Result", Visible: true},
	}
}

func (h *HelpView) showNav() model.MenuHints {
	return model.MenuHints{
		{Mnemonic: "g", Description: "Top (gg)", Visible: true},
		{Mnemonic: "G", Description: "Bottom", Visible: true},
		{Mnemonic: "j/↓", Description: "Down", Visible: true},
		{Mnemonic: "k/↑", Description: "Up", Visible: true},
		{Mnemonic: "h/←", Description: "Left", Visible: true},
		{Mnemonic: "l/→", Description: "Right", Visible: true},
		{Mnemonic: "^/0", Description: "Line start", Visible: true},
		{Mnemonic: "$", Description: "Line end", Visible: true},
		{Mnemonic: "{", Description: "Page up", Visible: true},
		{Mnemonic: "}", Description: "Page down", Visible: true},
	}
}
