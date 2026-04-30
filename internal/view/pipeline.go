// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// -----------------------------------------------------------------------
// PipelineGraphView — top level (build → DAG of nodes).
// Shows a Blue-Ocean-style pipeline graph with parallel branches.
// Enter on a node opens its step list; <l> jumps straight to the
// aggregated node log.
// -----------------------------------------------------------------------

// PipelineGraphView lists the Blue Ocean DAG nodes for a build with an
// ASCII graph in the first column and per-node status/duration cells.
type PipelineGraphView struct {
	*tview.Flex
	app      *App
	table    *ui.Table
	actions  *ui.KeyActions
	jobName  string
	buildNum int
	rows     []GraphRow
}

// NewPipelineGraphView returns a new pipeline graph view for a build.
func NewPipelineGraphView(app *App, jobName string, buildNum int) *PipelineGraphView {
	v := &PipelineGraphView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  jobName,
		buildNum: buildNum,
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	return v
}

// Name returns the view name (used by Crumbs).
func (v *PipelineGraphView) Name() string {
	return fmt.Sprintf("Pipeline[%s#%d]", v.jobName, v.buildNum)
}

// Hints returns the menu hints.
func (v *PipelineGraphView) Hints() model.MenuHints { return v.actions.Hints() }

// SetFilter forwards the filter to the table.
func (v *PipelineGraphView) SetFilter(filter string) { v.table.Filter(filter) }

// IDs returns the node IDs; powers prompt argument completion.
func (v *PipelineGraphView) IDs() []string { return v.table.GetRowIDs() }

// GetJenkinsURL returns the Blue Ocean web URL for this pipeline run,
// or — when a node is selected — the URL pointing into that node.
func (v *PipelineGraphView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns pipeline/<jobPath>/<buildNum>[/<nodeID>].
func (v *PipelineGraphView) GetViewPath() string {
	if id := v.table.GetSelectedID(); id != "" {
		return fmt.Sprintf("pipeline/%s/%d/%s", v.jobName, v.buildNum, id)
	}
	return fmt.Sprintf("pipeline/%s/%d", v.jobName, v.buildNum)
}

// GetParentID returns the build number string so navigation back to the
// builds view restores the originating row's selection.
func (v *PipelineGraphView) GetParentID() string {
	return fmt.Sprintf("#%d", v.buildNum)
}

func (v *PipelineGraphView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Steps", v.enterCmd, true),
		ui.KeyL:        ui.NewKeyAction("Logs", v.logsCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
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

func (v *PipelineGraphView) refresh() {
	if v.app.Client() == nil {
		v.app.Flash().Err(fmt.Errorf("not connected to Jenkins"))
		return
	}
	go func() {
		nodes, err := v.app.Client().GetPipelineNodes(context.Background(), v.jobName, v.buildNum)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				if errors.Is(err, client.ErrBlueOceanUnavailable) {
					v.app.Flash().Warn("Blue Ocean REST API unavailable (install the blueocean-rest plugin or check this is a Pipeline job)")
				} else {
					v.app.Flash().Err(err)
				}
				return
			}
			v.rows = LayoutGraph(nodes)
			v.renderRows()
		})
	}()
}

// renderRows populates the table from v.rows. Exposed for unit tests so
// the goroutine in refresh() can be sidestepped.
//
// The table follows j9s' convention of "col 0 = row ID" so that
// GetSelectedID() returns the Blue Ocean node ID. The visual shape of
// the pipeline lives in a dedicated GRAPH column where the ASCII
// prefix, status icon and display name are concatenated.
func (v *PipelineGraphView) renderRows() {
	v.table.SetHeaders([]string{"ID", "GRAPH", "STATUS", "DURATION"})
	rows := make([][]string, 0, len(v.rows))
	for _, r := range v.rows {
		graph := r.Prefix + colorizeBlueState(r.Node.Result, r.Node.State) + " " + r.Node.DisplayName
		rows = append(rows, []string{
			r.Node.ID,
			graph,
			colorizeBlueResult(r.Node.Result, r.Node.State),
			formatBlueDuration(r.Node),
		})
	}
	v.table.SetData(rows)
	v.table.SetTitle(fmt.Sprintf("Pipeline:%s#%d", v.jobName, v.buildNum))
	v.table.Refresh()
}

