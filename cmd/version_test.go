// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package cmd

import "testing"

func TestDisplayVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{"dev with commit appends sha", "dev", "abc1234", "dev-abc1234"},
		{"dev without commit stays dev", "dev", "", "dev"},
		{"dev with commit==dev stays dev", "dev", "dev", "dev"},
		{"release version returned as-is", "v1.2.3", "abc1234", "v1.2.3"},
		{"release without commit returned as-is", "v1.2.3", "", "v1.2.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := displayVersion(tc.version, tc.commit); got != tc.want {
				t.Errorf("displayVersion(%q, %q) = %q, want %q", tc.version, tc.commit, got, tc.want)
			}
		})
	}
}
