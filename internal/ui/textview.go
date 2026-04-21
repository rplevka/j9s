// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"regexp"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// FilterableTextView is a text view with filtering and highlighting support.
type FilterableTextView struct {
	*tview.TextView
	content   string
	filter    string
	actions   *KeyActions
	autoScroll bool
}

// NewFilterableTextView creates a new filterable text view.
func NewFilterableTextView() *FilterableTextView {
	tv := &FilterableTextView{
		TextView:   tview.NewTextView(),
		actions:    NewKeyActions(),
		autoScroll: true,
	}
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetBackgroundColor(tcell.ColorDefault)
	tv.SetWrap(true)
	return tv
}

// Actions returns the key actions.
func (t *FilterableTextView) Actions() *KeyActions {
	return t.actions
}

// SetContent sets the text content.
func (t *FilterableTextView) SetContent(content string) {
	t.content = content
	t.render()
}

// AppendContent appends text to the content.
func (t *FilterableTextView) AppendContent(text string) {
	t.content += text
	t.render()
	if t.autoScroll {
		t.ScrollToEnd()
	}
}

// GetContent returns the raw content.
func (t *FilterableTextView) GetContent() string {
	return t.content
}

// ClearContent clears all content.
func (t *FilterableTextView) ClearContent() {
	t.content = ""
	t.Clear()
}

// SetFilter sets the filter/search string.
func (t *FilterableTextView) SetFilter(filter string) {
	t.filter = filter
	t.render()
}

// GetFilter returns the current filter.
func (t *FilterableTextView) GetFilter() string {
	return t.filter
}

// ClearFilter clears the filter.
func (t *FilterableTextView) ClearFilter() {
	t.SetFilter("")
}

// SetAutoScroll enables/disables auto-scroll to end on new content.
func (t *FilterableTextView) SetAutoScroll(enabled bool) {
	t.autoScroll = enabled
}

// IsAutoScroll returns whether auto-scroll is enabled.
func (t *FilterableTextView) IsAutoScroll() bool {
	return t.autoScroll
}

// ScrollToTop scrolls to the beginning.
func (t *FilterableTextView) ScrollToTop() {
	t.ScrollToBeginning()
}

// ScrollToBottom scrolls to the end.
func (t *FilterableTextView) ScrollToBottom() {
	t.ScrollToEnd()
}

// render renders the content with optional highlighting.
func (t *FilterableTextView) render() {
	if t.content == "" {
		t.Clear()
		return
	}

	if t.filter == "" {
		t.SetText(t.content)
		return
	}

	// Escape any existing [ characters to prevent them being parsed as color tags
	escaped := strings.ReplaceAll(t.content, "[", "[[]")

	// Highlight matches
	highlighted := HighlightMatches(escaped, t.filter)
	t.SetText(highlighted)
}

// HighlightMatches highlights matching substrings in red bold.
// This is the common highlighting function used across all views.
func HighlightMatches(text, filter string) string {
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

	// Replace matches with highlighted version (red bold)
	return rx.ReplaceAllString(text, "[red::b]$1[-::-]")
}

// HighlightColor is the color used for highlighting matches.
const HighlightColor = "red"

// HighlightStyle is the tview style tag for highlighting.
const HighlightStyle = "[red::b]"

// HighlightReset is the tview style tag to reset highlighting.
const HighlightReset = "[-::-]"
