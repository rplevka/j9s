// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"fmt"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
)

// Prompt represents a command prompt with autocomplete.
type Prompt struct {
	*tview.TextView
	app         *App
	active      bool
	icon        rune
	buff        *model.FishBuff
	doneFunc    func(tcell.Key)
	changedFunc func(string)
}

// NewPrompt returns a new prompt.
func NewPrompt(app *App) *Prompt {
	p := &Prompt{
		TextView: tview.NewTextView(),
		app:      app,
		icon:     ':',
		buff:     model.NewFishBuff(':', model.CommandBuffer),
	}
	p.SetDynamicColors(true)
	p.SetBackgroundColor(tcell.ColorDefault)
	p.SetTextColor(tcell.ColorWhite)
	p.SetInputCapture(p.keyboard)
	return p
}

// SetSuggestionFn sets the suggestion function for autocomplete.
func (p *Prompt) SetSuggestionFn(fn model.SuggestionFunc) {
	p.buff.SetSuggestionFn(fn)
}

// SetChangedFunc sets the changed callback.
func (p *Prompt) SetChangedFunc(fn func(string)) {
	p.changedFunc = fn
}

// SetDoneFunc sets the done callback.
func (p *Prompt) SetDoneFunc(fn func(tcell.Key)) {
	p.doneFunc = fn
}

// GetText returns the current text.
func (p *Prompt) GetText() string {
	return p.buff.GetText()
}

// InCmdMode returns true if in command mode.
func (p *Prompt) InCmdMode() bool {
	return p.active
}

// SetActive sets the prompt active state.
func (p *Prompt) SetActive(b bool) {
	p.active = b
	if b {
		p.buff.Clear()
		p.buff.SetActive(true)
		p.render()
		p.app.Application.SetFocus(p)
	} else {
		p.buff.SetActive(false)
		p.Clear()
	}
}

// Reset resets the prompt.
func (p *Prompt) Reset() {
	p.buff.Clear()
	p.SetActive(false)
}

// SetIcon sets the prompt icon.
func (p *Prompt) SetIcon(r rune) {
	p.icon = r
}

func (p *Prompt) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	switch evt.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
		p.buff.Delete()
		p.render()
		if p.changedFunc != nil {
			p.changedFunc(p.buff.GetText())
		}
		return nil

	case tcell.KeyRune:
		p.buff.Add(evt.Rune())
		p.render()
		if p.changedFunc != nil {
			p.changedFunc(p.buff.GetText())
		}
		return nil

	case tcell.KeyTab, tcell.KeyRight:
		// Accept suggestion
		if p.buff.AcceptSuggestion() {
			p.render()
			if p.changedFunc != nil {
				p.changedFunc(p.buff.GetText())
			}
		}
		return nil

	case tcell.KeyUp:
		// Cycle through suggestions
		if _, ok := p.buff.NextSuggestion(); ok {
			p.render()
		}
		return nil

	case tcell.KeyDown:
		// Cycle through suggestions
		if _, ok := p.buff.PrevSuggestion(); ok {
			p.render()
		}
		return nil

	case tcell.KeyEscape:
		p.buff.Clear()
		if p.doneFunc != nil {
			p.doneFunc(tcell.KeyEscape)
		}
		return nil

	case tcell.KeyEnter:
		if p.doneFunc != nil {
			p.doneFunc(tcell.KeyEnter)
		}
		return nil

	case tcell.KeyCtrlU, tcell.KeyCtrlW:
		// Clear input
		p.buff.Clear()
		p.render()
		return nil
	}

	return evt
}

func (p *Prompt) render() {
	p.Clear()
	text := p.buff.GetText()
	suggestion := p.buff.GetSuggestion()

	// Format: icon text[dimmed suggestion]
	// Use a darker color for the suggestion (like k9s ghosted text)
	if suggestion != "" {
		fmt.Fprintf(p, "[white::b]%c [aqua::-]%s[gray::-]%s[-::-]", p.icon, text, suggestion)
	} else {
		fmt.Fprintf(p, "[white::b]%c [aqua::-]%s[-::-]", p.icon, text)
	}
}
