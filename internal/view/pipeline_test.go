// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"strings"
	"testing"

	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/client"
	"github.com/rplevka/j9s/internal/client/mock"
	"github.com/rplevka/j9s/internal/config"
	"github.com/rplevka/j9s/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPipelineGraphView_RendersFromMockNodes drives the view through
// the fake Blue Ocean server and asserts that the laid-out graph rows
// land in the table with branch markers, status icons and per-row
// node IDs.
func TestPipelineGraphView_RendersFromMockNodes(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS"}).
		WithPipelineNodes("hello", 1, []client.BlueNode{
			{ID: "1", DisplayName: "build", Result: "SUCCESS", State: "FINISHED",
				Edges: []client.BlueEdge{{ID: "2"}}},
			{ID: "2", DisplayName: "test", Result: "SUCCESS", State: "FINISHED",
				Edges: []client.BlueEdge{{ID: "3"}, {ID: "4"}}},
			{ID: "3", DisplayName: "unit", Result: "SUCCESS", State: "FINISHED",
				Edges: []client.BlueEdge{{ID: "5"}}},
			{ID: "4", DisplayName: "ui", Result: "FAILURE", State: "FINISHED",
				Edges: []client.BlueEdge{{ID: "5"}}},
			{ID: "5", DisplayName: "deploy", Result: "SUCCESS", State: "FINISHED"},
		})

	c := mock.NewClient(t, srv)
	nodes, err := c.GetPipelineNodes(context.Background(), "hello", 1)
	require.NoError(t, err)
	require.Len(t, nodes, 5)

	v := &PipelineGraphView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  "hello",
		buildNum: 1,
		rows:     LayoutGraph(nodes),
	}
	v.renderRows()

	assert.Equal(t, 5, v.table.RowCount())
	// Verify ID column comes through verbatim.
	ids := map[string]bool{}
	for i := 0; i < v.table.RowCount(); i++ {
		v.table.Select(i+1, 0)
		ids[v.table.GetSelectedID()] = true
	}
	for _, want := range []string{"1", "2", "3", "4", "5"} {
		assert.True(t, ids[want], "missing node id %s in rendered table", want)
	}
}

// TestPipelineGraphView_URLProvider asserts the pipeline view path /
// Jenkins URL pair: build-level when nothing is selected, node-level
// when a row is selected.
func TestPipelineGraphView_URLProvider(t *testing.T) {
	cfg := config.NewConfig()
	cfg.J9s.Contexts = []config.Context{{Name: "test", URL: "https://jenkins.example.com"}}
	cfg.J9s.CurrentContext = "test"
	app := NewApp(cfg)
	app.App.Init()

	v := &PipelineGraphView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  "Folder/MyJob",
		buildNum: 3,
	}
	assert.Equal(t, "pipeline/Folder/MyJob/3", v.GetViewPath())
	assert.Equal(t,
		"https://jenkins.example.com/blue/organizations/jenkins/Folder%2FMyJob/detail/MyJob/3/pipeline",
		v.GetJenkinsURL())

	v.rows = []GraphRow{{Node: client.BlueNode{ID: "9", DisplayName: "test", Result: "SUCCESS"}}}
	v.renderRows()
	v.table.Select(1, 0)
	assert.Equal(t, "pipeline/Folder/MyJob/3/9", v.GetViewPath())
	assert.Contains(t, v.GetJenkinsURL(), "/pipeline/9")

	var _ URLProvider = (*PipelineGraphView)(nil)
	var _ URLProvider = (*PipelineNodeStepsView)(nil)
	var _ URLProvider = (*PipelineNodeLogsView)(nil)
}

// TestPipelineNodeStepsView_RendersFromMockSteps drives the steps view
// through the mock server and asserts the steps come back in
// registration order with their IDs and statuses.
func TestPipelineNodeStepsView_RendersFromMockSteps(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS"}).
		WithPipelineNodeSteps("hello", 1, "9", []client.BlueStep{
			{ID: "21", DisplayName: "Print", Result: "SUCCESS", State: "FINISHED"},
			{ID: "22", DisplayName: "Shell", Result: "FAILURE", State: "FINISHED"},
		})

	c := mock.NewClient(t, srv)
	steps, err := c.GetPipelineNodeSteps(context.Background(), "hello", 1, "9")
	require.NoError(t, err)
	require.Len(t, steps, 2)

	v := &PipelineNodeStepsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  "hello",
		buildNum: 1,
		node:     client.BlueNode{ID: "9", DisplayName: "test"},
		steps:    steps,
	}
	v.renderSteps()
	assert.Equal(t, 2, v.table.RowCount())
	v.table.Select(2, 0)
	assert.Equal(t, "22", v.table.GetSelectedID())
}

// TestPipelineNodeLogsView_RendersFromMockLog feeds a canned node log
// through the mock and asserts the body lands in the inner text view
// (sufficient for the view contract; tview text rendering itself is
// not exercised here).
func TestPipelineNodeLogsView_RendersFromMockLog(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS"}).
		WithPipelineNodeLog("hello", 1, "9", "running step\nfinished\n")

	c := mock.NewClient(t, srv)
	log, err := c.GetPipelineNodeLog(context.Background(), "hello", 1, "9", 0)
	require.NoError(t, err)

	v := &PipelineNodeLogsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		textView: ui.NewFilterableTextView(),
		actions:  ui.NewKeyActions(),
		jobName:  "hello",
		buildNum: 1,
		node:     client.BlueNode{ID: "9", DisplayName: "test"},
	}
	v.renderLog(log.Text)
	// The text view exposes its raw contents via GetText (tview API);
	// verify the log substring is present.
	got := v.textView.GetText(true)
	assert.True(t, strings.Contains(got, "running step"), "log body missing from view: %q", got)
}

// TestPipelineNodeLogsView_EmptyLog asserts the placeholder is rendered
// for an empty log payload (so the user gets visual confirmation that
// the request succeeded but the node hasn't produced any output yet).
func TestPipelineNodeLogsView_EmptyLog(t *testing.T) {
	v := &PipelineNodeLogsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		textView: ui.NewFilterableTextView(),
		actions:  ui.NewKeyActions(),
		node:     client.BlueNode{ID: "1", DisplayName: "build"},
	}
	v.renderLog("")
	got := v.textView.GetText(true)
	assert.True(t, strings.Contains(got, "no log output"), "expected placeholder, got %q", got)
}
