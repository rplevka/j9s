// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"github.com/derailed/tcell/v2"
	"github.com/roman-plevka/j9s/internal/ui"
)

// AddGlobalKeys adds common global keys to a KeyActions map.
func AddGlobalKeys(app *App, actions *ui.KeyActions) {
	actions.Bulk(ui.KeyMap{
		tcell.KeyCtrlC: ui.NewKeyAction("Quit", func(*tcell.EventKey) *tcell.EventKey {
			app.Stop()
			return nil
		}, true),
		ui.KeyColon: ui.NewKeyAction("Command", func(*tcell.EventKey) *tcell.EventKey {
			app.SetFilterMode(false)
			app.Prompt().SetIcon(':')
			app.ActivateCmd(true)
			app.TogglePrompt(true)
			return nil
		}, true),
		ui.KeySlash: ui.NewKeyAction("Filter", func(*tcell.EventKey) *tcell.EventKey {
			app.SetFilterMode(true)
			app.Prompt().SetIcon('/')
			app.ActivateCmd(true)
			app.TogglePrompt(true)
			return nil
		}, true),
		tcell.KeyEsc: ui.NewKeyAction("Back", func(*tcell.EventKey) *tcell.EventKey {
			if app.InCmdMode() {
				app.ResetPrompt()
				app.TogglePrompt(false)
				return nil
			}
			if app.Content.CanPop() {
				app.Content.Pop()
			}
			return nil
		}, true),
	})
}
