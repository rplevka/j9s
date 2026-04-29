// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/roman-plevka/j9s/internal/config"
)

// Client represents a Jenkins API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       config.AuthConfig
	crumb      *Crumb
}

// Crumb represents a Jenkins CSRF crumb.
type Crumb struct {
	RequestField string `json:"crumbRequestField"`
	Value        string `json:"crumb"`
}

// NewClient creates a new Jenkins client.
func NewClient(ctx *config.Context) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: ctx.Insecure,
		},
	}

	client := &Client{
		baseURL: strings.TrimSuffix(ctx.URL, "/"),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		auth: ctx.Auth,
	}

	return client, nil
}

// doRequest performs an HTTP request with authentication.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	switch c.auth.Type {
	case config.AuthTypeToken:
		req.SetBasicAuth(c.auth.Username, c.auth.Token)
	case config.AuthTypePassword:
		req.SetBasicAuth(c.auth.Username, c.auth.Password)
	case config.AuthTypeOAuth:
		if c.auth.OAuth != nil && c.auth.OAuth.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.auth.OAuth.AccessToken)
		}
	}

	req.Header.Set("Accept", "application/json")

	// Add CSRF crumb if available
	if c.crumb != nil {
		req.Header.Set(c.crumb.RequestField, c.crumb.Value)
	}

	return c.httpClient.Do(req)
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// post performs a POST request.
func (c *Client) post(ctx context.Context, path string, body io.Reader) ([]byte, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}

// FetchCrumb fetches the CSRF crumb from Jenkins.
func (c *Client) FetchCrumb(ctx context.Context) error {
	data, err := c.get(ctx, "/crumbIssuer/api/json")
	if err != nil {
		// CSRF might be disabled
		return nil
	}

	var crumb Crumb
	if err := json.Unmarshal(data, &crumb); err != nil {
		return fmt.Errorf("failed to parse crumb: %w", err)
	}

	c.crumb = &crumb
	return nil
}

// CheckConnection checks if the connection to Jenkins is working.
func (c *Client) CheckConnection(ctx context.Context) error {
	_, err := c.get(ctx, "/api/json")
	return err
}

// JenkinsInfo represents basic Jenkins information.
type JenkinsInfo struct {
	Mode            string `json:"mode"`
	NodeDescription string `json:"nodeDescription"`
	NodeName        string `json:"nodeName"`
	NumExecutors    int    `json:"numExecutors"`
	Description     string `json:"description"`
	UseCrumbs       bool   `json:"useCrumbs"`
	UseSecurity     bool   `json:"useSecurity"`
	Version         string `json:"-"`
}

// GetInfo returns basic Jenkins information.
func (c *Client) GetInfo(ctx context.Context) (*JenkinsInfo, error) {
	data, err := c.get(ctx, "/api/json")
	if err != nil {
		return nil, err
	}

	var info JenkinsInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse info: %w", err)
	}

	// Get version from header
	resp, err := c.doRequest(ctx, http.MethodHead, "/", nil)
	if err == nil {
		info.Version = resp.Header.Get("X-Jenkins")
		resp.Body.Close()
	}

	return &info, nil
}

// Job represents a Jenkins job.
type Job struct {
	Class       string `json:"_class"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	FullName    string `json:"fullName"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Buildable   bool   `json:"buildable"`
	Color       string `json:"color"`
	InQueue     bool   `json:"inQueue"`

	// For folders
	Jobs []Job `json:"jobs,omitempty"`

	// Build info
	LastBuild             *Build `json:"lastBuild,omitempty"`
	LastSuccessfulBuild   *Build `json:"lastSuccessfulBuild,omitempty"`
	LastFailedBuild       *Build `json:"lastFailedBuild,omitempty"`
	LastCompletedBuild    *Build `json:"lastCompletedBuild,omitempty"`
	LastStableBuild       *Build `json:"lastStableBuild,omitempty"`
	LastUnstableBuild     *Build `json:"lastUnstableBuild,omitempty"`
	LastUnsuccessfulBuild *Build `json:"lastUnsuccessfulBuild,omitempty"`

	// Health
	HealthReport []HealthReport `json:"healthReport,omitempty"`

	// Builds list
	Builds []Build `json:"builds,omitempty"`

	// Parameters
	Property []JobProperty `json:"property,omitempty"`
}

