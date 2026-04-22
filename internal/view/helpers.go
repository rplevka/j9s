// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"github.com/atotto/clipboard"
	"github.com/derailed/tcell/v2"
	"github.com/roman-plevka/j9s/internal/ui"
)

// AddGlobalKeys adds common global keys to a KeyActions map.
func AddGlobalKeys(app *App, actions *ui.KeyActions) {
	actions.Bulk(ui.KeyMap{
		tcell.KeyCtrlC: ui.NewKeyAction("Quit", func(*tcell.EventKey) *tcell.EventKey {
			app.Stop()
			return nil
		}, false), // Hidden from menu
		ui.KeyColon: ui.NewKeyAction("Command", func(*tcell.EventKey) *tcell.EventKey {
			app.SetFilterMode(false)
			app.Prompt().SetIcon(':')
			app.ActivateCmd(true)
			app.TogglePrompt(true)
			return nil
		}, false), // Hidden from menu
		ui.KeySlash: ui.NewKeyAction("Filter", func(*tcell.EventKey) *tcell.EventKey {
			app.SetFilterMode(true)
			app.Prompt().SetIcon('/')
			app.ActivateCmd(true)
			app.TogglePrompt(true)
			return nil
		}, false), // Hidden from menu
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
		}, false), // Hidden from menu
		ui.KeyQuestion: ui.NewKeyAction("Help", func(*tcell.EventKey) *tcell.EventKey {
			app.Content.Push(NewHelpView(app, actions))
			return nil
		}, true),
		ui.KeyU: ui.NewKeyAction("Copy URL", func(*tcell.EventKey) *tcell.EventKey {
			top := app.Content.Top()
			if top == nil {
				return nil
			}
			if urlProvider, ok := top.(URLProvider); ok {
				url := urlProvider.GetJenkinsURL()
				if url != "" {
					if err := clipboard.WriteAll(url); err != nil {
						app.Flash().Warn("Failed to copy URL to clipboard")
					} else {
						app.Flash().Info("Copied: " + url)
					}
				} else {
					app.Flash().Warn("No URL available for this view")
				}
			} else {
				app.Flash().Warn("This view does not support URL generation")
			}
			return nil
		}, true),
	})
}
