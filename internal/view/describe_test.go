// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"testing"

	"github.com/roman-plevka/j9s/internal/config"
	"github.com/stretchr/testify/assert"
)

// TestDescribeView_URLProvider asserts the path/URL pair surfaced for the
// global "u" hotkey on the describe view: jobs map to /job/.../, builds
// map to /job/.../<num>/.
func TestDescribeView_URLProvider(t *testing.T) {
	cfg := config.NewConfig()
	cfg.J9s.Contexts = []config.Context{{
		Name: "test", URL: "https://jenkins.example.com",
	}}
	cfg.J9s.CurrentContext = "test"
	app := NewApp(cfg)
	app.App.Init()

	cases := []struct {
		name         string
		resourceType string
		resourceName string
		wantPath     string
		wantURL      string
	}{
		{
			name:         "job describe",
			resourceType: "job",
			resourceName: "Folder/MyJob",
			wantPath:     "jobs/Folder/MyJob",
			wantURL:      "https://jenkins.example.com/job/Folder/job/MyJob/",
		},
		{
			name:         "build describe",
			resourceType: "build",
			resourceName: "Folder/MyJob#42",
			wantPath:     "builds/Folder/MyJob#42",
			wantURL:      "https://jenkins.example.com/job/Folder/job/MyJob/42/",
		},
		{
			name:         "build describe without build number — no URL",
			resourceType: "build",
			resourceName: "Folder/MyJob",
			wantPath:     "",
			wantURL:      "",
		},
		{
			name:         "unknown resource type",
			resourceType: "node",
			resourceName: "agent-1",
			wantPath:     "",
			wantURL:      "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &DescribeView{
				app:          app,
				resourceType: tc.resourceType,
				resourceName: tc.resourceName,
			}
			assert.Equal(t, tc.wantPath, v.GetViewPath())
			assert.Equal(t, tc.wantURL, v.GetJenkinsURL())
		})
	}

	// URLProvider interface compliance.
	var _ URLProvider = (*DescribeView)(nil)
}