// HealthReport represents job health information.
type HealthReport struct {
	Description   string `json:"description"`
	IconClassName string `json:"iconClassName"`
	IconURL       string `json:"iconUrl"`
	Score         int    `json:"score"`
}

// JobProperty represents a job property.
type JobProperty struct {
	Class                string         `json:"_class"`
	ParameterDefinitions []ParameterDef `json:"parameterDefinitions,omitempty"`
}

// ParameterDef represents a parameter definition.
type ParameterDef struct {
	Class                 string      `json:"_class"`
	Name                  string      `json:"name"`
	Description           string      `json:"description"`
	DefaultParameterValue *ParamValue `json:"defaultParameterValue,omitempty"`
	Type                  string      `json:"type"`
	Choices               []string    `json:"choices,omitempty"`
}

// ParamValue represents a parameter value.
type ParamValue struct {
	Class string      `json:"_class"`
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// Build represents a Jenkins build.
type Build struct {
	Class             string `json:"_class"`
	Number            int    `json:"number"`
	URL               string `json:"url"`
	DisplayName       string `json:"displayName"`
	FullDisplayName   string `json:"fullDisplayName"`
	Description       string `json:"description"`
	Duration          int64  `json:"duration"`
	EstimatedDuration int64  `json:"estimatedDuration"`
	Building          bool   `json:"building"`
	Result            string `json:"result"`
	Timestamp         int64  `json:"timestamp"`
	ID                string `json:"id"`

	// Actions contain various build information
	Actions []BuildAction `json:"actions,omitempty"`

	// Artifacts
	Artifacts []Artifact `json:"artifacts,omitempty"`

	// Change sets
	ChangeSets []ChangeSet `json:"changeSets,omitempty"`
}

// BuildAction represents a build action.
type BuildAction struct {
	Class      string       `json:"_class"`
	Parameters []ParamValue `json:"parameters,omitempty"`
	Causes     []Cause      `json:"causes,omitempty"`
}

// Cause represents a build cause.
type Cause struct {
	Class            string `json:"_class"`
	ShortDescription string `json:"shortDescription"`
	UserID           string `json:"userId,omitempty"`
	UserName         string `json:"userName,omitempty"`
}

// Artifact represents a build artifact.
type Artifact struct {
	DisplayPath  string `json:"displayPath"`
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}

// ChangeSet represents a change set.
type ChangeSet struct {
	Class string       `json:"_class"`
	Kind  string       `json:"kind"`
	Items []ChangeItem `json:"items,omitempty"`
}

// ChangeItem represents a change item.
type ChangeItem struct {
	Class         string   `json:"_class"`
	CommitID      string   `json:"commitId"`
	Timestamp     int64    `json:"timestamp"`
	Author        Author   `json:"author"`
	Comment       string   `json:"comment"`
	AffectedPaths []string `json:"affectedPaths,omitempty"`
}

// Author represents a commit author.
type Author struct {
	FullName string `json:"fullName"`
}

// QueueItem represents an item in the build queue.
type QueueItem struct {
	Class        string `json:"_class"`
	ID           int    `json:"id"`
	Blocked      bool   `json:"blocked"`
	Buildable    bool   `json:"buildable"`
	InQueueSince int64  `json:"inQueueSince"`
	Params       string `json:"params"`
	Stuck        bool   `json:"stuck"`
	Why          string `json:"why"`
	Task         Task   `json:"task"`
}

// Task represents a queue task.
type Task struct {
	Class string `json:"_class"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
}

// Node represents a Jenkins node/agent.
type Node struct {
	Class               string                 `json:"_class"`
	DisplayName         string                 `json:"displayName"`
	Description         string                 `json:"description"`
	Idle                bool                   `json:"idle"`
	JnlpAgent           bool                   `json:"jnlpAgent"`
	LaunchSupported     bool                   `json:"launchSupported"`
	ManualLaunchAllowed bool                   `json:"manualLaunchAllowed"`
	NumExecutors        int                    `json:"numExecutors"`
	Offline             bool                   `json:"offline"`
	OfflineCause        interface{}            `json:"offlineCause"`
	OfflineCauseReason  string                 `json:"offlineCauseReason"`
	TemporarilyOffline  bool                   `json:"temporarilyOffline"`
	MonitorData         map[string]interface{} `json:"monitorData,omitempty"`
}

// User represents a Jenkins user.
type User struct {
	Class       string `json:"_class"`
	ID          string `json:"id"`
	FullName    string `json:"fullName"`
	Description string `json:"description"`
	AbsoluteURL string `json:"absoluteUrl"`
}

// Credential represents a Jenkins credential.
type Credential struct {
	Class       string      `json:"_class"`
	ID          string      `json:"id"`
	DisplayName string      `json:"displayName"`
	Description string      `json:"description"`
	TypeName    string      `json:"typeName"`
	Fingerprint interface{} `json:"fingerprint"`
}

// Plugin represents a Jenkins plugin.
type Plugin struct {
	Active              bool   `json:"active"`
	BackupVersion       string `json:"backupVersion,omitempty"`
	Bundled             bool   `json:"bundled"`
	Deleted             bool   `json:"deleted"`
	Downgradable        bool   `json:"downgradable"`
	Enabled             bool   `json:"enabled"`
	HasUpdate           bool   `json:"hasUpdate"`
	LongName            string `json:"longName"`
	Pinned              bool   `json:"pinned"`
	ShortName           string `json:"shortName"`
	SupportsDynamicLoad string `json:"supportsDynamicLoad"`
	URL                 string `json:"url"`
	Version             string `json:"version"`
}

// TestReport mirrors the JUnit-plugin /testReport/api/json document.
// Only fields surfaced by j9s views are decoded; the rest are silently
// dropped by encoding/json.
type TestReport struct {
	Class     string      `json:"_class"`
	Duration  float64     `json:"duration"`
	Empty     bool        `json:"empty"`
	FailCount int         `json:"failCount"`
	PassCount int         `json:"passCount"`
	SkipCount int         `json:"skipCount"`
	Suites    []TestSuite `json:"suites"`
}

// TestSuite is a single <testsuite> from the report. Cases live inline.
type TestSuite struct {
	Name     string     `json:"name"`
	Duration float64    `json:"duration"`
	Cases    []TestCase `json:"cases"`
}

// TestCase is a single <testcase>. Status follows the JUnit plugin's
// values: PASSED, FAILED, SKIPPED, REGRESSION, FIXED.
type TestCase struct {
	ClassName       string  `json:"className"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Duration        float64 `json:"duration"`
	ErrorDetails    string  `json:"errorDetails,omitempty"`
	ErrorStackTrace string  `json:"errorStackTrace,omitempty"`
	Skipped         bool    `json:"skipped,omitempty"`
	SkippedMessage  string  `json:"skippedMessage,omitempty"`
	Stdout          string  `json:"stdout,omitempty"`
	Stderr          string  `json:"stderr,omitempty"`
}

// HTMLReport is one HTML Publisher report attached to a build. j9s does
// not render the HTML; it lists reports and lets the user open them in
// the system browser via the Jenkins URL or copy the URL.
type HTMLReport struct {
	// URLName is the path segment under the build (e.g. "pytest_html").
	URLName string `json:"urlName"`
	// ReportName is the human-friendly name shown by Jenkins.
	ReportName string `json:"reportName"`
}

// View represents a Jenkins view.
type View struct {
	Class       string `json:"_class"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Jobs        []Job  `json:"jobs,omitempty"`
}

// GetJobs returns all jobs.
func (c *Client) GetJobs(ctx context.Context) ([]Job, error) {
	data, err := c.get(ctx, "/api/json?tree=jobs[_class,name,url,fullName,displayName,color,buildable,inQueue,lastBuild[number,result,timestamp,building],healthReport[description,score]]")
	if err != nil {
		return nil, err
	}

	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse jobs: %w", err)
	}

	return result.Jobs, nil
}

// GetFolderJobs returns jobs inside a folder.
func (c *Client) GetFolderJobs(ctx context.Context, folderPath string) ([]Job, error) {
	path := jobPath(folderPath) + "/api/json?tree=jobs[_class,name,url,fullName,displayName,color,buildable,inQueue,lastBuild[number,result,timestamp,building],healthReport[description,score]]"
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse folder jobs: %w", err)
	}

	return result.Jobs, nil
}

// IsFolder returns true if the job class indicates it's a folder.
func (j *Job) IsFolder() bool {
	return strings.Contains(j.Class, "Folder") ||
		strings.Contains(j.Class, "OrganizationFolder") ||
		strings.Contains(j.Class, "MultiBranchProject")
}

// jobPath converts a job name (potentially with folder path like "folder/subfolder/job")
// to Jenkins URL path format ("/job/folder/job/subfolder/job/job").
func jobPath(name string) string {
	parts := strings.Split(name, "/")
	escaped := make([]string, len(parts))
	for i, p := range parts {
		escaped[i] = url.PathEscape(p)
	}
	return "/job/" + strings.Join(escaped, "/job/")
}

// GetJob returns a specific job.
func (c *Client) GetJob(ctx context.Context, name string) (*Job, error) {
	// Note: parameterDefinitions[*] alone does NOT expand the nested
	// defaultParameterValue object — Jenkins' tree= filter only descends one
	// level. Without defaultParameterValue[*] the .Value field is missing,
	// which breaks default-value pre-fill in the trigger build dialog.
	path := jobPath(name) + "/api/json?tree=*,property[*,parameterDefinitions[*,defaultParameterValue[*]]],lastBuild[number,actions[parameters[*]]]"
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse job: %w", err)
	}

	return &job, nil
}

// GetJobParameters returns the parameter definitions for a job.
func (c *Client) GetJobParameters(ctx context.Context, name string) ([]ParameterDef, error) {
	job, err := c.GetJob(ctx, name)
	if err != nil {
		return nil, err
	}

	for _, prop := range job.Property {
		if len(prop.ParameterDefinitions) > 0 {
			return prop.ParameterDefinitions, nil
		}
	}

	return nil, nil // Job has no parameters
}

// GetLastBuildParameters returns the parameters used in the last build.
func (c *Client) GetLastBuildParameters(ctx context.Context, jobName string) (map[string]string, error) {
	job, err := c.GetJob(ctx, jobName)
	if err != nil {
		return nil, err
	}

	if job.LastBuild == nil {
		return nil, nil
	}

	// Get full build details with actions
	build, err := c.GetBuild(ctx, jobName, job.LastBuild.Number)
	if err != nil {
		return nil, err
	}

	params := make(map[string]string)
	for _, action := range build.Actions {
		for _, param := range action.Parameters {
			if param.Name != "" && param.Value != nil {
				params[param.Name] = fmt.Sprintf("%v", param.Value)
			}
		}
	}

	return params, nil
}

// GetJobConfig returns the job configuration XML.
func (c *Client) GetJobConfig(ctx context.Context, name string) (string, error) {
	path := jobPath(name) + "/config.xml"
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get job config: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GetBuilds returns builds for a job.
func (c *Client) GetBuilds(ctx context.Context, jobName string) ([]Build, error) {
	path := jobPath(jobName) + "/api/json?tree=builds[number,url,displayName,result,timestamp,duration,building]"
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Builds []Build `json:"builds"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse builds: %w", err)
	}

	return result.Builds, nil
}

