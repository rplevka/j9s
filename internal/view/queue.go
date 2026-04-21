// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"time"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// QueueView displays the Jenkins build queue.
type QueueView struct {
	*tview.Flex
	app     *App
	table   *ui.Table
	actions *ui.KeyActions
}

// NewQueueView returns a new queue view.
func NewQueueView(app *App) *QueueView {
	v := &QueueView{
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
func (v *QueueView) Name() string {
	return "Queue"
}

// Hints returns the view hints.
func (v *QueueView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *QueueView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		tcell.KeyCtrlD: ui.NewKeyAction("Cancel", v.cancelCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
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

func (v *QueueView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		items, err := v.app.Client().GetQueue(context.Background())
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderQueue(items)
		})
	}()
}

func (v *QueueView) renderQueue(items []client.QueueItem) {
	v.table.SetHeaders([]string{"ID", "JOB", "WHY", "WAITING"})

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		waiting := "-"
		if item.InQueueSince > 0 {
			waiting = formatAge(time.Unix(item.InQueueSince/1000, 0))
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", item.ID),
			item.Task.Name,
			truncate(item.Why, 50),
			waiting,
		})
	}

	v.table.SetData(rows)
	v.table.Refresh()
}

func (v *QueueView) cancelCmd(*tcell.EventKey) *tcell.EventKey {
	item := v.table.GetSelectedItem()
	if len(item) == 0 {
		return nil
	}

	var id int
	fmt.Sscanf(item[0], "%d", &id)
	if id <= 0 {
		return nil
	}

	go func() {
		err := v.app.Client().CancelQueueItem(context.Background(), id)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
			} else {
				v.app.Flash().Info(fmt.Sprintf("Queue item %d cancelled", id))
			}
			v.refresh()
		})
	}()
	return nil
}

func (v *QueueView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
