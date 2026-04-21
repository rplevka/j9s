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

// ViewsView displays Jenkins views.
type ViewsView struct {
	*tview.Flex
	app     *App
	table   *ui.Table
	actions *ui.KeyActions
}

// NewViewsView returns a new views view.
func NewViewsView(app *App) *ViewsView {
	v := &ViewsView{
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
func (v *ViewsView) Name() string {
	return "Views"
}

// Hints returns the view hints.
func (v *ViewsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *ViewsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Jobs", v.enterCmd, true),
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

func (v *ViewsView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		views, err := v.app.Client().GetViews(context.Background())
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderViews(views)
		})
	}()
}

func (v *ViewsView) renderViews(views []client.View) {
	v.table.SetHeaders([]string{"NAME", "DESCRIPTION"})

	rows := make([][]string, 0, len(views))
	for _, view := range views {
		desc := "-"
		if view.Description != "" {
			desc = truncate(view.Description, 60)
		}

		rows = append(rows, []string{
			view.Name,
			desc,
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle("Views")
	v.table.Refresh()
}

func (v *ViewsView) enterCmd(*tcell.EventKey) *tcell.EventKey {
	viewName := v.table.GetSelectedID()
	if viewName == "" {
		return nil
	}
	jobsView := NewViewJobsView(v.app, viewName)
	v.app.Content.Push(jobsView)
	return nil
}

func (v *ViewsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

// ViewJobsView displays jobs filtered by a Jenkins view.
type ViewJobsView struct {
	*JobsView
	viewName string
}

// NewViewJobsView returns a new view jobs view.
func NewViewJobsView(app *App, viewName string) *ViewJobsView {
	v := &ViewJobsView{
		JobsView: NewJobsView(app),
		viewName: viewName,
	}
	v.refreshViewJobs()
	return v
}

// Name returns the view name.
func (v *ViewJobsView) Name() string {
	return "Jobs[" + v.viewName + "]"
}

func (v *ViewJobsView) refreshViewJobs() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		view, err := v.app.Client().GetView(context.Background(), v.viewName)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderJobs(view.Jobs)
		})
	}()
}
