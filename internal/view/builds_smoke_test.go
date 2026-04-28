// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client/mock"
	"github.com/roman-plevka/j9s/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildsView_RendersMixedBuilds wires the mock Jenkins server through
// the real client into a hand-built BuildsView and verifies that completed
// and live ("building") builds render with the right markers.
func TestBuildsView_RendersMixedBuilds(t *testing.T) {
	now := time.Now().UnixMilli()
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{Color: "blue", LastBuildNumber: 3}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "FAILURE", Duration: 5000, Timestamp: now - 30000}).
		WithBuild("hello", 2, mock.BuildOpts{Result: "SUCCESS", Duration: 7500, Timestamp: now - 20000}).
		WithBuild("hello", 3, mock.BuildOpts{Building: true, Timestamp: now - 5000})

	c := mock.NewClient(t, srv)
	builds, err := c.GetBuilds(context.Background(), "hello")
	require.NoError(t, err)
	require.Len(t, builds, 3)

	v := &BuildsView{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		table:   ui.NewTable(),
		actions: ui.NewKeyActions(),
		jobName: "hello",
	}
	v.renderBuilds(builds)

	assert.Equal(t, 3, v.table.RowCount())

	// Find the row for build #3 and confirm its RESULT cell signals BUILDING.
	var buildingCell string
	for i := 0; i < v.table.RowCount(); i++ {
		if v.table.GetCell(i+1, 0).Text == "#3" {
			buildingCell = v.table.GetCell(i+1, 1).Text
			break
		}
	}
	assert.Contains(t, buildingCell, "BUILDING", "build #3 RESULT cell should show BUILDING")
	assert.True(t, strings.Contains(strings.ToUpper(buildingCell), "BUILDING"))
}