// GetBuild returns a specific build.
func (c *Client) GetBuild(ctx context.Context, jobName string, buildNumber int) (*Build, error) {
	path := fmt.Sprintf("%s/%d/api/json", jobPath(jobName), buildNumber)
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var build Build
	if err := json.Unmarshal(data, &build); err != nil {
		return nil, fmt.Errorf("failed to parse build: %w", err)
	}

	return &build, nil
}

// GetBuildConsoleOutput returns the console output for a build.
func (c *Client) GetBuildConsoleOutput(ctx context.Context, jobName string, buildNumber int) (string, error) {
	path := fmt.Sprintf("%s/%d/consoleText", jobPath(jobName), buildNumber)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ProgressFunc is called with bytes downloaded so far and total size (0 if unknown).
type ProgressFunc func(bytesRead int64, totalSize int64)

// GetBuildConsoleOutputFull returns the FULL console output for a build with progress reporting.
func (c *Client) GetBuildConsoleOutputFull(ctx context.Context, jobName string, buildNumber int) (string, error) {
	return c.GetBuildConsoleOutputFullWithProgress(ctx, jobName, buildNumber, nil)
}

// GetBuildConsoleOutputFullWithProgress returns the FULL console output with progress callback.
// Uses /consoleText endpoint which returns the complete log in one request.
func (c *Client) GetBuildConsoleOutputFullWithProgress(ctx context.Context, jobName string, buildNumber int, progress ProgressFunc) (string, error) {
	// Use consoleText endpoint for complete log (not progressiveText which is for streaming)
	path := fmt.Sprintf("%s/%d/consoleText", jobPath(jobName), buildNumber)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Get total size from Content-Length if available
	var totalSize int64
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		fmt.Sscanf(cl, "%d", &totalSize)
	}

	// Read with progress reporting
	var data []byte
	if progress != nil {
		// Read in chunks and report progress
		buf := make([]byte, 64*1024) // 64KB chunks
		var bytesRead int64
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				data = append(data, buf[:n]...)
				bytesRead += int64(n)
				progress(bytesRead, totalSize)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return string(data), readErr
			}
		}
	} else {
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
	}

	return string(data), nil
}

