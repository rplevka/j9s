// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"fmt"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

var cowTpl = `
 ________________________
< %s >
 ------------------------
        \   ^__^
         \  (oo)\_______
            (__)\       )\/\
                ||----w |
                ||     ||
`

// Cow represents an error display with a cow.
type Cow struct {
	*tview.TextView
	app *App
}

// NewCow returns a new cow view.
func NewCow(app *App) *Cow {
	c := Cow{
		TextView: tview.NewTextView(),
		app:      app,
	}
	c.SetDynamicColors(true)
	c.SetBackgroundColor(tcell.ColorDefault)
	c.SetTextAlign(tview.AlignCenter)
	return &c
}

// Show displays an error message.
func (c *Cow) Show(msg string) {
	// Truncate long messages
	if len(msg) > 40 {
		msg = msg[:37] + "..."
	}
	// Pad short messages
	if len(msg) < 20 {
		msg = msg + strings.Repeat(" ", 20-len(msg))
	}
	c.SetText(fmt.Sprintf("[red::b]%s", fmt.Sprintf(cowTpl, msg)))
}
