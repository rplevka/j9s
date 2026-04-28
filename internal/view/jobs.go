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

// JobsView displays Jenkins jobs.
type JobsView struct {
	*tview.Flex
	app              *App
	table            *ui.Table
	actions          *ui.KeyActions
	folderPath       string       // Current folder path (empty for root)
	jobs             []client.Job // Current jobs list
	autoRefresh      *time.Ticker
	stopRefresh      chan struct{}
	pendingSelection string // ID to select after data loads
}

// NewJobsView returns a new jobs view.
func NewJobsView(app *App) *JobsView {
	return NewJobsViewWithPath(app, "")
}

// NewJobsViewWithPath returns a new jobs view for a specific folder.
func NewJobsViewWithPath(app *App, folderPath string) *JobsView {
	v := &JobsView{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		app:         app,
		table:       ui.NewTable(),
		actions:     ui.NewKeyActions(),
		folderPath:  folderPath,
		stopRefresh: make(chan struct{}),
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	v.startAutoRefresh()

	return v
}

// Table returns the table for focus management.
func (v *JobsView) Table() *ui.Table {
	return v.table
}

// Name returns the view name.
func (v *JobsView) Name() string {
	if v.folderPath != "" {
		return "Jobs:" + v.folderPath
	}
	return "Jobs"
}

// Hints returns the view hints.
func (v *JobsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

// SetFilter sets the filter using the table's built-in filtering.
func (v *JobsView) SetFilter(filter string) {
	v.table.Filter(filter)
}

// GetFilter returns the current filter.
func (v *JobsView) GetFilter() string {
	return v.table.GetFilter()
}

// GetJenkinsURL returns the Jenkins web UI URL for this view.
func (v *JobsView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns the internal view path for bookmarking.
func (v *JobsView) GetViewPath() string {
	if v.folderPath != "" {
		return "jobs/" + v.folderPath
	}
	return "jobs"
}

// SelectByID selects the job with the given name.
// Implements the Selectable interface for selection restoration.
// If data hasn't loaded yet, stores the ID for selection after load.
func (v *JobsView) SelectByID(id string) bool {
	// Try to select immediately
	if v.table.SelectByID(id) {
		return true
	}
	// If not found, store for later (data might not be loaded yet)
	v.pendingSelection = id
	return false
}

// GetSelectedID returns the currently selected job name.
func (v *JobsView) GetSelectedID() string {
	return v.table.GetSelectedID()
}

// IDs returns the displayed job/folder names. Implements IDProvider so the
// command prompt can offer them as argument suggestions.
func (v *JobsView) IDs() []string {
	return v.table.GetRowIDs()
}

func (v *JobsView) bindKeys() {
	// Add global keys first
	AddGlobalKeys(v.app, v.actions)

	// Add view-specific keys
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Builds", v.enterCmd, true),
		ui.KeyD:        ui.NewKeyAction("Describe", v.describeCmd, true),
		ui.KeyA:        ui.NewKeyAction("Artifacts", v.artifactsCmd, true),
		ui.KeyL:        ui.NewKeyAction("Logs", v.logsCmd, true),
		ui.KeyV:        ui.NewKeyAction("Views", v.viewsCmd, true),
		ui.KeyB:        ui.NewKeyAction("Build", v.triggerCmd, true),
		ui.KeyShiftE:   ui.NewKeyAction("Toggle", v.toggleEnabledCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
		tcell.KeyCtrlD: ui.NewKeyAction("Delete", v.deleteCmd, true),
		// Sorting shortcuts
		ui.KeyShiftN: ui.NewKeyAction("Sort Name", v.sortByNameCmd, true),
		ui.KeyShiftS: ui.NewKeyAction("Sort Status", v.sortByStatusCmd, true),
		ui.KeyShiftA: ui.NewKeyAction("Sort Age", v.sortByAgeCmd, true),
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

func (v *JobsView) refresh() {
	if v.app.Client() == nil {
		v.app.Flash().Err(fmt.Errorf("not connected to Jenkins"))
		return
	}

	go func() {
		var jobs []client.Job
		var err error

		if v.folderPath == "" {
			jobs, err = v.app.Client().GetJobs(context.Background())
		} else {
			jobs, err = v.app.Client().GetFolderJobs(context.Background(), v.folderPath)
		}

		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.jobs = jobs
			v.renderJobs(jobs)
		})
	}()
}

func (v *JobsView) renderJobs(jobs []client.Job) {
	v.table.SetHeaders([]string{"NAME", "TYPE", "STATUS", "HEALTH", "LAST BUILD", "RESULT", "AGE"})

	rows := make([][]string, 0, len(jobs))
	for _, job := range jobs {
		jobType := "Job"
		if job.IsFolder() {
			jobType = "[aqua::b]Folder[-::-]"
		}

		status := colorizeStatus(job.Color)
		health := "-"
		if len(job.HealthReport) > 0 {
			health = fmt.Sprintf("%d%%", job.HealthReport[0].Score)
		}

		lastBuild := "-"
		result := "-"
		age := "-"
		if job.LastBuild != nil {
			lastBuild = fmt.Sprintf("#%d", job.LastBuild.Number)
			result = job.LastBuild.Result
			if job.LastBuild.Building {
				result = "BUILDING"
			}
			if job.LastBuild.Timestamp > 0 {
				age = formatAge(time.Unix(job.LastBuild.Timestamp/1000, 0))
			}
		}

		// For folders, show different info
		if job.IsFolder() {
			status = "-"
			health = "-"
			lastBuild = "-"
			result = "-"
			age = "-"
		}

		rows = append(rows, []string{
			job.Name,
			jobType,
			status,
			health,
			lastBuild,
			colorizeResult(result),
			age,
		})
	}

	v.table.SetData(rows)
	// Set title with folder path if in a subfolder
	title := "Jobs"
	if v.folderPath != "" {
		title = "Jobs:" + v.folderPath
	}
	v.table.SetTitle(title)
	v.table.Refresh()

	// Apply pending selection if any (e.g., from bookmark navigation)
	if v.pendingSelection != "" {
		v.table.SelectByID(v.pendingSelection)
		v.pendingSelection = "" // Clear after applying
	}
}

func (v *JobsView) enterCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}

	// Find the selected job to check if it's a folder
	var selectedJob *client.Job
	for i := range v.jobs {
		if v.jobs[i].Name == jobName {
			selectedJob = &v.jobs[i]
			break
		}
	}

	if selectedJob != nil && selectedJob.IsFolder() {
		// Navigate into the folder
		var newPath string
		if v.folderPath == "" {
			newPath = jobName
		} else {
			newPath = v.folderPath + "/" + jobName
		}
		folderView := NewJobsViewWithPath(v.app, newPath)
		v.app.Content.Push(folderView)
	} else {
		// Show builds for the job
		var fullJobName string
		if v.folderPath == "" {
			fullJobName = jobName
		} else {
			fullJobName = v.folderPath + "/" + jobName
		}
		buildsView := NewBuildsView(v.app, fullJobName)
		v.app.Content.Push(buildsView)
	}
	return nil
}

func (v *JobsView) describeCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	// Include folder path for nested jobs
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}
	descView := NewDescribeView(v.app, "job", fullJobName)
	v.app.Content.Push(descView)
	return nil
}

