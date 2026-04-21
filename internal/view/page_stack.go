// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"

	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
)

// PageStack manages a stack of pages.
type PageStack struct {
	*tview.Pages
	stack     []model.Component
	listeners []model.StackListener
	app       *tview.Application
}

// NewPageStack returns a new page stack.
func NewPageStack() *PageStack {
	return &PageStack{
		Pages:     tview.NewPages(),
		stack:     make([]model.Component, 0),
		listeners: make([]model.StackListener, 0),
	}
}

// SetApplication sets the tview application for focus management.
func (p *PageStack) SetApplication(app *tview.Application) {
	p.app = app
}

// Init initializes the page stack.
func (p *PageStack) Init(ctx context.Context) error {
	return nil
}

// AddListener adds a stack listener.
func (p *PageStack) AddListener(l model.StackListener) {
	p.listeners = append(p.listeners, l)
}

// RemoveListener removes a stack listener.
func (p *PageStack) RemoveListener(l model.StackListener) {
	for i, listener := range p.listeners {
		if listener == l {
			p.listeners = append(p.listeners[:i], p.listeners[i+1:]...)
			return
		}
	}
}

// Push pushes a component onto the stack.
func (p *PageStack) Push(c model.Component) {
	p.stack = append(p.stack, c)
	prim := c.(tview.Primitive)
	p.AddPage(c.Name(), prim, true, true)
	if p.app != nil {
		p.app.SetFocus(prim)
	}
	p.notifyPush(c)
}

// Stoppable is an interface for components that can be stopped.
type Stoppable interface {
	Stop()
}

// Pop pops a component from the stack.
func (p *PageStack) Pop() model.Component {
	if len(p.stack) == 0 {
		return nil
	}

	old := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	p.RemovePage(old.Name())

	// Stop the component if it implements Stoppable (e.g., auto-refresh)
	if stoppable, ok := old.(Stoppable); ok {
		stoppable.Stop()
	}

	var top model.Component
	if len(p.stack) > 0 {
		top = p.stack[len(p.stack)-1]
		p.SwitchToPage(top.Name())
	}

	p.notifyPop(old, top)
	return old
}

// Top returns the top component.
func (p *PageStack) Top() model.Component {
	if len(p.stack) == 0 {
		return nil
	}
	return p.stack[len(p.stack)-1]
}

// CanPop returns true if the stack can be popped.
func (p *PageStack) CanPop() bool {
	return len(p.stack) > 1
}

// Clear clears the stack.
func (p *PageStack) Clear() {
	for len(p.stack) > 0 {
		p.Pop()
	}
}

// Len returns the stack length.
func (p *PageStack) Len() int {
	return len(p.stack)
}

func (p *PageStack) notifyPush(c model.Component) {
	for _, l := range p.listeners {
		l.StackPushed(c)
	}
}

func (p *PageStack) notifyPop(old, top model.Component) {
	for _, l := range p.listeners {
		l.StackPopped(old, top)
	}
}

func (p *PageStack) notifyTop(c model.Component) {
	for _, l := range p.listeners {
		l.StackTop(c)
	}
}
