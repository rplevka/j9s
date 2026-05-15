// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/client"
	"github.com/rplevka/j9s/internal/model"
	"github.com/rplevka/j9s/internal/ui"
)

// NodesView displays Jenkins nodes/agents.
type NodesView struct {
	*tview.Flex
	app     *App
	table   *ui.Table
	actions *ui.KeyActions
}

// NewNodesView returns a new nodes view.
func NewNodesView(app *App) *NodesView {
	v := &NodesView{
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
func (v *NodesView) Name() string {
	return "Nodes"
}

// Hints returns the view hints.
func (v *NodesView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *NodesView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		ui.KeyD: ui.NewKeyAction("Describe", v.describeCmd, true),
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

func (v *NodesView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		nodes, err := v.app.Client().GetNodes(context.Background())
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderNodes(nodes)
		})
	}()
}

func (v *NodesView) renderNodes(nodes []client.Node) {
	v.table.SetHeaders([]string{"NAME", "STATUS", "EXECUTORS", "IDLE", "REASON"})

	rows := make([][]string, 0, len(nodes))
	for _, node := range nodes {
		status := "[green::b]Online[-::-]"
		if node.Offline {
			status = "[red::b]Offline[-::-]"
		} else if node.TemporarilyOffline {
			status = "[yellow::b]Temp Offline[-::-]"
		}

		idle := "No"
		if node.Idle {
			idle = "Yes"
		}

		reason := "-"
		if node.OfflineCauseReason != "" {
			reason = truncate(node.OfflineCauseReason, 40)
		}

		rows = append(rows, []string{
			node.DisplayName,
			status,
			fmt.Sprintf("%d", node.NumExecutors),
			idle,
			reason,
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle("Nodes")
	v.table.Refresh()
}

func (v *NodesView) describeCmd(*tcell.EventKey) *tcell.EventKey {
	nodeName := v.table.GetSelectedID()
	if nodeName == "" {
		return nil
	}
	descView := NewDescribeView(v.app, "node", nodeName)
	v.app.Content.Push(descView)
	return nil
}

func (v *NodesView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}
