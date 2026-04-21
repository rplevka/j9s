// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
)

const maxMenuRows = 6

// Menu presents menu options.
type Menu struct {
	*tview.Table
}

// NewMenu returns a new menu.
func NewMenu() *Menu {
	m := Menu{
		Table: tview.NewTable(),
	}
	m.SetBackgroundColor(tcell.ColorDefault)
	return &m
}

// StackPushed notifies a component was added.
func (m *Menu) StackPushed(c model.Component) {
	m.HydrateMenu(c.Hints())
}

// StackPopped notifies a component was removed.
func (m *Menu) StackPopped(_, top model.Component) {
	if top != nil {
		m.HydrateMenu(top.Hints())
	} else {
		m.Clear()
	}
}

// StackTop notifies the top component.
func (m *Menu) StackTop(t model.Component) {
	m.HydrateMenu(t.Hints())
}

// HydrateMenu populates menu from hints.
func (m *Menu) HydrateMenu(hh model.MenuHints) {
	m.Clear()
	sort.Sort(hh)

	visible := make(model.MenuHints, 0)
	for _, h := range hh {
		if h.Visible {
			visible = append(visible, h)
		}
	}

	cols := (len(visible) / maxMenuRows) + 1
	for i, h := range visible {
		row := i % maxMenuRows
		col := i / maxMenuRows
		cell := tview.NewTableCell(formatHint(h))
		cell.SetBackgroundColor(tcell.ColorDefault)
		m.SetCell(row, col, cell)
	}
	_ = cols
}

func formatHint(h model.MenuHint) string {
	if _, err := strconv.Atoi(h.Mnemonic); err == nil {
		return fmt.Sprintf(" [yellow:-:b]<%s> [white:-:-]%s ", h.Mnemonic, h.Description)
	}
	return fmt.Sprintf(" [aqua:-:b]<%s> [white:-:-]%s ", h.Mnemonic, h.Description)
}
