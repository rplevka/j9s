// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"context"
	"regexp"
	"sync"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
)

// Table represents tabular data display.
type Table struct {
	*tview.Table
	actions   *KeyActions
	cmdBuff   *model.FishBuff
	filterFn  func(string)
	selectFn  func(int, int)
	ctx       context.Context
	mx        sync.RWMutex
	sortCol   int
	sortAsc   bool
	headers   []string
	rows      [][]string
	filtered  [][]string
	filter    string
	selection int
}

// NewTable returns a new table view.
func NewTable() *Table {
	t := Table{
		Table:   tview.NewTable(),
		actions: NewKeyActions(),
		cmdBuff: model.NewFishBuff('/', model.FilterBuffer),
		ctx:     context.Background(),
		sortAsc: true,
	}
	t.SetBackgroundColor(tcell.ColorDefault)
	t.SetBorders(false)
	t.SetSelectable(true, false)
	t.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite))
	return &t
}

// Init initializes the table.
func (t *Table) Init(ctx context.Context) {
	t.ctx = ctx
}

// Actions returns the key actions.
func (t *Table) Actions() *KeyActions {
	return t.actions
}

// CmdBuff returns the command buffer.
func (t *Table) CmdBuff() *model.FishBuff {
	return t.cmdBuff
}

// SetFilterFn sets the filter function.
func (t *Table) SetFilterFn(fn func(string)) {
	t.filterFn = fn
}

// SetSelectFn sets the selection function.
func (t *Table) SetSelectFn(fn func(int, int)) {
	t.selectFn = fn
}

// SetHeaders sets the table headers.
func (t *Table) SetHeaders(headers []string) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.headers = headers
}

// SetData sets the table data.
func (t *Table) SetData(rows [][]string) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.rows = rows
	t.applyFilter()
}

// GetSelectedItem returns the selected row data.
func (t *Table) GetSelectedItem() []string {
	t.mx.RLock()
	defer t.mx.RUnlock()

	row, _ := t.GetSelection()
	if row <= 0 || row > len(t.filtered) {
		return nil
	}
	return t.filtered[row-1]
}

// GetSelectedID returns the first column of selected row (typically ID/name).
func (t *Table) GetSelectedID() string {
	item := t.GetSelectedItem()
	if len(item) == 0 {
		return ""
	}
	return item[0]
}

// Filter sets the filter string and re-filters data.
func (t *Table) Filter(s string) {
	t.mx.Lock()
	t.filter = s
	t.applyFilter()
	t.mx.Unlock()
	t.Refresh()
}

// SetHighlight sets the filter string for highlighting only (no re-filtering).
func (t *Table) SetHighlight(s string) {
	t.mx.Lock()
	t.filter = s
	t.mx.Unlock()
}

// ClearFilter clears the filter.
func (t *Table) ClearFilter() {
	t.Filter("")
}

func (t *Table) applyFilter() {
	if t.filter == "" {
		t.filtered = t.rows
		return
	}

	// Try to compile as regex, fall back to substring match
	rx, err := regexp.Compile("(?i)" + t.filter)
	useRegex := err == nil

	t.filtered = make([][]string, 0)
	for _, row := range t.rows {
		for _, cell := range row {
			var matches bool
			if useRegex {
				matches = rx.MatchString(cell)
			} else {
				matches = containsIgnoreCase(cell, t.filter)
			}
			if matches {
				t.filtered = append(t.filtered, row)
				break
			}
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsLower(s, substr)))
}

// matchesFilter checks if text matches the filter (regex or substring).
func matchesFilter(text, filter string) bool {
	if filter == "" {
		return false
	}
	// Try regex first
	rx, err := regexp.Compile("(?i)" + filter)
	if err == nil {
		return rx.MatchString(text)
	}
	// Fall back to substring match
	return containsIgnoreCase(text, filter)
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Refresh refreshes the table display.
func (t *Table) Refresh() {
	t.mx.RLock()
	defer t.mx.RUnlock()

	t.Clear()

	// Render headers
	for col, h := range t.headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		t.SetCell(0, col, cell)
	}

	// Render data rows with highlighting
	for row, data := range t.filtered {
		for col, val := range data {
			displayVal := val
			if t.filter != "" && matchesFilter(val, t.filter) {
				// Highlight only the matching substring in red
				displayVal = highlightMatch(val, t.filter)
			}
			cell := tview.NewTableCell(displayVal).
				SetTextColor(tcell.ColorWhite).
				SetExpansion(1)
			t.SetCell(row+1, col, cell)
		}
	}

	t.ScrollToBeginning()
	if len(t.filtered) > 0 {
		t.Select(1, 0)
	}
}

// highlightMatch highlights matching substrings in red (like k9s).
func highlightMatch(text, filter string) string {
	if filter == "" {
		return text
	}

	// Try to use filter as regex, fall back to literal match
	rx, err := regexp.Compile("(?i)(" + filter + ")")
	if err != nil {
		// Invalid regex, escape and use as literal
		rx, err = regexp.Compile("(?i)(" + regexp.QuoteMeta(filter) + ")")
		if err != nil {
			return text
		}
	}

	// Replace matches with highlighted version
	return rx.ReplaceAllStringFunc(text, func(match string) string {
		return "[red::b]" + match + "[-::-]"
	})
}

// RowCount returns the number of data rows.
func (t *Table) RowCount() int {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return len(t.filtered)
}
