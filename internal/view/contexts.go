// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// ContextsView displays Jenkins contexts.
type ContextsView struct {
	*tview.Flex
	app     *App
	table   *ui.Table
	actions *ui.KeyActions
}

// NewContextsView returns a new contexts view.
func NewContextsView(app *App) *ContextsView {
	v := &ContextsView{
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
func (v *ContextsView) Name() string {
	return "Contexts"
}

// Hints returns the view hints.
func (v *ContextsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *ContextsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Switch", v.switchCmd, true),
	})

	v.table.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
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

func (v *ContextsView) refresh() {
	cfg := v.app.Config()
	if cfg.J9s == nil {
		return
	}

	v.table.SetHeaders([]string{"", "NAME", "URL"})

	rows := make([][]string, 0, len(cfg.J9s.Contexts))
	for _, ctx := range cfg.J9s.Contexts {
		current := ""
		if ctx.Name == cfg.J9s.CurrentContext {
			current = "[green::b]*[-::-]"
		}

		rows = append(rows, []string{
			current,
			ctx.Name,
			ctx.URL,
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle("Contexts")
	v.table.Refresh()
}

func (v *ContextsView) switchCmd(*tcell.EventKey) *tcell.EventKey {
	item := v.table.GetSelectedItem()
	if len(item) < 2 {
		return nil
	}

	ctxName := item[1]
	if ctxName == "" {
		return nil
	}

	if err := v.app.SwitchContext(ctxName); err != nil {
		v.app.Flash().Err(err)
		return nil
	}

	v.app.Flash().Info("Switched to context: " + ctxName)
	// Navigate to jobs view for the selected context
	v.app.Content.Push(NewJobsView(v.app))
	return nil
}
