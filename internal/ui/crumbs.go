// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"fmt"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/model"
)

// Crumbs represents breadcrumb navigation.
type Crumbs struct {
	*tview.TextView
	stack []string
}

// NewCrumbs returns a new crumbs view.
func NewCrumbs() *Crumbs {
	c := Crumbs{
		TextView: tview.NewTextView(),
		stack:    make([]string, 0),
	}
	c.SetDynamicColors(true)
	c.SetBackgroundColor(tcell.ColorDefault)
	return &c
}

// StackPushed notifies a component was added.
func (c *Crumbs) StackPushed(comp model.Component) {
	c.stack = append(c.stack, comp.Name())
	c.refresh()
}

// StackPopped notifies a component was removed.
func (c *Crumbs) StackPopped(_, _ model.Component) {
	if len(c.stack) > 0 {
		c.stack = c.stack[:len(c.stack)-1]
	}
	c.refresh()
}

// StackTop notifies the top component.
func (c *Crumbs) StackTop(_ model.Component) {
	c.refresh()
}

func (c *Crumbs) refresh() {
	if len(c.stack) == 0 {
		c.SetText("")
		return
	}

	parts := make([]string, len(c.stack))
	for i, s := range c.stack {
		if i == len(c.stack)-1 {
			parts[i] = fmt.Sprintf("[aqua::b]%s[-::-]", s)
		} else {
			parts[i] = fmt.Sprintf("[white::-]%s[-::-]", s)
		}
	}
	c.SetText(strings.Join(parts, " [gray::b]>[white::-] "))
}
