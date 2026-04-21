// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package model

import (
	"sort"
	"strings"
)

// MenuHint represents a menu hint.
type MenuHint struct {
	Mnemonic    string
	Description string
	Visible     bool
}

// IsBlank checks if hint is blank.
func (h MenuHint) IsBlank() bool {
	return h.Mnemonic == "" && h.Description == ""
}

// MenuHints represents a collection of menu hints.
type MenuHints []MenuHint

// Len returns the number of hints.
func (h MenuHints) Len() int {
	return len(h)
}

// Swap swaps hints at the given indices.
func (h MenuHints) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Less returns true if hint at i is less than hint at j.
func (h MenuHints) Less(i, j int) bool {
	return strings.ToLower(h[i].Mnemonic) < strings.ToLower(h[j].Mnemonic)
}

// Sort sorts the hints.
func (h MenuHints) Sort() {
	sort.Sort(h)
}

// Component represents a UI component.
type Component interface {
	// Name returns the component name.
	Name() string

	// Hints returns the component hints.
	Hints() MenuHints
}

// StackListener represents a stack listener.
type StackListener interface {
	// StackPushed notifies a component was added.
	StackPushed(Component)

	// StackPopped notifies a component was removed.
	StackPopped(old, top Component)

	// StackTop notifies the top component.
	StackTop(Component)
}

// TableListener represents a table listener.
type TableListener interface {
	// TableDataChanged notifies the table data changed.
	TableDataChanged(TableData)

	// TableLoadFailed notifies the table load failed.
	TableLoadFailed(error)
}

// TableData represents table data.
type TableData struct {
	Header    Row
	RowEvents RowEvents
	Namespace string
}

// Row represents a table row.
type Row []string

// RowEvent represents a row event.
type RowEvent struct {
	Kind   RowEventKind
	Row    Row
	Deltas Row
}

// RowEventKind represents a row event kind.
type RowEventKind int

const (
	// EventUnchanged indicates no change.
	EventUnchanged RowEventKind = iota
	// EventAdd indicates a new row.
	EventAdd
	// EventUpdate indicates a row update.
	EventUpdate
	// EventDelete indicates a row deletion.
	EventDelete
)

// RowEvents represents a collection of row events.
type RowEvents []RowEvent

// SuggestionFunc produces suggestions based on input text.
type SuggestionFunc func(text string) []string

// FishBuff represents a filter buffer with autocomplete support.
type FishBuff struct {
	text            []rune
	suggestion      string
	suggestions     []string
	suggestionIndex int
	suggestionFn    SuggestionFunc
	active          bool
	changed         bool
	kind            BufferKind
	hotKey          rune
}

// BufferKind represents a buffer kind.
type BufferKind int

const (
	// CommandBuffer represents a command buffer.
	CommandBuffer BufferKind = iota
	// FilterBuffer represents a filter buffer.
	FilterBuffer
)

// NewFishBuff returns a new fish buffer.
func NewFishBuff(hotKey rune, kind BufferKind) *FishBuff {
	return &FishBuff{
		hotKey:          hotKey,
		kind:            kind,
		text:            make([]rune, 0, 10),
		suggestionIndex: -1,
	}
}

// SetSuggestionFn sets the suggestion function.
func (f *FishBuff) SetSuggestionFn(fn SuggestionFunc) {
	f.suggestionFn = fn
}

// IsActive returns true if buffer is active.
func (f *FishBuff) IsActive() bool {
	return f.active
}

// SetActive sets the buffer active state.
func (f *FishBuff) SetActive(b bool) {
	f.active = b
	if !b {
		f.ClearSuggestions()
	}
}

// GetText returns the buffer text.
func (f *FishBuff) GetText() string {
	return string(f.text)
}

// SetText sets the buffer text.
func (f *FishBuff) SetText(s string) {
	f.text = []rune(s)
	f.changed = true
}

// Add adds a rune to the buffer.
func (f *FishBuff) Add(r rune) {
	f.text = append(f.text, r)
	f.changed = true
	f.updateSuggestions()
}

// Delete deletes the last rune from the buffer.
func (f *FishBuff) Delete() {
	if len(f.text) > 0 {
		f.text = f.text[:len(f.text)-1]
		f.changed = true
		f.updateSuggestions()
	}
}

