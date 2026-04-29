// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseJenkinsURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "root jobs URL",
			url:      "https://jenkins.example.com/",
			wantPath: "jobs",
			wantErr:  false,
		},
		{
			name:     "simple job URL",
			url:      "https://jenkins.example.com/job/MyJob/",
			wantPath: "jobs/MyJob",
			wantErr:  false,
		},
		{
			name:     "nested folder URL",
			url:      "https://jenkins.example.com/job/Folder/job/SubFolder/job/MyJob/",
			wantPath: "jobs/Folder/SubFolder/MyJob",
			wantErr:  false,
		},
		{
			name:     "build URL",
			url:      "https://jenkins.example.com/job/MyJob/123/",
			wantPath: "builds/MyJob#123",
			wantErr:  false,
		},
		{
			name:     "nested job build URL",
			url:      "https://jenkins.example.com/job/Folder/job/MyJob/456/",
			wantPath: "builds/Folder/MyJob#456",
			wantErr:  false,
		},
		{
			name:     "view URL",
			url:      "https://jenkins.example.com/view/MyView/",
			wantPath: "views/MyView",
			wantErr:  false,
		},
		{
			name:     "URL without trailing slash",
			url:      "https://jenkins.example.com/job/MyJob",
			wantPath: "jobs/MyJob",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJenkinsURL(tt.url, nil)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPath, got)
		})
	}
}

func TestGenerateJenkinsURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{
			name:    "root jobs",
			baseURL: "https://jenkins.example.com",
			path:    "jobs",
			want:    "https://jenkins.example.com/",
		},
		{
			name:    "simple job",
			baseURL: "https://jenkins.example.com",
			path:    "jobs/MyJob",
			want:    "https://jenkins.example.com/job/MyJob/",
		},
		{
			name:    "nested folder",
			baseURL: "https://jenkins.example.com",
			path:    "jobs/Folder/SubFolder/MyJob",
			want:    "https://jenkins.example.com/job/Folder/job/SubFolder/job/MyJob/",
		},
		{
			name:    "builds without number",
			baseURL: "https://jenkins.example.com",
			path:    "builds/MyJob",
			want:    "https://jenkins.example.com/job/MyJob/",
		},
		{
			name:    "builds with number",
			baseURL: "https://jenkins.example.com",
			path:    "builds/MyJob#123",
			want:    "https://jenkins.example.com/job/MyJob/123/",
		},
		{
			name:    "nested job builds",
			baseURL: "https://jenkins.example.com",
			path:    "builds/Folder/MyJob#456",
			want:    "https://jenkins.example.com/job/Folder/job/MyJob/456/",
		},
		{
			name:    "view",
			baseURL: "https://jenkins.example.com",
			path:    "views/MyView",
			want:    "https://jenkins.example.com/view/MyView/",
		},
		{
			name:    "base URL with trailing slash",
			baseURL: "https://jenkins.example.com/",
			path:    "jobs/MyJob",
			want:    "https://jenkins.example.com/job/MyJob/",
		},
		{
			name:    "logs with simple job",
			baseURL: "https://jenkins.example.com",
			path:    "logs/MyJob/123",
			want:    "https://jenkins.example.com/job/MyJob/123/console",
		},
		{
			name:    "logs with nested job",
			baseURL: "https://jenkins.example.com",
			path:    "logs/Folder/Sub/MyJob/42",
			want:    "https://jenkins.example.com/job/Folder/job/Sub/job/MyJob/42/console",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateJenkinsURL(tt.baseURL, tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractJobPath(t *testing.T) {
	tests := []struct {
		name    string
		urlPath string
		want    string
	}{
		{
			name:    "simple job",
			urlPath: "/job/MyJob/",
			want:    "MyJob",
		},
		{
			name:    "nested folders",
			urlPath: "/job/Folder/job/SubFolder/job/MyJob/",
			want:    "Folder/SubFolder/MyJob",
		},
		{
			name:    "job with build number",
			urlPath: "/job/MyJob/123/",
			want:    "MyJob",
		},
		{
			name:    "nested job with build number",
			urlPath: "/job/Folder/job/MyJob/456/console",
			want:    "Folder/MyJob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJobPath(tt.urlPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"999999", true},
		{"", false},
		{"abc", false},
		{"12a3", false},
		{"12.3", false},
		{"-123", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isNumeric(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
