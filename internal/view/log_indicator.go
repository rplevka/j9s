// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"fmt"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

const indicatorSpacer = "  "

// LogIndicator represents a log view status indicator.
type LogIndicator struct {
	*tview.TextView

	autoScroll bool
	textWrap   bool
	fullScreen bool
}

// NewLogIndicator returns a new log indicator.
func NewLogIndicator() *LogIndicator {
	l := &LogIndicator{
		TextView:   tview.NewTextView(),
		autoScroll: true,
		textWrap:   true,
		fullScreen: false,
	}

	l.SetTextAlign(tview.AlignCenter)
	l.SetDynamicColors(true)
	l.SetBackgroundColor(tcell.ColorDefault)
	l.SetTextColor(tcell.ColorAqua)
	l.Refresh()

	return l
}

// SetAutoScroll sets the autoscroll state.
func (l *LogIndicator) SetAutoScroll(on bool) {
	l.autoScroll = on
	l.Refresh()
}

// SetTextWrap sets the text wrap state.
func (l *LogIndicator) SetTextWrap(on bool) {
	l.textWrap = on
	l.Refresh()
}

// SetFullScreen sets the full screen state.
func (l *LogIndicator) SetFullScreen(on bool) {
	l.fullScreen = on
	l.Refresh()
}

// Refresh updates the indicator display.
func (l *LogIndicator) Refresh() {
	l.Clear()

	var indicator string

	// Autoscroll indicator
	if l.autoScroll {
		indicator += fmt.Sprintf("[::b]Autoscroll:[green::b]On[-::-]%s", indicatorSpacer)
	} else {
		indicator += fmt.Sprintf("[::b]Autoscroll:[red::d]Off[-::-]%s", indicatorSpacer)
	}

	// FullScreen indicator
	if l.fullScreen {
		indicator += fmt.Sprintf("[::b]FullScreen:[green::b]On[-::-]%s", indicatorSpacer)
	} else {
		indicator += fmt.Sprintf("[::b]FullScreen:[red::d]Off[-::-]%s", indicatorSpacer)
	}

	// Wrap indicator
	if l.textWrap {
		indicator += "[::b]Wrap:[green::b]On[-::-]"
	} else {
		indicator += "[::b]Wrap:[red::d]Off[-::-]"
	}

	l.SetText(indicator)
}
