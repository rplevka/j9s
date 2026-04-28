// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"testing"

	"github.com/derailed/tcell/v2"
	"github.com/stretchr/testify/assert"
)

// TestShiftKeyNamesRegistered ensures that Shift-letter keys have distinct
// names registered in tcell.KeyNames so they render distinctly in the menu
// (e.g. "<shift-d>" vs "<d>").
func TestShiftKeyNamesRegistered(t *testing.T) {
	cases := map[tcell.Key]string{
		KeyShiftA: "Shift-A",
		KeyShiftC: "Shift-C",
		KeyShiftD: "Shift-D",
		KeyShiftG: "Shift-G",
		KeyShiftN: "Shift-N",
		KeyShiftR: "Shift-R",
		KeyShiftS: "Shift-S",
	}
	for k, want := range cases {
		assert.Equal(t, want, tcell.KeyNames[k], "key 0x%x should be named %q", k, want)
	}
}

// TestHintsDistinguishCaseConflicts ensures that when both a lowercase and a
// shift-letter binding exist (e.g. d=Describe and Shift-D=Disable), the menu
// hints produce distinct mnemonics rather than collapsing both to "<d>".
func TestHintsDistinguishCaseConflicts(t *testing.T) {
	a := NewKeyActions()
	a.Bulk(KeyMap{
		KeyD:      NewKeyAction("Describe", nil, true),
		KeyShiftD: NewKeyAction("Disable", nil, true),
	})

	hints := a.Hints()
	mnemonics := make(map[string]string)
	for _, h := range hints {
		mnemonics[h.Description] = h.Mnemonic
	}
	assert.Equal(t, "d", mnemonics["Describe"])
	assert.Equal(t, "Shift-D", mnemonics["Disable"])
	assert.NotEqual(t, mnemonics["Describe"], mnemonics["Disable"])
}
