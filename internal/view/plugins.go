// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// PluginsView displays Jenkins plugins.
type PluginsView struct {
	*tview.Flex
	app     *App
	table   *ui.Table
	actions *ui.KeyActions
}

// NewPluginsView returns a new plugins view.
func NewPluginsView(app *App) *PluginsView {
	v := &PluginsView{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		app:     app,
		table:   ui.NewTable(),
		actions: ui.NewKeyActions(),
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	return v
}

// Name returns the view name.
func (v *PluginsView) Name() string {
	return "Plugins"
}

// Hints returns the view hints.
func (v *PluginsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *PluginsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		ui.KeyR: ui.NewKeyAction("Refresh", v.refreshCmd, true),
	})

	v.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		if key == tcell.KeyRune {
			key = tcell.Key(evt.Rune())
		}
		if action, ok := v.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

func (v *PluginsView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		plugins, err := v.app.Client().GetPlugins(context.Background())
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderPlugins(plugins)
		})
	}()
}

func (v *PluginsView) renderPlugins(plugins []client.Plugin) {
	v.table.SetHeaders([]string{"NAME", "VERSION", "ENABLED", "UPDATE"})

	rows := make([][]string, 0, len(plugins))
	for _, plugin := range plugins {
		enabled := "[red::b]No[-::-]"
		if plugin.Enabled {
			enabled = "[green::b]Yes[-::-]"
		}

		update := "-"
		if plugin.HasUpdate {
			update = "[yellow::b]Available[-::-]"
		}

		rows = append(rows, []string{
			plugin.LongName,
			plugin.Version,
			enabled,
			update,
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle("Plugins")
	v.table.Refresh()
}

func (v *PluginsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}
