// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"strings"
	"testing"

	"github.com/roman-plevka/j9s/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// node is a tiny test helper that builds a BlueNode with edges from a
// list of successor IDs, keeping the test cases readable.
func node(id, name, result string, edges ...string) client.BlueNode {
	es := make([]client.BlueEdge, 0, len(edges))
	for _, e := range edges {
		es = append(es, client.BlueEdge{ID: e})
	}
	return client.BlueNode{ID: id, DisplayName: name, Result: result, State: "FINISHED", Edges: es}
}

// rowsByName returns the prefix indexed by displayName so test
// assertions stay readable even if input order shifts.
func rowsByName(rows []GraphRow) map[string]GraphRow {
	out := make(map[string]GraphRow, len(rows))
	for _, r := range rows {
		out[r.Node.DisplayName] = r
	}
	return out
}

// TestLayoutGraph_Sequential: a flat chain of stages renders as a
// single trunk column with no tree-drawing characters.
func TestLayoutGraph_Sequential(t *testing.T) {
	nodes := []client.BlueNode{
		node("1", "build", "SUCCESS", "2"),
		node("2", "test", "SUCCESS", "3"),
		node("3", "deploy", "SUCCESS"),
	}
	rows := LayoutGraph(nodes)
	require.Len(t, rows, 3)
	for _, r := range rows {
		assert.Equal(t, 0, r.Depth, "row %s should be at depth 0", r.Node.DisplayName)
		assert.False(t, strings.Contains(r.Prefix, "├"), "no branch markers for %s", r.Node.DisplayName)
		assert.False(t, strings.Contains(r.Prefix, "└"), "no branch markers for %s", r.Node.DisplayName)
	}
}

// TestLayoutGraph_Parallel: a single split with 3 parallel branches
// followed by a join. Branches use ├─/└─ markers, the join collapses
// back to depth 0.
func TestLayoutGraph_Parallel(t *testing.T) {
	nodes := []client.BlueNode{
		node("1", "build", "SUCCESS", "2"),
		node("2", "test", "SUCCESS", "3", "4", "5"),
		node("3", "unit", "SUCCESS", "6"),
		node("4", "integration", "SUCCESS", "6"),
		node("5", "ui", "SUCCESS", "6"),
		node("6", "deploy", "SUCCESS"),
	}
	rows := LayoutGraph(nodes)
	by := rowsByName(rows)
	require.Len(t, rows, 6)

	assert.Equal(t, 0, by["build"].Depth)
	assert.Equal(t, 0, by["test"].Depth)
	assert.Equal(t, 1, by["unit"].Depth)
	assert.Equal(t, 1, by["integration"].Depth)
	assert.Equal(t, 1, by["ui"].Depth, "last sibling still at depth 1")
	assert.Equal(t, 0, by["deploy"].Depth, "join collapses back to trunk")

	assert.Contains(t, by["unit"].Prefix, "├─")
	assert.Contains(t, by["integration"].Prefix, "├─")
	assert.Contains(t, by["ui"].Prefix, "└─", "last sibling is └─")
	assert.NotContains(t, by["deploy"].Prefix, "├")
	assert.NotContains(t, by["deploy"].Prefix, "└")
}

// TestLayoutGraph_FutureNodes: future nodes (state==null,
// result==null) still render — the layout shouldn't drop them just
// because they have no result yet.
func TestLayoutGraph_FutureNodes(t *testing.T) {
	nodes := []client.BlueNode{
		node("1", "build", "SUCCESS", "2"),
		{ID: "2", DisplayName: "deploy", State: "", Edges: []client.BlueEdge{}},
	}
	rows := LayoutGraph(nodes)
	require.Len(t, rows, 2)
	assert.Equal(t, "deploy", rows[1].Node.DisplayName)
}

// TestLayoutGraph_Empty: nil-safe.
func TestLayoutGraph_Empty(t *testing.T) {
	assert.Nil(t, LayoutGraph(nil))
	assert.Nil(t, LayoutGraph([]client.BlueNode{}))
}

// TestLayoutGraph_DoubleParallel: two successive parallel clusters
// (split → join → split → join) both render at depth 1, with the
// trunk returning to depth 0 in between.
func TestLayoutGraph_DoubleParallel(t *testing.T) {
	nodes := []client.BlueNode{
		node("1", "split-a", "SUCCESS", "2", "3"),
		node("2", "branch-a1", "SUCCESS", "4"),
		node("3", "branch-a2", "SUCCESS", "4"),
		node("4", "split-b", "SUCCESS", "5", "6"),
		node("5", "branch-b1", "SUCCESS", "7"),
		node("6", "branch-b2", "SUCCESS", "7"),
		node("7", "deploy", "SUCCESS"),
	}
	rows := LayoutGraph(nodes)
	by := rowsByName(rows)

	assert.Equal(t, 0, by["split-a"].Depth)
	assert.Equal(t, 1, by["branch-a1"].Depth)
	assert.Equal(t, 1, by["branch-a2"].Depth)
	assert.Equal(t, 0, by["split-b"].Depth)
	assert.Equal(t, 1, by["branch-b1"].Depth)
	assert.Equal(t, 1, by["branch-b2"].Depth)
	assert.Equal(t, 0, by["deploy"].Depth)

	assert.Contains(t, by["branch-a2"].Prefix, "└─")
	assert.Contains(t, by["branch-b2"].Prefix, "└─")
}

// TestColorizeBlueState verifies the status icon mapping for the four
// states the pipeline view exercises.
func TestColorizeBlueState(t *testing.T) {
	cases := []struct {
		result, state string
		wantContains  string
	}{
		{"SUCCESS", "FINISHED", "●"},
		{"FAILURE", "FINISHED", "●"},
		{"UNSTABLE", "FINISHED", "●"},
		{"", "RUNNING", "◐"},
		{"NOT_BUILT", "", "○"},
		{"", "", "○"},
	}
	for _, c := range cases {
		got := colorizeBlueState(c.result, c.state)
		assert.Contains(t, got, c.wantContains, "result=%s state=%s", c.result, c.state)
	}
}
