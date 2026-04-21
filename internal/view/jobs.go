// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"regexp"
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
	app        *App
	table      *ui.Table
	actions    *ui.KeyActions
	folderPath string       // Current folder path (empty for root)
	jobs       []client.Job // Current jobs list (for filtering)
	filter     string       // Current filter
}

// NewJobsView returns a new jobs view.
func NewJobsView(app *App) *JobsView {
	return NewJobsViewWithPath(app, "")
}

// NewJobsViewWithPath returns a new jobs view for a specific folder.
func NewJobsViewWithPath(app *App, folderPath string) *JobsView {
	v := &JobsView{
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

// SetFilter sets the filter and re-renders the jobs.
func (v *JobsView) SetFilter(filter string) {
	v.filter = filter
	v.table.SetHighlight(filter) // Set highlight for matching text
	v.renderJobs(v.filterJobs(v.jobs))
}

// GetFilter returns the current filter.
func (v *JobsView) GetFilter() string {
	return v.filter
}

func (v *JobsView) bindKeys() {
	// Add global keys first
	AddGlobalKeys(v.app, v.actions)

	// Add view-specific keys
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Builds", v.enterCmd, true),
		ui.KeyD:        ui.NewKeyAction("Describe", v.describeCmd, true),
		ui.KeyT:        ui.NewKeyAction("Trigger", v.triggerCmd, true),
		ui.KeyE:        ui.NewKeyAction("Enable", v.enableCmd, true),
		ui.KeyShiftD:   ui.NewKeyAction("Disable", v.disableCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
		tcell.KeyCtrlD: ui.NewKeyAction("Delete", v.deleteCmd, true),
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
			v.renderJobs(v.filterJobs(jobs))
		})
	}()
}

func (v *JobsView) filterJobs(jobs []client.Job) []client.Job {
	if v.filter == "" {
		return jobs
	}

	// Try to compile as regex, fall back to substring match
	rx, err := regexp.Compile("(?i)" + v.filter)
	useRegex := err == nil

	filtered := make([]client.Job, 0)
	for _, job := range jobs {
		var matches bool
		if useRegex {
			matches = rx.MatchString(job.Name)
		} else {
			matches = containsIgnoreCase(job.Name, v.filter)
		}
		if matches {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(substr == "" ||
			len(s) > 0 && containsLower(toLower(s), toLower(substr)))
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
			status = "[aqua::b]📁[-::-]"
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
	v.table.Refresh()
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

	go func() {
		err := v.app.Client().TriggerBuild(context.Background(), fullJobName, nil)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
			} else {
				v.app.Flash().Info(fmt.Sprintf("Build triggered for %s", fullJobName))
			}
			v.refresh()
		})
	}()
	return nil
}

func (v *JobsView) enableCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	// Include folder path for nested jobs
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	go func() {
		err := v.app.Client().EnableJob(context.Background(), fullJobName)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
			} else {
				v.app.Flash().Info(fmt.Sprintf("Job %s enabled", fullJobName))
			}
			v.refresh()
		})
	}()
	return nil
}

func (v *JobsView) disableCmd(*tcell.EventKey) *tcell.EventKey {
	jobName := v.table.GetSelectedID()
	if jobName == "" {
		return nil
	}
	// Include folder path for nested jobs
	fullJobName := jobName
	if v.folderPath != "" {
		fullJobName = v.folderPath + "/" + jobName
	}

	go func() {
		err := v.app.Client().DisableJob(context.Background(), fullJobName)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
			} else {
				v.app.Flash().Info(fmt.Sprintf("Job %s disabled", fullJobName))
			}
			v.refresh()
		})
	}()
	return nil
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
		return "[green::b]●[-::-]"
	case "red", "red_anime":
		return "[red::b]●[-::-]"
	case "yellow", "yellow_anime":
		return "[yellow::b]●[-::-]"
	case "disabled", "disabled_anime":
		return "[gray::b]●[-::-]"
	case "notbuilt", "notbuilt_anime":
		return "[white::b]○[-::-]"
	case "aborted", "aborted_anime":
		return "[gray::b]◌[-::-]"
	default:
		return "[white::b]?[-::-]"
	}
}

func colorizeResult(result string) string {
	switch result {
	case "SUCCESS":
		return "[green::b]SUCCESS[-::-]"
	case "FAILURE":
		return "[red::b]FAILURE[-::-]"
	case "UNSTABLE":
		return "[yellow::b]UNSTABLE[-::-]"
	case "ABORTED":
		return "[gray::b]ABORTED[-::-]"
	case "BUILDING":
		return "[aqua::b]BUILDING[-::-]"
	case "NOT_BUILT":
		return "[white::-]NOT_BUILT[-::-]"
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