// Clear clears the buffer.
func (f *FishBuff) Clear() {
	f.text = f.text[:0]
	f.suggestion = ""
	f.changed = true
	f.ClearSuggestions()
}

// Empty returns true if buffer is empty.
func (f *FishBuff) Empty() bool {
	return len(f.text) == 0
}

// Changed returns true if buffer changed.
func (f *FishBuff) Changed() bool {
	c := f.changed
	f.changed = false
	return c
}

// SetSuggestion sets the suggestion.
func (f *FishBuff) SetSuggestion(s string) {
	f.suggestion = s
}

// GetSuggestion returns the current suggestion (the part to complete).
func (f *FishBuff) GetSuggestion() string {
	return f.suggestion
}

// updateSuggestions updates the suggestions based on current text.
func (f *FishBuff) updateSuggestions() {
	if f.suggestionFn == nil {
		return
	}
	text := string(f.text)
	f.suggestions = f.suggestionFn(text)
	f.suggestionIndex = -1

	if len(f.suggestions) > 0 {
		f.suggestionIndex = 0
		// Set suggestion as the remaining part to complete
		if strings.HasPrefix(f.suggestions[0], text) {
			f.suggestion = strings.TrimPrefix(f.suggestions[0], text)
		} else {
			f.suggestion = f.suggestions[0]
		}
	} else {
		f.suggestion = ""
	}
}

// ClearSuggestions clears all suggestions.
func (f *FishBuff) ClearSuggestions() {
	f.suggestions = nil
	f.suggestionIndex = -1
	f.suggestion = ""
}

// CurrentSuggestion returns the current suggestion.
func (f *FishBuff) CurrentSuggestion() (string, bool) {
	if len(f.suggestions) == 0 || f.suggestionIndex < 0 || f.suggestionIndex >= len(f.suggestions) {
		return "", false
	}
	text := string(f.text)
	sug := f.suggestions[f.suggestionIndex]
	if strings.HasPrefix(sug, text) {
		return strings.TrimPrefix(sug, text), true
	}
	return sug, true
}

// NextSuggestion moves to the next suggestion.
func (f *FishBuff) NextSuggestion() (string, bool) {
	if len(f.suggestions) == 0 {
		return "", false
	}
	f.suggestionIndex++
	if f.suggestionIndex >= len(f.suggestions) {
		f.suggestionIndex = 0
	}
	return f.CurrentSuggestion()
}

// PrevSuggestion moves to the previous suggestion.
func (f *FishBuff) PrevSuggestion() (string, bool) {
	if len(f.suggestions) == 0 {
		return "", false
	}
	f.suggestionIndex--
	if f.suggestionIndex < 0 {
		f.suggestionIndex = len(f.suggestions) - 1
	}
	return f.CurrentSuggestion()
}

// AcceptSuggestion accepts the current suggestion.
func (f *FishBuff) AcceptSuggestion() bool {
	if sug, ok := f.CurrentSuggestion(); ok && sug != "" {
		f.text = append(f.text, []rune(sug)...)
		f.changed = true
		f.ClearSuggestions()
		return true
	}
	return false
}

// History represents command history.
type History struct {
	commands []string
	index    int
	max      int
}

// MaxHistory is the maximum history size.
const MaxHistory = 100

// NewHistory returns a new history.
func NewHistory(max int) *History {
	return &History{
		commands: make([]string, 0, max),
		max:      max,
	}
}

// Push adds a command to history.
func (h *History) Push(cmd string) {
	if cmd == "" {
		return
	}
	// Don't add duplicates
	if len(h.commands) > 0 && h.commands[len(h.commands)-1] == cmd {
		return
	}
	h.commands = append(h.commands, cmd)
	if len(h.commands) > h.max {
		h.commands = h.commands[1:]
	}
	h.index = len(h.commands)
}

// Previous returns the previous command.
func (h *History) Previous() string {
	if h.index > 0 {
		h.index--
	}
	if h.index < len(h.commands) {
		return h.commands[h.index]
	}
	return ""
}

// Next returns the next command.
func (h *History) Next() string {
	if h.index < len(h.commands)-1 {
		h.index++
		return h.commands[h.index]
	}
	h.index = len(h.commands)
	return ""
}

// Clear clears the history.
func (h *History) Clear() {
	h.commands = h.commands[:0]
	h.index = 0
}
