// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"strings"

	"github.com/roman-plevka/j9s/internal/client"
)

// GraphRow is one rendered row of the pipeline graph: the source
// BlueNode, an ASCII prefix that visualises its position in the DAG
// (status icon + tree-drawing characters for parallel branches), and
// the depth at which it sits.
type GraphRow struct {
	Node   client.BlueNode
	Prefix string
	Depth  int
}

// LayoutGraph computes a per-row prefix for each node in a pipeline
// run, mirroring how Blue Ocean visualises parallel branches:
//
//	● build
//	● test
//	  ├─ ● unit
//	  ├─ ● integration
//	  └─ ● ui
//	● deploy
//
// Algorithm:
//
//  1. Build parent[id] = []parentID from each node's edges[].
//  2. Walk nodes in their input order (which Blue Ocean already returns
//     topologically) and track a stack of "open" split nodes.
//  3. A node with multiple parents is a JOIN — pop one level off the
//     stack so subsequent rows return to the outer rail.
//  4. A node whose only parent is a split (>1 outgoing edges) is a
//     branch row — render with ├─ / └─ depending on whether it is the
//     last sibling in input order.
//  5. A node with multiple outgoing edges is a SPLIT — push it so its
//     children render at depth+1.
//
// The function is pure (no I/O, no tview deps) so it is exhaustively
// unit-tested in blueocean_graph_test.go.
func LayoutGraph(nodes []client.BlueNode) []GraphRow {
	if len(nodes) == 0 {
		return nil
	}

	// id -> list of parent IDs (i.e. nodes whose edges include this id).
	parents := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		for _, e := range n.Edges {
			parents[e.ID] = append(parents[e.ID], n.ID)
		}
	}

	// Index of last sibling per split parent: the largest input position
	// of any node whose only parent is the split. Used to pick └─ vs ├─.
	indexOf := make(map[string]int, len(nodes))
	for i, n := range nodes {
		indexOf[n.ID] = i
	}
	lastSibling := make(map[string]string)
	for _, n := range nodes {
		ps := parents[n.ID]
		if len(ps) != 1 {
			continue
		}
		parent := ps[0]
		// Only treat as a "branch row" if the parent is a split (>1 edges).
		if !isSplit(nodes, parent) {
			continue
		}
		if cur, ok := lastSibling[parent]; !ok || indexOf[n.ID] > indexOf[cur] {
			lastSibling[parent] = n.ID
		}
	}

	// Walk + render. depth tracks the current "trunk" indent so that
	// nested splits accumulate and joins collapse symmetrically.
	rows := make([]GraphRow, 0, len(nodes))
	depth := 0
	for _, n := range nodes {
		ps := parents[n.ID]

		// Joins collapse one level of indent for this and subsequent rows.
		if len(ps) > 1 && depth > 0 {
			depth--
		}

		// A node whose only parent is a split renders as a branch row
		// (├─/└─). Otherwise it's a trunk row at the current depth.
		isBranch := len(ps) == 1 && isSplit(nodes, ps[0])
		marker := "● "
		if isBranch {
			if lastSibling[ps[0]] == n.ID {
				marker = "└─ ● "
			} else {
				marker = "├─ ● "
			}
		}

		rows = append(rows, GraphRow{
			Node:   n,
			Prefix: rails(depth) + marker,
			Depth:  depth,
		})

		// Trunk depth increases when a node forks; the bump applies to
		// future rows (the branches themselves and any further nesting),
		// not the split row itself.
		if len(n.Edges) > 1 {
			depth++
		}
	}
	return rows
}

// isSplit reports whether the named node has >1 outgoing edges in the
// input list. Linear in the worst case; the call count is O(branch
// rows) so this stays well below the cost of the BFS layout itself.
func isSplit(nodes []client.BlueNode, id string) bool {
	for i := range nodes {
		if nodes[i].ID == id {
			return len(nodes[i].Edges) > 1
		}
	}
	return false
}

// rails returns the leading "│  " continuation columns rendered before
// every row at depth >= 1. depth == 0 → empty string. depth == 1 →
// "" (the branch marker carries the visual weight). depth == N>1 →
// (N-1) repetitions of "│  ".
func rails(depth int) string {
	if depth <= 1 {
		return ""
	}
	return strings.Repeat("│  ", depth-1)
}

// colorizeBlueState renders a status icon for a Blue Ocean node/step.
// Mirrors Jenkins' colour palette so the pipeline graph feels visually
// consistent with the rest of j9s.
//
//	SUCCESS              -> green ●
//	FAILURE / ABORTED    -> red ●  / base01 ●
//	UNSTABLE             -> yellow ●
//	RUNNING              -> blue ◐
//	null / NOT_BUILT etc -> grey ○ (future / pending)
func colorizeBlueState(result, state string) string {
	if state == "RUNNING" {
		return "[#268bd2::b]◐[-::-]"
	}
	switch result {
	case "SUCCESS":
		return "[#859900::b]●[-::-]"
	case "FAILURE":
		return "[#dc322f::b]●[-::-]"
	case "UNSTABLE":
		return "[#b58900::b]●[-::-]"
	case "ABORTED":
		return "[#586e75::b]●[-::-]"
	case "NOT_BUILT":
		return "[#93a1a1::-]○[-::-]"
	default:
		return "[#93a1a1::-]○[-::-]"
	}
}