func (v *PipelineGraphView) enterCmd(*tcell.EventKey) *tcell.EventKey {
	id := v.table.GetSelectedID()
	if id == "" {
		return nil
	}
	node := v.findNode(id)
	if node == nil {
		return nil
	}
	v.app.Content.Push(NewPipelineNodeStepsView(v.app, v.jobName, v.buildNum, *node))
	return nil
}

func (v *PipelineGraphView) logsCmd(*tcell.EventKey) *tcell.EventKey {
	id := v.table.GetSelectedID()
	if id == "" {
		return nil
	}
	node := v.findNode(id)
	if node == nil {
		return nil
	}
	v.app.Content.Push(NewPipelineNodeLogsView(v.app, v.jobName, v.buildNum, *node, ""))
	return nil
}

func (v *PipelineGraphView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

func (v *PipelineGraphView) findNode(id string) *client.BlueNode {
	for i := range v.rows {
		if v.rows[i].Node.ID == id {
			return &v.rows[i].Node
		}
	}
	return nil
}

// -----------------------------------------------------------------------
// PipelineNodeStepsView — second level (node → list of steps).
// Each step row drills into PipelineNodeLogsView for that step.
// -----------------------------------------------------------------------

// PipelineNodeStepsView lists the steps of one pipeline node.
type PipelineNodeStepsView struct {
	*tview.Flex
	app      *App
	table    *ui.Table
	actions  *ui.KeyActions
	jobName  string
	buildNum int
	node     client.BlueNode
	steps    []client.BlueStep
}

// NewPipelineNodeStepsView returns a new step list view for a pipeline node.
func NewPipelineNodeStepsView(app *App, jobName string, buildNum int, node client.BlueNode) *PipelineNodeStepsView {
	v := &PipelineNodeStepsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  jobName,
		buildNum: buildNum,
		node:     node,
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	return v
}

// Name returns the view name.
func (v *PipelineNodeStepsView) Name() string {
	return fmt.Sprintf("Steps[%s#%d/%s]", v.jobName, v.buildNum, v.node.DisplayName)
}

// Hints returns the menu hints.
func (v *PipelineNodeStepsView) Hints() model.MenuHints { return v.actions.Hints() }

// SetFilter forwards the filter to the table.
func (v *PipelineNodeStepsView) SetFilter(filter string) { v.table.Filter(filter) }

// IDs returns the step IDs.
func (v *PipelineNodeStepsView) IDs() []string { return v.table.GetRowIDs() }

// GetJenkinsURL returns the Blue Ocean URL for this node.
func (v *PipelineNodeStepsView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns pipeline/<jobPath>/<buildNum>/<nodeID>[/<stepID>].
func (v *PipelineNodeStepsView) GetViewPath() string {
	if id := v.table.GetSelectedID(); id != "" {
		return fmt.Sprintf("pipeline/%s/%d/%s/%s", v.jobName, v.buildNum, v.node.ID, id)
	}
	return fmt.Sprintf("pipeline/%s/%d/%s", v.jobName, v.buildNum, v.node.ID)
}

// GetParentID returns the parent node ID for selection restoration.
func (v *PipelineNodeStepsView) GetParentID() string {
	return v.node.ID
}

func (v *PipelineNodeStepsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Logs", v.enterCmd, true),
		ui.KeyL:        ui.NewKeyAction("Logs", v.enterCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
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

func (v *PipelineNodeStepsView) refresh() {
	if v.app.Client() == nil {
		v.app.Flash().Err(fmt.Errorf("not connected to Jenkins"))
		return
	}
	go func() {
		steps, err := v.app.Client().GetPipelineNodeSteps(context.Background(), v.jobName, v.buildNum, v.node.ID)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
				return
			}
			v.steps = steps
			v.renderSteps()
		})
	}()
}

// renderSteps populates the table from v.steps. Exposed for tests.
func (v *PipelineNodeStepsView) renderSteps() {
	v.table.SetHeaders([]string{"ID", "STEP", "STATUS", "DURATION"})
	rows := make([][]string, 0, len(v.steps))
	for _, s := range v.steps {
		name := s.DisplayName
		if name == "" {
			name = "step " + s.ID
		}
		row := []string{
			s.ID,
			colorizeBlueState(s.Result, s.State) + " " + name,
			colorizeBlueResult(s.Result, s.State),
			formatBlueDurationStep(s),
		}
		rows = append(rows, row)
	}
	v.table.SetData(rows)
	v.table.SetTitle(fmt.Sprintf("Steps:%s#%d/%s", v.jobName, v.buildNum, v.node.DisplayName))
	v.table.Refresh()
}

func (v *PipelineNodeStepsView) enterCmd(*tcell.EventKey) *tcell.EventKey {
	id := v.table.GetSelectedID()
	if id == "" {
		return nil
	}
	v.app.Content.Push(NewPipelineNodeLogsView(v.app, v.jobName, v.buildNum, v.node, id))
	return nil
}

func (v *PipelineNodeStepsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

// -----------------------------------------------------------------------
// PipelineNodeLogsView — third level (node/step → text).
// stepID == "" means "show the aggregated log for the whole node",
// matching what the <l> hotkey on PipelineGraphView produces.
// -----------------------------------------------------------------------

// PipelineNodeLogsView shows the canned log text of either an entire
// pipeline node or one specific step inside it. The view does *not*
// stream — pipeline nodes are short-lived; if you want live tail use
// the regular LogsView (`<l>` from the builds list).
type PipelineNodeLogsView struct {
	*tview.Flex
	app      *App
	textView *ui.FilterableTextView
	actions  *ui.KeyActions
	jobName  string
	buildNum int
	node     client.BlueNode
	stepID   string // "" = node-level log
}

// NewPipelineNodeLogsView returns a new log viewer. stepID == "" means
// "aggregated node log".
func NewPipelineNodeLogsView(app *App, jobName string, buildNum int, node client.BlueNode, stepID string) *PipelineNodeLogsView {
	v := &PipelineNodeLogsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		textView: ui.NewFilterableTextView(),
		actions:  ui.NewKeyActions(),
		jobName:  jobName,
		buildNum: buildNum,
		node:     node,
		stepID:   stepID,
	}
	v.SetBorder(true)
	v.SetBorderColor(tcell.ColorAqua)
	v.SetTitleColor(tcell.ColorAqua)
	v.SetTitleAlign(tview.AlignLeft)
	v.textView.SetBorderPadding(0, 0, 1, 1)
	v.SetTitle(v.styledTitle())
	v.AddItem(v.textView, 0, 1, true)
	v.bindKeys()
	v.refresh()
	return v
}

// Name returns the view name.
func (v *PipelineNodeLogsView) Name() string {
	if v.stepID != "" {
		return fmt.Sprintf("StepLog[%s#%d/%s/%s]", v.jobName, v.buildNum, v.node.DisplayName, v.stepID)
	}
	return fmt.Sprintf("NodeLog[%s#%d/%s]", v.jobName, v.buildNum, v.node.DisplayName)
}

// Hints returns the menu hints.
func (v *PipelineNodeLogsView) Hints() model.MenuHints { return v.actions.Hints() }

// SetFilter forwards to the inner text view.
func (v *PipelineNodeLogsView) SetFilter(filter string) { v.textView.SetFilter(filter) }

// GetJenkinsURL returns the Blue Ocean URL for this node/step.
func (v *PipelineNodeLogsView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns pipeline/<jobPath>/<buildNum>/<nodeID>[/<stepID>].
func (v *PipelineNodeLogsView) GetViewPath() string {
	if v.stepID != "" {
		return fmt.Sprintf("pipeline/%s/%d/%s/%s", v.jobName, v.buildNum, v.node.ID, v.stepID)
	}
	return fmt.Sprintf("pipeline/%s/%d/%s", v.jobName, v.buildNum, v.node.ID)
}

// GetParentID returns the node or step ID for selection restoration in
// the parent view.
func (v *PipelineNodeLogsView) GetParentID() string {
	if v.stepID != "" {
		return v.stepID
	}
	return v.node.ID
}

func (v *PipelineNodeLogsView) styledTitle() string {
	if v.stepID != "" {
		return fmt.Sprintf(" [aqua::b]%s[-::-] / [white]step %s[-] ", v.node.DisplayName, v.stepID)
	}
	return fmt.Sprintf(" [aqua::b]%s[-::-] ", v.node.DisplayName)
}

func (v *PipelineNodeLogsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)
	v.actions.Bulk(ui.KeyMap{
		ui.KeyG:      ui.NewKeyAction("Top", v.topCmd, true),
		ui.KeyShiftG: ui.NewKeyAction("Bottom", v.bottomCmd, true),
		ui.KeyR:      ui.NewKeyAction("Refresh", v.refreshCmd, true),
		tcell.KeyEsc: ui.NewKeyAction("Back", v.backCmd, true),
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

func (v *PipelineNodeLogsView) refresh() {
	if v.app.Client() == nil {
		v.app.Flash().Err(fmt.Errorf("not connected to Jenkins"))
		return
	}
	go func() {
		var (
			log client.BlueLog
			err error
		)
		if v.stepID != "" {
			log, err = v.app.Client().GetPipelineStepLog(context.Background(), v.jobName, v.buildNum, v.node.ID, v.stepID, 0)
		} else {
			log, err = v.app.Client().GetPipelineNodeLog(context.Background(), v.jobName, v.buildNum, v.node.ID, 0)
		}
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
				return
			}
			v.renderLog(log.Text)
		})
	}()
}

// renderLog writes the log body to the inner text view. Exposed for
// unit tests.
func (v *PipelineNodeLogsView) renderLog(body string) {
	if strings.TrimSpace(body) == "" {
		v.textView.SetContentWithColors("[gray]<no log output>[white]")
		return
	}
	v.textView.SetContentWithColors(tview.Escape(body))
}

func (v *PipelineNodeLogsView) topCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToTop()
	return nil
}