func (v *JobsView) artifactsCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	// Include folder path for nested jobs
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	// Find the selected job to get last build number
	var selectedJob *client.Job
	for i := range v.jobs {
		if v.jobs[i].Name == jobName {
			selectedJob = &v.jobs[i]
			break
		}
	}
	if selectedJob == nil || selectedJob.LastBuild == nil {
		v.app.Flash().Warn("No builds available for this job")
		return nil
	}

	artifactsView := NewArtifactsView(v.app, fullJobName, selectedJob.LastBuild.Number)
	v.app.Content.Push(artifactsView)
	return nil
}

func (v *JobsView) logsCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	// Include folder path for nested jobs
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	// Find the selected job to get last build number
	var selectedJob *client.Job
	for i := range v.jobs {
		if v.jobs[i].Name == jobName {
			selectedJob = &v.jobs[i]
			break
		}
	}
	if selectedJob == nil || selectedJob.LastBuild == nil {
		v.app.Flash().Warn("No builds available for this job")
		return nil
	}

	logsView := NewLogsView(v.app, fullJobName, selectedJob.LastBuild.Number)
	v.app.Content.Push(logsView)
	return nil
}

func (v *JobsView) viewsCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}

	// Find the selected job to check if it's a folder
	var selectedJob *client.Job
	for i := range v.jobs {
		if v.jobs[i].Name == jobName {
			selectedJob = &v.jobs[i]
			break
		}
	}
	if selectedJob == nil {
		return nil
	}

	// Only folders have views
	if !selectedJob.IsFolder() {
		v.app.Flash().Warn("Only folders have views")
		return nil
	}

	// Include folder path for nested folders
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	viewsView := NewViewsViewWithPath(v.app, fullJobName)
	v.app.Content.Push(viewsView)
	return nil
}

func (v *JobsView) triggerCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	// Include folder path for nested jobs
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	// Show parameter form (handles both parameterized and non-parameterized jobs)
	ShowParamsForm(v.app, fullJobName, false)
	return nil
}

