// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/client"
	"github.com/rplevka/j9s/internal/model"
	"github.com/rplevka/j9s/internal/ui"
)

// CredentialsView displays Jenkins credentials.
type CredentialsView struct {
	*tview.Flex
	app     *App
	table   *ui.Table
	actions *ui.KeyActions
}

// NewCredentialsView returns a new credentials view.
func NewCredentialsView(app *App) *CredentialsView {
	v := &CredentialsView{
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
func (v *CredentialsView) Name() string {
	return "Credentials"
}

// Hints returns the view hints.
func (v *CredentialsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *CredentialsView) bindKeys() {
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

func (v *CredentialsView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		creds, err := v.app.Client().GetCredentials(context.Background(), "_")
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderCredentials(creds)
		})
	}()
}

func (v *CredentialsView) renderCredentials(creds []client.Credential) {
	v.table.SetHeaders([]string{"ID", "NAME", "TYPE", "DESCRIPTION"})

	rows := make([][]string, 0, len(creds))
	for _, cred := range creds {
		desc := "-"
		if cred.Description != "" {
			desc = truncate(cred.Description, 40)
		}

		rows = append(rows, []string{
			cred.ID,
			cred.DisplayName,
			cred.TypeName,
			desc,
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle("Credentials")
	v.table.Refresh()
}

func (v *CredentialsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}
