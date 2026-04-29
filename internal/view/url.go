// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/roman-plevka/j9s/internal/config"
)

// URLProvider is an interface for views that can provide their Jenkins URL.
type URLProvider interface {
	// GetJenkinsURL returns the Jenkins web UI URL for this view.
	GetJenkinsURL() string
	// GetViewPath returns the internal view path for bookmarking.
	GetViewPath() string
}

// jobPathRegex matches /job/name/job/name/... patterns
var jobPathRegex = regexp.MustCompile(`^/job/([^/]+)(?:/job/([^/]+))*`)

// buildNumberRegex matches /123/ at the end of a URL path
var buildNumberRegex = regexp.MustCompile(`/(\d+)/?$`)

// viewPathRegex matches /view/name/ patterns
var viewPathRegex = regexp.MustCompile(`^/view/([^/]+)`)

// ParseJenkinsURL parses a Jenkins URL and returns the internal view path.
// Examples:
//   - https://jenkins.example.com/job/Folder/job/MyJob/ -> jobs/Folder/MyJob
//   - https://jenkins.example.com/job/MyJob/123/ -> builds/MyJob#123
//   - https://jenkins.example.com/view/MyView/ -> views/MyView
func ParseJenkinsURL(jenkinsURL string, cfg *config.Config) (string, error) {
	parsed, err := url.Parse(jenkinsURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	path := parsed.Path

	// Check for view URL
	if viewMatch := viewPathRegex.FindStringSubmatch(path); len(viewMatch) > 1 {
		return "views/" + viewMatch[1], nil
	}

	// Check for job/build URL
	if strings.Contains(path, "/job/") {
		// Extract job path
		jobPath := extractJobPath(path)
		if jobPath == "" {
			return "", fmt.Errorf("could not parse job path from URL")
		}

		// Check if it's a build URL (has build number)
		if buildMatch := buildNumberRegex.FindStringSubmatch(path); len(buildMatch) > 1 {
			return fmt.Sprintf("builds/%s#%s", jobPath, buildMatch[1]), nil
		}

		// It's a job/folder URL
		return "jobs/" + jobPath, nil
	}

	// Default to jobs view
	return "jobs", nil
}

// extractJobPath extracts the job path from a Jenkins URL path.
// /job/Folder/job/SubFolder/job/MyJob/ -> Folder/SubFolder/MyJob
func extractJobPath(urlPath string) string {
	parts := strings.Split(urlPath, "/")
	var jobParts []string

	for i := 0; i < len(parts); i++ {
		if parts[i] == "job" && i+1 < len(parts) && parts[i+1] != "" {
			// Skip numeric parts (build numbers)
			if !isNumeric(parts[i+1]) {
				jobParts = append(jobParts, parts[i+1])
			}
			i++ // Skip the next part since we've processed it
		}
	}

	return strings.Join(jobParts, "/")
}

// isNumeric checks if a string is a number.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// GenerateJenkinsURL generates a Jenkins web UI URL from a base URL and path.
func GenerateJenkinsURL(baseURL, path string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Handle different path types
	if strings.HasPrefix(path, "jobs/") {
		jobPath := strings.TrimPrefix(path, "jobs/")
		if jobPath == "" {
			return baseURL + "/"
		}
		return baseURL + "/job/" + strings.ReplaceAll(jobPath, "/", "/job/") + "/"
	}

	if strings.HasPrefix(path, "builds/") {
		// builds/JobName#123 or builds/Folder/JobName#123
		buildPath := strings.TrimPrefix(path, "builds/")
		parts := strings.SplitN(buildPath, "#", 2)
		jobPath := parts[0]
		jenkinsPath := baseURL + "/job/" + strings.ReplaceAll(jobPath, "/", "/job/") + "/"
		if len(parts) > 1 {
			jenkinsPath += parts[1] + "/"
		}
		return jenkinsPath
	}

	if strings.HasPrefix(path, "views/") {
		viewName := strings.TrimPrefix(path, "views/")
		return baseURL + "/view/" + viewName + "/"
	}

	// logs/<jobPath>/<buildNum> → <base>/job/.../<buildNum>/console
	if strings.HasPrefix(path, "logs/") {
		rest := strings.TrimPrefix(path, "logs/")
		idx := strings.LastIndex(rest, "/")
		if idx <= 0 || idx == len(rest)-1 {
			return baseURL + "/"
		}
		jobPath := rest[:idx]
		buildNum := rest[idx+1:]
		return baseURL + "/job/" + strings.ReplaceAll(jobPath, "/", "/job/") + "/" + buildNum + "/console"
	}

	// Default
	return baseURL + "/"
}