// toggleEnabledCmd flips the enabled state of the selected job: a
// Buildable job is disabled, a non-Buildable one is enabled. Folders are
// skipped since they are not directly buildable.
func (v *JobsView) toggleEnabledCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	var selected *client.Job
	for i := range v.jobs {
		if v.jobs[i].Name == jobName {
			selected = &v.jobs[i]
			break
		}
	}
	if selected == nil {
		return nil
	}
	if selected.IsFolder() {
		v.app.Flash().Warn("Folders cannot be enabled or disabled")
		return nil
	}

	action := chooseToggleAction(selected.Buildable)
	go func() {
		var err error
		if action == "disable" {
			err = v.app.Client().DisableJob(context.Background(), fullJobName)
		} else {
			err = v.app.Client().EnableJob(context.Background(), fullJobName)
		}
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
			} else {
				v.app.Flash().Info(fmt.Sprintf("Job %s %sd", fullJobName, action))
			}
			v.refresh()
		})
	}()
	return nil
}

// chooseToggleAction picks the verb to send to Jenkins based on the job's
// current Buildable flag.
func chooseToggleAction(buildable bool) string {
	if buildable {
		return "disable"
	}
	return "enable"
}

func (v *JobsView) deleteCmd(*tcell.EventKey) *tcell.EventKey {
	if v.app.IsReadOnly() {
		v.app.Flash().Warn("Read-only mode")
		return nil
	}

	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	// Include folder path for nested jobs
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	// TODO: Add confirmation dialog
	go func() {
		err := v.app.Client().DeleteJob(context.Background(), fullJobName)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
			} else {
				v.app.Flash().Info(fmt.Sprintf("Job %s deleted", fullJobName))
			}
			v.refresh()
		})
	}()
	return nil
}

func (v *JobsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

func colorizeStatus(color string) string {
	switch color {
	case "blue", "blue_anime":
		return "[#859900::b]●[-::-]" // solarized green
	case "red", "red_anime":
		return "[#dc322f::b]●[-::-]" // solarized red
	case "yellow", "yellow_anime":
		return "[#b58900::b]●[-::-]" // solarized yellow
	case "disabled", "disabled_anime":
		return "[#586e75::b]●[-::-]" // solarized base01
	case "notbuilt", "notbuilt_anime":
		return "[#93a1a1::b]○[-::-]" // solarized base1
	case "aborted", "aborted_anime":
		return "[#586e75::b]◌[-::-]" // solarized base01
	default:
		return "[#93a1a1::b]?[-::-]" // solarized base1
	}
}

func colorizeResult(result string) string {
	switch result {
	case "SUCCESS":
		return "[#859900::b]SUCCESS[-::-]" // solarized green
	case "FAILURE":
		return "[#dc322f::b]FAILURE[-::-]" // solarized red
	case "UNSTABLE":
		return "[#b58900::b]UNSTABLE[-::-]" // solarized yellow
	case "ABORTED":
		return "[#586e75::b]ABORTED[-::-]" // solarized base01
	case "BUILDING":
		return "[#268bd2::b]BUILDING[-::-]" // solarized blue
	case "NOT_BUILT":
		return "[#93a1a1::-]NOT_BUILT[-::-]" // solarized base1
	default:
		return result
	}
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// startAutoRefresh starts the auto-refresh timer.
func (v *JobsView) startAutoRefresh() {
	rate := float32(2) // default 2 seconds
	if v.app.Config() != nil && v.app.Config().J9s.RefreshRate > 0 {
		rate = v.app.Config().J9s.RefreshRate
	}

	v.autoRefresh = time.NewTicker(time.Duration(rate) * time.Second)

	go func() {
		for {
			select {
			case <-v.stopRefresh:
				return
			case <-v.autoRefresh.C:
				v.refresh()
			}
		}
	}()
}

// Stop stops the auto-refresh timer.
func (v *JobsView) Stop() {
	if v.autoRefresh != nil {
		v.autoRefresh.Stop()
		v.autoRefresh = nil
	}
	if v.stopRefresh != nil {
		close(v.stopRefresh)
		v.stopRefresh = nil
	}
}

// Sorting commands - columns: NAME(0), TYPE(1), STATUS(2), HEALTH(3), LAST BUILD(4), RESULT(5), AGE(6)
func (v *JobsView) sortByNameCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(0)
	v.table.Refresh()
	return nil
}

func (v *JobsView) sortByStatusCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(2)
	v.table.Refresh()
	return nil
}

func (v *JobsView) sortByAgeCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(6)
	v.table.Refresh()
	return nil
}
