// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"fmt"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

var j9sLogo = []string{
	` ╦╔═╗╔═╗`,
	` ║╚═╗╚═╗`,
	`╚╝╚═╝╚═╝`,
}

// Logo represents the application logo.
type Logo struct {
	*tview.TextView
}

// NewLogo returns a new logo view.
func NewLogo() *Logo {
	l := Logo{
		TextView: tview.NewTextView(),
	}
	l.SetDynamicColors(true)
	l.SetBackgroundColor(tcell.ColorDefault)
	l.SetTextAlign(tview.AlignCenter)
	l.render()
	return &l
}

func (l *Logo) render() {
	text := ""
	for _, line := range j9sLogo {
		text += fmt.Sprintf("[orange::b]%s\n", line)
	}
	l.SetText(text)
}

// Reset resets the logo.
func (l *Logo) Reset() {
	l.render()
}
