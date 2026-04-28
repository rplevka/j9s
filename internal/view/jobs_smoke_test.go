// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"testing"

	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client/mock"
	"github.com/roman-plevka/j9s/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobsView_RendersTopLevelJobs is a smoke test wiring the fake Jenkins
// server through the real client and into a hand-constructed JobsView (the
// goroutines spawned by NewJobsViewWithPath would require a running
// tview.Application; this test exercises just the data-fetch + render path
// to prove the harness works end to end).
func TestJobsView_RendersTopLevelJobs(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{Color: "blue", LastBuildNumber: 1, LastBuildResult: "SUCCESS"}).
		WithJob("world", mock.JobOpts{Color: "red", LastBuildNumber: 2, LastBuildResult: "FAILURE"}).
		WithFolder("team-a")

	c := mock.NewClient(t, srv)
	jobs, err := c.GetJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 3)

	v := &JobsView{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		table:   ui.NewTable(),
		actions: ui.NewKeyActions(),
		jobs:    jobs,
	}
	v.renderJobs(jobs)

	assert.Equal(t, 3, v.table.RowCount(), "two jobs + one folder")

	// Spot-check that the job names made it into the first column.
	names := map[string]bool{}
	for i := 0; i < v.table.RowCount(); i++ {
		v.table.Select(i+1, 0)
		names[v.table.GetSelectedID()] = true
	}
	assert.True(t, names["hello"])
	assert.True(t, names["world"])
	assert.True(t, names["team-a"])
}

// TestChooseToggleAction encodes the contract of the merged enable/disable
// hotkey: a buildable job toggles to "disable", a non-buildable one toggles
// to "enable".
func TestChooseToggleAction(t *testing.T) {
	cases := []struct {
		buildable bool
		want      string
	}{
		{buildable: true, want: "disable"},
		{buildable: false, want: "enable"},
	}
	for _, tc := range cases {
		if got := chooseToggleAction(tc.buildable); got != tc.want {
			t.Errorf("chooseToggleAction(%v) = %q, want %q", tc.buildable, got, tc.want)
		}
	}
}

// TestJobsView_RendersFolderContents covers the nested-folder navigation
// path: a JobsView pinned to "team-a" pulls the folder listing via
// GetFolderJobs and renders only the children of that folder.
func TestJobsView_RendersFolderContents(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithFolder("team-a").
		WithJobInFolder("team-a", "deploy", mock.JobOpts{Color: "blue"}).
		WithFolderInFolder("team-a", "sub").
		WithJobInFolder("team-a/sub", "nightly", mock.JobOpts{Color: "yellow"})

	c := mock.NewClient(t, srv)

	teamA, err := c.GetFolderJobs(context.Background(), "team-a")
	require.NoError(t, err)
	assert.Len(t, teamA, 2)

	v := &JobsView{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		table:      ui.NewTable(),
		actions:    ui.NewKeyActions(),
		folderPath: "team-a",
		jobs:       teamA,
	}
	v.renderJobs(teamA)
	assert.Equal(t, 2, v.table.RowCount())

	sub, err := c.GetFolderJobs(context.Background(), "team-a/sub")
	require.NoError(t, err)
	require.Len(t, sub, 1)
	assert.Equal(t, "nightly", sub[0].Name)
}
