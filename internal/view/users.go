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

// UsersView displays Jenkins users.
type UsersView struct {
	*tview.Flex
	app     *App
	table   *ui.Table
	actions *ui.KeyActions
}

// NewUsersView returns a new users view.
func NewUsersView(app *App) *UsersView {
	v := &UsersView{
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
func (v *UsersView) Name() string {
	return "Users"
}

// Hints returns the view hints.
func (v *UsersView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *UsersView) bindKeys() {
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

func (v *UsersView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		users, err := v.app.Client().GetUsers(context.Background())
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderUsers(users)
		})
	}()
}

func (v *UsersView) renderUsers(users []client.User) {
	v.table.SetHeaders([]string{"ID", "FULL NAME", "DESCRIPTION"})

	rows := make([][]string, 0, len(users))
	for _, user := range users {
		desc := "-"
		if user.Description != "" {
			desc = truncate(user.Description, 50)
		}

		rows = append(rows, []string{
			user.ID,
			user.FullName,
			desc,
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle("Users")
	v.table.Refresh()
}

func (v *UsersView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}
