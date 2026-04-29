// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import "testing"

func TestShouldShowLogo(t *testing.T) {
	// Threshold: width must leave at least minMenuWidth (50) for the menu
	// after subtracting the info panel (40) and logo (26) → width >= 116.
	cases := []struct {
		name          string
		width         int
		userWantsLogo bool
		want          bool
	}{
		{"user disabled — always hidden", 200, false, false},
		{"wide terminal — visible", 200, true, true},
		{"exactly at threshold (116) — visible", 116, true, true},
		{"one column under threshold — hidden", 115, true, false},
		{"narrow terminal — hidden", 80, true, false},
		{"tiny terminal — hidden", 40, true, false},
	}
	for _, tc := range cases {
		got := shouldShowLogo(tc.width, tc.userWantsLogo)
		if got != tc.want {
			t.Errorf("%s: shouldShowLogo(%d, %v) = %v, want %v",
				tc.name, tc.width, tc.userWantsLogo, got, tc.want)
		}
	}
}
