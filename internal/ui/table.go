// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
		sortCol: -1, // No sort column selected initially
		sortAsc: true,
	}
	t.SetBackgroundColor(tcell.ColorDefault)
	t.SetBorders(false)
	t.SetSelectable(true, false)
	t.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite))
	t.SetFixed(1, 0) // Keep header row fixed (always visible)
	// Add border and title like k9s
	t.SetBorder(true)
	t.SetBorderColor(tcell.ColorAqua)
	t.SetBorderPadding(0, 0, 1, 1)
	t.SetTitleColor(tcell.ColorAqua)
	t.SetTitleAlign(tview.AlignLeft)
	return &t
}

// SetTitle sets the table title with count.
func (t *Table) SetTitle(title string) {
	t.Table.SetTitle(t.styleTitle(title))
}

// styleTitle formats the title with styling.
func (t *Table) styleTitle(title string) string {
	rc := t.RowCount()
	filter := t.GetFilter()

	// Format: " Title (count) "
	styled := fmt.Sprintf(" [aqua::b]%s[white::d][[aqua::b]%d[white::d]] ", title, rc)

	// Add filter indicator if active
	if filter != "" {
		styled += fmt.Sprintf("[white::d]|[aqua::b]/%s ", filter)
	}

	return styled
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

// SelectByID selects the row with the given ID (first column value).
// Returns true if the row was found and selected.
func (t *Table) SelectByID(id string) bool {
	t.mx.RLock()
	defer t.mx.RUnlock()

	for i, row := range t.filtered {
		if len(row) > 0 && row[0] == id {
			// Row index is i+1 because row 0 is the header
			t.Select(i+1, 0)
			return true
		}
	}
	return false
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

// GetFilter returns the current filter string.
func (t *Table) GetFilter() string {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.filter
}

func (t *Table) applyFilter() {
	if t.filter == "" {
		// Make a copy to avoid sorting the original rows
		t.filtered = make([][]string, len(t.rows))
		copy(t.filtered, t.rows)
	} else {
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

	// Re-apply sort if a sort column is set
	if t.sortCol >= 0 {
		t.sortData()
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

	// Save current selection
	currentRow, currentCol := t.GetSelection()
	var selectedID string
	if currentRow > 0 && currentRow <= len(t.filtered) {
		// Save the ID of the selected row to restore it after refresh
		if len(t.filtered[currentRow-1]) > 0 {
			selectedID = t.filtered[currentRow-1][0]
		}
	}

	t.Clear()

	// Render headers with sort indicator
	for col, h := range t.headers {
		header := h
		if t.sortCol >= 0 && col == t.sortCol {
			if t.sortAsc {
				header = h + " ↑"
			} else {
				header = h + " ↓"
			}
		}
		cell := tview.NewTableCell(header).
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

	// Restore selection
	if len(t.filtered) > 0 {
		newRow := 1 // Default to first row
		// Try to find the previously selected item by ID
		if selectedID != "" {
			for i, row := range t.filtered {
				if len(row) > 0 && row[0] == selectedID {
					newRow = i + 1
					break
				}
			}
		} else if currentRow > 0 {
			// Fall back to same row number if possible
			newRow = currentRow
		}
		// Clamp to valid range
		if newRow > len(t.filtered) {
			newRow = len(t.filtered)
		}
		t.Select(newRow, currentCol)
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

// SortColCmd returns a command to sort by a specific column.
func (t *Table) SortColCmd(colIdx int) func(*tcell.EventKey) *tcell.EventKey {
	return func(*tcell.EventKey) *tcell.EventKey {
		t.SortByColumn(colIdx)
		return nil
	}
}

// SortByColumn sorts the table by the specified column index.
func (t *Table) SortByColumn(colIdx int) {
	t.mx.Lock()
	defer t.mx.Unlock()

	if colIdx < 0 || colIdx >= len(t.headers) {
		return
	}

	// Toggle sort order if same column, otherwise default to ascending
	if t.sortCol == colIdx {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = colIdx
		t.sortAsc = true
	}

	t.sortData()
}

// ToggleSortOrder toggles the sort order for the current column.
func (t *Table) ToggleSortOrder() {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.sortAsc = !t.sortAsc
	t.sortData()
}

// sortData sorts the filtered data by the current sort column.
func (t *Table) sortData() {
	if len(t.filtered) == 0 || t.sortCol < 0 {
		return
	}

	sort.SliceStable(t.filtered, func(i, j int) bool {
		if t.sortCol >= len(t.filtered[i]) || t.sortCol >= len(t.filtered[j]) {
			return false
		}

		a := stripColorTags(t.filtered[i][t.sortCol])
		b := stripColorTags(t.filtered[j][t.sortCol])

		// Try numeric comparison first
		aNum, aErr := strconv.ParseFloat(a, 64)
		bNum, bErr := strconv.ParseFloat(b, 64)
		if aErr == nil && bErr == nil {
			if t.sortAsc {
				return aNum < bNum
			}
			return aNum > bNum
		}

		// Try age/duration comparison (e.g., "5m", "2h", "3d")
		aDur := parseAgeDuration(a)
		bDur := parseAgeDuration(b)
		if aDur >= 0 && bDur >= 0 {
			if t.sortAsc {
				return aDur < bDur
			}
			return aDur > bDur
		}

		// Fall back to string comparison (case-insensitive)
		cmp := strings.Compare(strings.ToLower(a), strings.ToLower(b))
		if t.sortAsc {
			return cmp < 0
		}
		return cmp > 0
	})
}

// parseAgeDuration parses age strings like "5s", "10m", "2h", "3d" into seconds.
// Returns -1 if the string is not a valid age format.
func parseAgeDuration(s string) int64 {
	if len(s) < 2 {
		return -1
	}

	// Handle empty or dash values
	if s == "-" || s == "" {
		return -1
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return -1
	}

	switch unit {
	case 's':
		return num
	case 'm':
		return num * 60
	case 'h':
		return num * 3600
	case 'd':
		return num * 86400
	default:
		return -1
	}
}

// stripColorTags removes tview color tags from a string for comparison.
func stripColorTags(s string) string {
	// Remove color tags like [red::b], [-::-], [yellow], etc.
	re := regexp.MustCompile(`\[[^\]]*\]`)
	return re.ReplaceAllString(s, "")
}

// GetSortInfo returns the current sort column and order.
func (t *Table) GetSortInfo() (int, bool) {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.sortCol, t.sortAsc
}