// GetBuildConsoleSize returns the current size of the build console log.
// This is useful for determining where to start tailing from.
func (c *Client) GetBuildConsoleSize(ctx context.Context, jobName string, buildNumber int) (int64, bool, error) {
	// Use progressiveText with a very high start offset to just get the X-Text-Size header
	// without downloading any content
	path := fmt.Sprintf("%s/%d/logText/progressiveText?start=999999999999", jobPath(jobName), buildNumber)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()

	// Discard any body content (should be empty or minimal)
	io.Copy(io.Discard, resp.Body)

	// Get the total size from X-Text-Size header
	var size int64
	if s := resp.Header.Get("X-Text-Size"); s != "" {
		fmt.Sscanf(s, "%d", &size)
	}

	// Check if build is still running
	moreData := resp.Header.Get("X-More-Data") == "true"

	return size, moreData, nil
}

// StreamBuildConsoleOutput streams the console output for a build.
func (c *Client) StreamBuildConsoleOutput(ctx context.Context, jobName string, buildNumber int, start int64) (string, int64, bool, error) {
	return c.StreamBuildConsoleOutputWithProgress(ctx, jobName, buildNumber, start, nil)
}

// StreamBuildConsoleOutputWithProgress streams console output with a progress callback.
// The progress callback receives bytes read so far and total size (from Content-Length, 0 if unknown).
func (c *Client) StreamBuildConsoleOutputWithProgress(ctx context.Context, jobName string, buildNumber int, start int64, progress func(bytesRead, totalSize int64)) (string, int64, bool, error) {
	path := fmt.Sprintf("%s/%d/logText/progressiveText?start=%d", jobPath(jobName), buildNumber, start)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", start, false, err
	}
	defer resp.Body.Close()

	// Get total size from Content-Length if available
	var totalSize int64
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		fmt.Sscanf(cl, "%d", &totalSize)
	}

	// Read with progress reporting
	var data []byte
	if progress != nil {
		// Read in chunks and report progress
		buf := make([]byte, 32*1024) // 32KB chunks
		var bytesRead int64
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				data = append(data, buf[:n]...)
				bytesRead += int64(n)
				progress(bytesRead, totalSize) // totalSize may be 0 if unknown
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return "", start, false, readErr
			}
		}
	} else {
		// No progress callback - read all at once
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", start, false, err
		}
	}

	// Get the next start position from header
	nextStart := start
	if s := resp.Header.Get("X-Text-Size"); s != "" {
		fmt.Sscanf(s, "%d", &nextStart)
	}

	// Check if more data is available
	moreData := resp.Header.Get("X-More-Data") == "true"

	return string(data), nextStart, moreData, nil
}