func (v *PipelineNodeLogsView) bottomCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToBottom()
	return nil
}

func (v *PipelineNodeLogsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

func (v *PipelineNodeLogsView) backCmd(*tcell.EventKey) *tcell.EventKey {
	if v.app.Content.CanPop() {
		v.app.Content.Pop()
	}
	return nil
}

// -----------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------

// colorizeBlueResult renders the textual STATUS cell. Falls back to a
// muted "—" for nodes that haven't run yet.
func colorizeBlueResult(result, state string) string {
	if state == "RUNNING" {
		return "[#268bd2::b]running[-::-]"
	}
	switch result {
	case "SUCCESS":
		return "[#859900::b]success[-::-]"
	case "FAILURE":
		return "[#dc322f::b]failed[-::-]"
	case "UNSTABLE":
		return "[#b58900::b]unstable[-::-]"
	case "ABORTED":
		return "[#586e75::b]aborted[-::-]"
	case "NOT_BUILT":
		return "[#93a1a1]not built[-]"
	case "":
		return "[gray]—[-]"
	default:
		return result
	}
}

// formatBlueDuration renders a node's elapsed time. Blue Ocean exposes
// DurationInMillis even for in-progress nodes (as "elapsed so far").
func formatBlueDuration(n client.BlueNode) string {
	if n.DurationInMillis <= 0 {
		return "-"
	}
	return formatDuration(time.Duration(n.DurationInMillis) * time.Millisecond)
}

// formatBlueDurationStep is the BlueStep equivalent of formatBlueDuration.
func formatBlueDurationStep(s client.BlueStep) string {
	if s.DurationInMillis <= 0 {
		return "-"
	}
	return formatDuration(time.Duration(s.DurationInMillis) * time.Millisecond)
}
