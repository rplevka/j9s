// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"fmt"
	"time"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// FlashLevel represents flash message severity.
type FlashLevel int

const (
	// FlashInfo is an info message.
	FlashInfo FlashLevel = iota
	// FlashWarn is a warning message.
	FlashWarn
	// FlashErr is an error message.
	FlashErr
)

// Flash represents a flash message display.
type Flash struct {
	*tview.TextView
	app      *App
	cancelFn func()
}

// NewFlash returns a new flash.
func NewFlash(app *App) *Flash {
	f := Flash{
		TextView: tview.NewTextView(),
		app:      app,
	}
	f.SetDynamicColors(true)
	f.SetTextAlign(tview.AlignCenter)
	f.SetBackgroundColor(tcell.ColorDefault)
	return &f
}

// Info displays an info message.
func (f *Flash) Info(msg string) {
	f.show(FlashInfo, msg)
}

// Warn displays a warning message.
func (f *Flash) Warn(msg string) {
	f.show(FlashWarn, msg)
}

// Err displays an error message.
func (f *Flash) Err(err error) {
	if err == nil {
		return
	}
	f.show(FlashErr, err.Error())
}

// Errf displays a formatted error message.
func (f *Flash) Errf(format string, args ...interface{}) {
	f.show(FlashErr, fmt.Sprintf(format, args...))
}

// Clear clears the flash message.
func (f *Flash) Clear() {
	if f.cancelFn != nil {
		f.cancelFn()
		f.cancelFn = nil
	}
	f.SetText("")
}

func (f *Flash) show(level FlashLevel, msg string) {
	f.Clear()

	var color string
	switch level {
	case FlashInfo:
		color = "green"
	case FlashWarn:
		color = "yellow"
	case FlashErr:
		color = "red"
	}

	f.SetText(fmt.Sprintf("[%s::b]%s", color, msg))

	done := make(chan struct{})
	f.cancelFn = func() { close(done) }

	go func() {
		select {
		case <-done:
			return
		case <-time.After(3 * time.Second):
			f.app.QueueUpdateDraw(func() {
				f.SetText("")
			})
		}
	}()
}