// TriggerBuild triggers a build for a job.
// For parameterized jobs, use buildWithParameters endpoint.
func (c *Client) TriggerBuild(ctx context.Context, jobName string, params map[string]string) error {
	var path string
	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}
		path = fmt.Sprintf("%s/buildWithParameters?%s", jobPath(jobName), values.Encode())
	} else {
		// Try /build first, if it fails with 400, try /buildWithParameters
		// (parameterized jobs require buildWithParameters even without params)
		path = jobPath(jobName) + "/build"
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// If /build returns 400 or 405, try /buildWithParameters for parameterized jobs
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusMethodNotAllowed {
		path = jobPath(jobName) + "/buildWithParameters"
		resp2, err := c.doRequest(ctx, http.MethodPost, path, nil)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated && resp2.StatusCode != http.StatusNoContent {
			respBody, _ := io.ReadAll(resp2.Body)
			return fmt.Errorf("request failed with status %d: %s", resp2.StatusCode, string(respBody))
		}
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// StopBuild stops a running build.
func (c *Client) StopBuild(ctx context.Context, jobName string, buildNumber int) error {
	path := fmt.Sprintf("%s/%d/stop", jobPath(jobName), buildNumber)
	_, err := c.post(ctx, path, nil)
	return err
}

// EnableJob enables a job.
func (c *Client) EnableJob(ctx context.Context, jobName string) error {
	path := jobPath(jobName) + "/enable"
	_, err := c.post(ctx, path, nil)
	return err
}

// DisableJob disables a job.
func (c *Client) DisableJob(ctx context.Context, jobName string) error {
	path := jobPath(jobName) + "/disable"
	_, err := c.post(ctx, path, nil)
	return err
}

// DeleteJob deletes a job.
func (c *Client) DeleteJob(ctx context.Context, jobName string) error {
	path := jobPath(jobName) + "/doDelete"
	_, err := c.post(ctx, path, nil)
	return err
}

// GetQueue returns the build queue.
func (c *Client) GetQueue(ctx context.Context) ([]QueueItem, error) {
	data, err := c.get(ctx, "/queue/api/json")
	if err != nil {
		return nil, err
	}

	var result struct {
		Items []QueueItem `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse queue: %w", err)
	}

	return result.Items, nil
}

// CancelQueueItem cancels a queue item.
func (c *Client) CancelQueueItem(ctx context.Context, id int) error {
	path := fmt.Sprintf("/queue/cancelItem?id=%d", id)
	_, err := c.post(ctx, path, nil)
	return err
}

// GetNodes returns all nodes.
func (c *Client) GetNodes(ctx context.Context) ([]Node, error) {
	data, err := c.get(ctx, "/computer/api/json?tree=computer[displayName,description,idle,jnlpAgent,numExecutors,offline,offlineCauseReason,temporarilyOffline]")
	if err != nil {
		return nil, err
	}

	var result struct {
		Computer []Node `json:"computer"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse nodes: %w", err)
	}

	return result.Computer, nil
}

// GetNode returns a specific node.
func (c *Client) GetNode(ctx context.Context, name string) (*Node, error) {
	nodeName := name
	if name == "master" || name == "Built-In Node" {
		nodeName = "(master)"
	}
	path := fmt.Sprintf("/computer/%s/api/json", url.PathEscape(nodeName))
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var node Node
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse node: %w", err)
	}

	return &node, nil
}

// GetUsers returns all users.
func (c *Client) GetUsers(ctx context.Context) ([]User, error) {
	data, err := c.get(ctx, "/asynchPeople/api/json?tree=users[user[id,fullName,description,absoluteUrl]]")
	if err != nil {
		return nil, err
	}

	var result struct {
		Users []struct {
			User User `json:"user"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse users: %w", err)
	}

	users := make([]User, 0, len(result.Users))
	for _, u := range result.Users {
		users = append(users, u.User)
	}

	return users, nil
}

// GetCredentials returns all credentials from a domain.
func (c *Client) GetCredentials(ctx context.Context, domain string) ([]Credential, error) {
	if domain == "" {
		domain = "_"
	}
	path := fmt.Sprintf("/credentials/store/system/domain/%s/api/json?tree=credentials[id,displayName,description,typeName]", url.PathEscape(domain))
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Credentials []Credential `json:"credentials"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return result.Credentials, nil
}

// GetPlugins returns all plugins.
func (c *Client) GetPlugins(ctx context.Context) ([]Plugin, error) {
	data, err := c.get(ctx, "/pluginManager/api/json?tree=plugins[active,enabled,hasUpdate,longName,shortName,version,url]&depth=1")
	if err != nil {
		return nil, err
	}

	var result struct {
		Plugins []Plugin `json:"plugins"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse plugins: %w", err)
	}

	return result.Plugins, nil
}

// GetViews returns all views at the root level.
func (c *Client) GetViews(ctx context.Context) ([]View, error) {
	return c.GetFolderViews(ctx, "")
}

// GetFolderViews returns views for a folder (or root if folderPath is empty).
func (c *Client) GetFolderViews(ctx context.Context, folderPath string) ([]View, error) {
	var path string
	if folderPath == "" {
		path = "/api/json?tree=views[name,url,description]"
	} else {
		path = jobPath(folderPath) + "/api/json?tree=views[name,url,description]"
	}

	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var result struct {
		Views []View `json:"views"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse views: %w", err)
	}

	return result.Views, nil
}

// GetView returns a specific view with its jobs at the root level.
func (c *Client) GetView(ctx context.Context, name string) (*View, error) {
	return c.GetFolderView(ctx, "", name)
}

// GetFolderView returns a specific view with its jobs from a folder (or root if folderPath is empty).
func (c *Client) GetFolderView(ctx context.Context, folderPath, viewName string) (*View, error) {
	var path string
	if folderPath == "" {
		path = fmt.Sprintf("/view/%s/api/json?tree=name,url,description,jobs[_class,name,url,fullName,displayName,color,buildable,inQueue,lastBuild[number,result,timestamp,building],healthReport[description,score]]", url.PathEscape(viewName))
	} else {
		path = fmt.Sprintf("%s/view/%s/api/json?tree=name,url,description,jobs[_class,name,url,fullName,displayName,color,buildable,inQueue,lastBuild[number,result,timestamp,building],healthReport[description,score]]", jobPath(folderPath), url.PathEscape(viewName))
	}

	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var view View
	if err := json.Unmarshal(data, &view); err != nil {
		return nil, fmt.Errorf("failed to parse view: %w", err)
	}

	return &view, nil
}

// GetBuildArtifacts returns the list of artifacts for a build.
func (c *Client) GetBuildArtifacts(ctx context.Context, jobName string, buildNumber int) ([]Artifact, error) {
	build, err := c.GetBuild(ctx, jobName, buildNumber)
	if err != nil {
		return nil, err
	}
	return build.Artifacts, nil
}

// GetTestReport returns the JUnit-plugin test report for a build. Returns
// (nil, nil) when the build does not publish a JUnit report (404), so
// callers can distinguish "no report" from genuine errors.
func (c *Client) GetTestReport(ctx context.Context, jobName string, buildNumber int) (*TestReport, error) {
	path := fmt.Sprintf("%s/%d/testReport/api/json?tree=duration,empty,failCount,passCount,skipCount,suites[name,duration,cases[className,name,status,duration,errorDetails,errorStackTrace,skipped,skippedMessage,stdout,stderr]]", jobPath(jobName), buildNumber)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var report TestReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse test report: %w", err)
	}
	return &report, nil
}

// GetHTMLReports returns the list of HTML Publisher reports attached to
// the build. Reports are detected by scanning the build's actions[] for
// any entry that has both urlName and reportName populated, which is
// what htmlpublisher.HtmlPublisherTarget / HtmlPublisherBuildAction
// surface in the JSON tree. Other plugins that follow the same shape
// (e.g. some pytest-html targets) are picked up automatically.
func (c *Client) GetHTMLReports(ctx context.Context, jobName string, buildNumber int) ([]HTMLReport, error) {
	path := fmt.Sprintf("%s/%d/api/json?tree=actions[_class,urlName,reportName,reportTitles]", jobPath(jobName), buildNumber)
	data, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Actions []struct {
			URLName    string `json:"urlName"`
			ReportName string `json:"reportName"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse build actions: %w", err)
	}

	out := make([]HTMLReport, 0, len(payload.Actions))
	for _, a := range payload.Actions {
		if a.URLName == "" || a.ReportName == "" {
			continue
		}
		out = append(out, HTMLReport{URLName: a.URLName, ReportName: a.ReportName})
	}
	return out, nil
}

// DownloadArtifact downloads an artifact and returns its content.
func (c *Client) DownloadArtifact(ctx context.Context, jobName string, buildNumber int, relativePath string) ([]byte, error) {
	path := fmt.Sprintf("%s/%d/artifact/%s", jobPath(jobName), buildNumber, relativePath)
	return c.getRaw(ctx, path)
}

// getRaw performs a GET request and returns raw bytes (not expecting JSON).
func (c *Client) getRaw(ctx context.Context, path string) ([]byte, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// GetArtifactURL returns the full URL for downloading an artifact.
func (c *Client) GetArtifactURL(jobName string, buildNumber int, relativePath string) string {
	return fmt.Sprintf("%s%s/%d/artifact/%s", c.baseURL, jobPath(jobName), buildNumber, relativePath)
}
