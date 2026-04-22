// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// BuildsView displays Jenkins builds for a job.
type BuildsView struct {
	*tview.Flex
	app              *App
	table            *ui.Table
	actions          *ui.KeyActions
	jobName          string
	autoRefresh      *time.Ticker
	stopRefresh      chan struct{}
	pendingSelection string // ID to select after data loads
}

// NewBuildsView returns a new builds view.
func NewBuildsView(app *App, jobName string) *BuildsView {
	v := &BuildsView{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		app:         app,
		table:       ui.NewTable(),
		actions:     ui.NewKeyActions(),
		jobName:     jobName,
		stopRefresh: make(chan struct{}),
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	v.startAutoRefresh()
	return v
}

// Name returns the view name.
func (v *BuildsView) Name() string {
	if v.jobName != "" {
		return fmt.Sprintf("Builds[%s]", v.jobName)
	}
	return "Builds"
}

// Hints returns the view hints.
func (v *BuildsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

// GetJenkinsURL returns the Jenkins web UI URL for this view.
func (v *BuildsView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns the internal view path for bookmarking.
func (v *BuildsView) GetViewPath() string {
	if v.jobName != "" {
		return "builds/" + v.jobName
	}
	return "builds"
}

// GetParentID returns the job name this builds view was opened from.
// This is used to restore selection when navigating back.
// Returns just the job name (without folder path) to match the jobs table.
func (v *BuildsView) GetParentID() string {
	// Extract just the job name from full path (e.g., "folder/job-name" -> "job-name")
	if idx := strings.LastIndex(v.jobName, "/"); idx >= 0 {
		return v.jobName[idx+1:]
	}
	return v.jobName
}

// SelectByID selects the build with the given number (e.g., "#123").
// Implements the Selectable interface for selection restoration.
// If data hasn't loaded yet, stores the ID for selection after load.
func (v *BuildsView) SelectByID(id string) bool {
	if v.table.SelectByID(id) {
		return true
	}
	// Store for later if data not loaded yet
	v.pendingSelection = id
	return false
}

func (v *BuildsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Logs", v.logsCmd, true),
		ui.KeyL:        ui.NewKeyAction("Logs", v.logsCmd, true),
		ui.KeyD:        ui.NewKeyAction("Describe", v.describeCmd, true),
		ui.KeyA:        ui.NewKeyAction("Artifacts", v.artifactsCmd, true),
		ui.KeyS:        ui.NewKeyAction("Stop", v.stopCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
		ui.KeyB:        ui.NewKeyAction("Rebuild", v.rebuildCmd, true),
		// Sorting shortcuts
		ui.KeyShiftN: ui.NewKeyAction("Sort Number", v.sortByNumberCmd, true),
		ui.KeyShiftR: ui.NewKeyAction("Sort Result", v.sortByResultCmd, true),
		ui.KeyShiftA: ui.NewKeyAction("Sort Age", v.sortByAgeCmd, true),
	})

	// Set input capture on the Flex (view) itself, not just the table
	// This ensures key events are captured even when focus is on the Flex
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

func (v *BuildsView) refresh() {
	if v.app.Client() == nil || v.jobName == "" {
		return
	}

	go func() {
		builds, err := v.app.Client().GetBuilds(context.Background(), v.jobName)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			v.renderBuilds(builds)
		})
	}()
}

func (v *BuildsView) renderBuilds(builds []client.Build) {
	v.table.SetHeaders([]string{"NUMBER", "RESULT", "DURATION", "AGE"})

	rows := make([][]string, 0, len(builds))
	for _, build := range builds {
		result := build.Result
		if build.Building {
			result = "BUILDING"
		}

		duration := "-"
		if build.Building && build.Timestamp > 0 {
			// Show elapsed time in yellow/orange for running builds
			elapsed := time.Since(time.Unix(build.Timestamp/1000, 0))
			duration = fmt.Sprintf("[yellow::b]%s[-::-]", formatDuration(elapsed))
		} else if build.Duration > 0 {
			duration = formatDuration(time.Duration(build.Duration) * time.Millisecond)
		}

		age := "-"
		if build.Timestamp > 0 {
			age = formatAge(time.Unix(build.Timestamp/1000, 0))
		}

		rows = append(rows, []string{
			fmt.Sprintf("#%d", build.Number),
			colorizeResult(result),
			duration,
			age,
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle("Builds:" + v.jobName)
	v.table.Refresh()

	// Apply pending selection if any (e.g., from bookmark navigation)
	if v.pendingSelection != "" {
		v.table.SelectByID(v.pendingSelection)
		v.pendingSelection = "" // Clear after applying
	}
}

func (v *BuildsView) logsCmd(*tcell.EventKey) *tcell.EventKey {
	buildNum := v.getSelectedBuildNumber()
	if buildNum <= 0 {
		return nil
	}
	logsView := NewLogsView(v.app, v.jobName, buildNum)
	v.app.Content.Push(logsView)
	return nil
}

func (v *BuildsView) describeCmd(*tcell.EventKey) *tcell.EventKey {
	buildNum := v.getSelectedBuildNumber()
	if buildNum <= 0 {
		return nil
	}
	descView := NewDescribeView(v.app, "build", fmt.Sprintf("%s#%d", v.jobName, buildNum))
	v.app.Content.Push(descView)
	return nil
}

func (v *BuildsView) artifactsCmd(*tcell.EventKey) *tcell.EventKey {
	buildNum := v.getSelectedBuildNumber()
	if buildNum <= 0 {
		return nil
	}
	artifactsView := NewArtifactsView(v.app, v.jobName, buildNum)
	v.app.Content.Push(artifactsView)
	return nil
}

func (v *BuildsView) stopCmd(*tcell.EventKey) *tcell.EventKey {
	buildNum := v.getSelectedBuildNumber()
	if buildNum <= 0 {
		return nil
	}

	go func() {
		err := v.app.Client().StopBuild(context.Background(), v.jobName, buildNum)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
			} else {
				v.app.Flash().Info(fmt.Sprintf("Build #%d stopped", buildNum))
			}
			v.refresh()
		})
	}()
	return nil
}

func (v *BuildsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

func (v *BuildsView) rebuildCmd(*tcell.EventKey) *tcell.EventKey {
	if v.jobName == "" {
		v.app.Flash().Warn("No job selected")
		return nil
	}

	// Show parameter form with last build's params pre-filled
	ShowParamsForm(v.app, v.jobName, true)
	return nil
}

func (v *BuildsView) getSelectedBuildNumber() int {
	item := v.table.GetSelectedItem()
	if len(item) == 0 {
		return 0
	}
	// Parse "#123" format
	numStr := item[0]
	if len(numStr) > 1 && numStr[0] == '#' {
		numStr = numStr[1:]
	}
	num, _ := strconv.Atoi(numStr)
	return num
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// startAutoRefresh starts the auto-refresh timer.
func (v *BuildsView) startAutoRefresh() {
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
func (v *BuildsView) Stop() {
	if v.autoRefresh != nil {
		v.autoRefresh.Stop()
		v.autoRefresh = nil
	}
	if v.stopRefresh != nil {
		close(v.stopRefresh)
		v.stopRefresh = nil
	}
}

// Sorting commands - columns: NUMBER(0), RESULT(1), DURATION(2), AGE(3)
func (v *BuildsView) sortByNumberCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(0)
	v.table.Refresh()
	return nil
}

func (v *BuildsView) sortByResultCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(1)
	v.table.Refresh()
	return nil
}

func (v *BuildsView) sortByAgeCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(3)
	v.table.Refresh()
	return nil
}
