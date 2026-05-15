// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/model"
)

const maxMenuRows = 6

var digitRX = regexp.MustCompile(`^\d+$`)

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

// HydrateMenu populates menu from hints (k9s style).
func (m *Menu) HydrateMenu(hh model.MenuHints) {
	m.Clear()
	sort.Sort(hh)

	// Filter visible hints
	visible := make(model.MenuHints, 0)
	for _, h := range hh {
		if h.Visible {
			visible = append(visible, h)
		}
	}

	if len(visible) == 0 {
		return
	}

	// Separate numeric hints (like 0-9) from regular hints
	var numericHints, regularHints model.MenuHints
	for _, h := range visible {
		if digitRX.MatchString(h.Mnemonic) {
			numericHints = append(numericHints, h)
		} else {
			regularHints = append(regularHints, h)
		}
	}

	// Calculate columns needed
	colCount := 1
	if len(numericHints) > 0 {
		colCount++
	}
	colCount += (len(regularHints)-1)/maxMenuRows + 1

	// Build table structure
	table := make([][]model.MenuHint, maxMenuRows)
	for i := range table {
		table[i] = make([]model.MenuHint, colCount)
	}

	// Track max key width per column for alignment
	maxKeyWidth := make([]int, colCount)

	// Fill numeric hints in first column
	col := 0
	if len(numericHints) > 0 {
		for row, h := range numericHints {
			if row >= maxMenuRows {
				break
			}
			table[row][col] = h
			if len(h.Mnemonic) > maxKeyWidth[col] {
				maxKeyWidth[col] = len(h.Mnemonic)
			}
		}
		col++
	}

	// Fill regular hints in remaining columns
	row := 0
	for _, h := range regularHints {
		table[row][col] = h
		if len(h.Mnemonic) > maxKeyWidth[col] {
			maxKeyWidth[col] = len(h.Mnemonic)
		}
		row++
		if row >= maxMenuRows {
			row = 0
			col++
		}
	}

	// Render table
	for r := range table {
		for c := range table[r] {
			h := table[r][c]
			var cellText string
			if h.Mnemonic != "" && h.Description != "" {
				cellText = formatHint(h, maxKeyWidth[c])
			}
			cell := tview.NewTableCell(cellText)
			cell.SetBackgroundColor(tcell.ColorDefault)
			m.SetCell(r, c, cell)
		}
	}
}

func formatHint(h model.MenuHint, keyWidth int) string {
	mnemonic := toMnemonic(h.Mnemonic)
	// Pad mnemonic for alignment
	padded := fmt.Sprintf("%-*s", keyWidth+2, mnemonic)

	if _, err := strconv.Atoi(h.Mnemonic); err == nil {
		// Numeric keys in yellow
		return fmt.Sprintf(" [yellow:-:b]%s[white:-:-] %s ", padded, h.Description)
	}
	// Regular keys in aqua
	return fmt.Sprintf(" [aqua:-:b]%s[white:-:-] %s ", padded, h.Description)
}

func toMnemonic(s string) string {
	if s == "" {
		return s
	}
	return "<" + strings.ToLower(s) + ">"
}
