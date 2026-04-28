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

// ViewsView displays Jenkins views for a folder or root.
type ViewsView struct {
	*tview.Flex
	app        *App
	table      *ui.Table
	actions    *ui.KeyActions
	folderPath string        // Folder path (empty for root)
	views      []client.View // Current views list
}

// NewViewsView returns a new views view for the root level.
func NewViewsView(app *App) *ViewsView {
	return NewViewsViewWithPath(app, "")
}

// NewViewsViewWithPath returns a new views view for a specific folder.
func NewViewsViewWithPath(app *App, folderPath string) *ViewsView {
	v := &ViewsView{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		app:        app,
		table:      ui.NewTable(),
		actions:    ui.NewKeyActions(),
		folderPath: folderPath,
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	return v
}

// IDs returns the displayed view names so the command prompt can offer
// them as argument suggestions.
func (v *ViewsView) IDs() []string {
	return v.table.GetRowIDs()
}

// Name returns the view name.
func (v *ViewsView) Name() string {
	if v.folderPath != "" {
		return fmt.Sprintf("Views[%s]", v.folderPath)
	}
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
		ui.KeyD:        ui.NewKeyAction("Describe", v.describeCmd, true),
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

func (v *ViewsView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		views, err := v.app.Client().GetFolderViews(context.Background(), v.folderPath)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.views = views
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

	// Set title based on folder path
	if v.folderPath != "" {
		v.table.SetTitle(fmt.Sprintf("Views:%s", v.folderPath))
	} else {
		v.table.SetTitle("Views")
	}
	v.table.Refresh()
}

func (v *ViewsView) enterCmd(*tcell.EventKey) *tcell.EventKey {
	viewName := v.table.GetSelectedID()
	if viewName == "" {
		return nil
	}
	jobsView := NewViewJobsView(v.app, v.folderPath, viewName)
	v.app.Content.Push(jobsView)
	return nil
}

func (v *ViewsView) describeCmd(*tcell.EventKey) *tcell.EventKey {
	viewName := v.table.GetSelectedID()
	if viewName == "" {
		return nil
	}
	descView := NewDescribeView(v.app, "view", viewName)
	v.app.Content.Push(descView)
	return nil
}

func (v *ViewsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

// ViewJobsView displays jobs filtered by a Jenkins view.
type ViewJobsView struct {
	*tview.Flex
	app         *App
	table       *ui.Table
	actions     *ui.KeyActions
	folderPath  string       // Folder containing the view (empty for root)
	viewName    string       // Name of the view
	jobs        []client.Job // Jobs in this view
	autoRefresh *time.Ticker
	stopRefresh chan struct{}
}

// NewViewJobsView returns a new view jobs view.
func NewViewJobsView(app *App, folderPath, viewName string) *ViewJobsView {
	v := &ViewJobsView{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		app:         app,
		table:       ui.NewTable(),
		actions:     ui.NewKeyActions(),
		folderPath:  folderPath,
		viewName:    viewName,
		stopRefresh: make(chan struct{}),
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	v.startAutoRefresh()
	return v
}

// IDs returns the displayed job names so the command prompt can offer
// them as argument suggestions.
func (v *ViewJobsView) IDs() []string {
	return v.table.GetRowIDs()
}

// Name returns the view name.
func (v *ViewJobsView) Name() string {
	if v.folderPath != "" {
		return fmt.Sprintf("Jobs[%s/%s]", v.folderPath, v.viewName)
	}
	return fmt.Sprintf("Jobs[%s]", v.viewName)
}

// Hints returns the view hints.
func (v *ViewJobsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

// SetFilter sets the filter.
func (v *ViewJobsView) SetFilter(filter string) {
	v.table.Filter(filter)
}

func (v *ViewJobsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Builds", v.buildsCmd, true),
		ui.KeyD:        ui.NewKeyAction("Describe", v.describeCmd, true),
		ui.KeyA:        ui.NewKeyAction("Artifacts", v.artifactsCmd, true),
		ui.KeyL:        ui.NewKeyAction("Logs", v.logsCmd, true),
		ui.KeyB:        ui.NewKeyAction("Build", v.triggerCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
		ui.KeyV:        ui.NewKeyAction("Views", v.viewsCmd, true),
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

func (v *ViewJobsView) startAutoRefresh() {
	v.autoRefresh = time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-v.autoRefresh.C:
				v.refresh()
			case <-v.stopRefresh:
				v.autoRefresh.Stop()
				return
			}
		}
	}()
}

func (v *ViewJobsView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		view, err := v.app.Client().GetFolderView(context.Background(), v.folderPath, v.viewName)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.jobs = view.Jobs
			v.renderJobs(view.Jobs)
		})
	}()
}

func (v *ViewJobsView) renderJobs(jobs []client.Job) {
	v.table.SetHeaders([]string{"NAME", "STATUS", "LAST BUILD", "HEALTH"})

	rows := make([][]string, 0, len(jobs))
	for _, job := range jobs {
		status := colorizeStatus(job.Color)
		lastBuild := "-"
		if job.LastBuild != nil {
			lastBuild = fmt.Sprintf("#%d", job.LastBuild.Number)
			if job.LastBuild.Building {
				lastBuild += " (building)"
			}
		}

		health := "-"
		if len(job.HealthReport) > 0 {
			health = fmt.Sprintf("%d%%", job.HealthReport[0].Score)
		}

		name := job.Name

		rows = append(rows, []string{
			name,
			status,
			lastBuild,
			health,
		})
	}

	v.table.SetData(rows)

	// Set title
	if v.folderPath != "" {
		v.table.SetTitle(fmt.Sprintf("Jobs:%s/%s", v.folderPath, v.viewName))
	} else {
		v.table.SetTitle(fmt.Sprintf("Jobs:%s", v.viewName))
	}
	v.table.Refresh()
}

func (v *ViewJobsView) getSelectedJob() *client.Job {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	for i := range v.jobs {
		if v.jobs[i].Name == jobName {
			return &v.jobs[i]
		}
	}
	return nil
}

func (v *ViewJobsView) getFullJobName(jobName string) string {
	if v.folderPath != "" {
		return v.folderPath + "/" + jobName
	}
	return jobName
}

func (v *ViewJobsView) buildsCmd(*tcell.EventKey) *tcell.EventKey {
	job := v.getSelectedJob()
	if job == nil {
		return nil
	}

	// Check if it's a folder - enter it
	if job.IsFolder() {
		folderPath := v.getFullJobName(job.Name)
		jobsView := NewJobsViewWithPath(v.app, folderPath)
		v.app.Content.Push(jobsView)
		return nil
	}

	buildsView := NewBuildsView(v.app, v.getFullJobName(job.Name))
	v.app.Content.Push(buildsView)
	return nil
}

func (v *ViewJobsView) describeCmd(*tcell.EventKey) *tcell.EventKey {
	job := v.getSelectedJob()
	if job == nil {
		return nil
	}
	descView := NewDescribeView(v.app, "job", v.getFullJobName(job.Name))
	v.app.Content.Push(descView)
	return nil
}

func (v *ViewJobsView) artifactsCmd(*tcell.EventKey) *tcell.EventKey {
	job := v.getSelectedJob()
	if job == nil || job.LastBuild == nil {
		v.app.Flash().Warn("No builds available for this job")
		return nil
	}
	artifactsView := NewArtifactsView(v.app, v.getFullJobName(job.Name), job.LastBuild.Number)
	v.app.Content.Push(artifactsView)
	return nil
}

func (v *ViewJobsView) logsCmd(*tcell.EventKey) *tcell.EventKey {
	job := v.getSelectedJob()
	if job == nil || job.LastBuild == nil {
		v.app.Flash().Warn("No builds available for this job")
		return nil
	}
	logsView := NewLogsView(v.app, v.getFullJobName(job.Name), job.LastBuild.Number)
	v.app.Content.Push(logsView)
	return nil
}

func (v *ViewJobsView) triggerCmd(*tcell.EventKey) *tcell.EventKey {
	job := v.getSelectedJob()
	if job == nil {
		return nil
	}
	ShowParamsForm(v.app, v.getFullJobName(job.Name), false)
	return nil
}

func (v *ViewJobsView) viewsCmd(*tcell.EventKey) *tcell.EventKey {
	job := v.getSelectedJob()
	if job == nil {
		return nil
	}

	// Only folders have views
	if !job.IsFolder() {
		v.app.Flash().Warn("Only folders have views")
		return nil
	}

	viewsView := NewViewsViewWithPath(v.app, v.getFullJobName(job.Name))
	v.app.Content.Push(viewsView)
	return nil
}

func (v *ViewJobsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}
