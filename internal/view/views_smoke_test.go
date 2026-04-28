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

func TestViewsView_ListsViews(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithView("", "all").
		WithView("", "broken")

	c := mock.NewClient(t, srv)
	views, err := c.GetViews(context.Background())
	require.NoError(t, err)
	assert.Len(t, views, 2)

	v := &ViewsView{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		table:   ui.NewTable(),
		actions: ui.NewKeyActions(),
		views:   views,
	}
	v.renderViews(views)
	assert.Equal(t, 2, v.table.RowCount())

	names := map[string]bool{}
	for i := 0; i < v.table.RowCount(); i++ {
		names[v.table.GetCell(i+1, 0).Text] = true
	}
	assert.True(t, names["all"])
	assert.True(t, names["broken"])
}

func TestViewJobsView_RendersJobsForView(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("alpha", mock.JobOpts{Color: "blue", LastBuildNumber: 7, LastBuildResult: "SUCCESS"}).
		WithJob("beta", mock.JobOpts{Color: "red", LastBuildNumber: 4, LastBuildResult: "FAILURE"}).
		WithView("", "all")

	c := mock.NewClient(t, srv)
	view, err := c.GetView(context.Background(), "all")
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Equal(t, "all", view.Name)
	assert.Len(t, view.Jobs, 2, "view should expose its jobs")

	v := &ViewJobsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		viewName: "all",
		jobs:     view.Jobs,
	}
	v.renderJobs(view.Jobs)
	assert.Equal(t, 2, v.table.RowCount())

	names := map[string]bool{}
	for i := 0; i < v.table.RowCount(); i++ {
		names[v.table.GetCell(i+1, 0).Text] = true
	}
	assert.True(t, names["alpha"])
	assert.True(t, names["beta"])
}
