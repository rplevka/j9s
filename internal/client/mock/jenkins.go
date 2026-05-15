// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

// Package mock provides a fake Jenkins HTTP server for use in tests.
//
// It is intentionally placed in a non-_test package so that tests in any
// package (client, view, ...) can import it. Inspired by the k9s
// internal/config/mock package and internal/client/switch_context_test
// pattern of using httptest.NewServer + http.ServeMux to fake an upstream API.
package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/rplevka/j9s/internal/client"
)

// JobOpts customises a job entry on the fake server.
type JobOpts struct {
	// Class is the Jenkins _class string. If empty, defaults to a freestyle
	// project; for folders use mock.FolderClass via WithFolder.
	Class string
	Color string
	// LastBuildNumber, when >0, is reported as the job's lastBuild.
	LastBuildNumber int
	LastBuildResult string
}

// BuildOpts customises a build entry on the fake server.
type BuildOpts struct {
	Result    string
	Building  bool
	Duration  int64
	Timestamp int64
	// Console is the canned full console text returned by /consoleText.
	Console string
}

const (
	// FreestyleClass is the default Jenkins job class.
	FreestyleClass = "hudson.model.FreeStyleProject"
	// FolderClass is the Jenkins folder class.
	FolderClass = "com.cloudbees.hudson.plugins.folder.Folder"
)

// streamChunk represents one chunk delivered by progressiveText. The chunk
// is appended to the cumulative log buffer and the X-Text-Size header is
// advanced by len(chunk). After the queue drains, the server reports
// X-More-Data=false.
type streamChunk struct {
	body []byte
}

type buildState struct {
	opts BuildOpts
	// liveChunks, when non-nil, makes /logText/progressiveText return one
	// chunk per request and emit X-More-Data=true until the queue is empty.
	// Each delivered chunk is also appended to the cumulative buffer used
	// for subsequent /consoleText calls and for replays at offset 0.
	liveChunks []streamChunk
	// buf is the full log accumulated so far (canned Console + delivered
	// live chunks).
	buf []byte
	// testReport, when non-nil, is served at /testReport/api/json.
	testReport *client.TestReport
	// htmlReports are surfaced as actions[] entries on the build JSON.
	htmlReports []client.HTMLReport
	// pipelineNodes is the canned response for the Blue Ocean
	// /blue/rest/.../runs/<num>/nodes/ endpoint.
	pipelineNodes []client.BlueNode
	// pipelineSteps maps nodeID -> canned step list for /nodes/<id>/steps/.
	pipelineSteps map[string][]client.BlueStep
	// pipelineNodeLogs maps nodeID -> canned text returned by /nodes/<id>/log/.
	pipelineNodeLogs map[string]string
	// pipelineStepLogs maps "nodeID/stepID" -> canned text for /steps/<id>/log/.
	pipelineStepLogs map[string]string
}

// JenkinsServer is a fluent fake Jenkins instance backed by httptest.Server.
type JenkinsServer struct {
	t       testing.TB
	server  *httptest.Server
	mu      sync.Mutex
	jobs    map[string]*JobOpts            // fullName -> opts
	builds  map[string]map[int]*buildState // fullName -> buildNum -> state
	folders map[string]bool                // fullName -> true if folder
	views   map[string]map[string]bool     // owner ("" = root or folder fullName) -> viewName -> true
	calls   map[string]int                 // path -> call count, for assertions
}

// NewJenkinsServer constructs a fake Jenkins server. Call Close on the
// returned server (or rely on t.Cleanup) when done.
func NewJenkinsServer(t testing.TB) *JenkinsServer {
	t.Helper()
	js := &JenkinsServer{
		t:       t,
		jobs:    make(map[string]*JobOpts),
		builds:  make(map[string]map[int]*buildState),
		folders: make(map[string]bool),
		views:   make(map[string]map[string]bool),
		calls:   make(map[string]int),
	}
	js.server = httptest.NewServer(http.HandlerFunc(js.handle))
	t.Cleanup(js.server.Close)
	return js
}

// URL returns the base URL of the fake server.
func (j *JenkinsServer) URL() string { return j.server.URL }

// Close stops the underlying httptest server. Idempotent.
func (j *JenkinsServer) Close() { j.server.Close() }

// Calls returns the number of times a given path (without query) has been
// hit. Useful for tests that want to assert behaviour.
func (j *JenkinsServer) Calls(path string) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.calls[path]
}

// WithJob registers a top-level job.
func (j *JenkinsServer) WithJob(name string, opts JobOpts) *JenkinsServer {
	return j.withJobAt("", name, opts)
}

// WithFolder registers a top-level folder.
func (j *JenkinsServer) WithFolder(name string) *JenkinsServer {
	return j.withFolderAt("", name)
}

// WithJobInFolder registers a job inside an existing folder path (e.g.
// "team-a" or "team-a/sub").
func (j *JenkinsServer) WithJobInFolder(folder, name string, opts JobOpts) *JenkinsServer {
	return j.withJobAt(folder, name, opts)
}

// WithFolderInFolder registers a nested folder.
func (j *JenkinsServer) WithFolderInFolder(parent, name string) *JenkinsServer {
	return j.withFolderAt(parent, name)
}

// WithBuild registers a completed build with a canned full console output.
func (j *JenkinsServer) WithBuild(jobFullName string, num int, opts BuildOpts) *JenkinsServer {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, ok := j.builds[jobFullName]; !ok {
		j.builds[jobFullName] = make(map[int]*buildState)
	}
	st := &buildState{opts: opts}
	if opts.Console != "" {
		st.buf = []byte(opts.Console)
	}
	j.builds[jobFullName][num] = st
	return j
}

// WithLiveBuild registers a build whose progressiveText endpoint will deliver
// one chunk per request, reporting X-More-Data=true until the queue drains.
// Each chunk is also appended to the cumulative log so that re-fetching from
// offset 0 (or via /consoleText) returns the full text.
func (j *JenkinsServer) WithLiveBuild(jobFullName string, num int, chunks []string) *JenkinsServer {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, ok := j.builds[jobFullName]; !ok {
		j.builds[jobFullName] = make(map[int]*buildState)
	}
	live := make([]streamChunk, 0, len(chunks))
	for _, c := range chunks {
		live = append(live, streamChunk{body: []byte(c)})
	}
	j.builds[jobFullName][num] = &buildState{
		opts:       BuildOpts{Building: true, Result: ""},
		liveChunks: live,
	}
	return j
}

// WithTestReport attaches a JUnit-plugin test report to a previously
// registered build. The build must already exist (use WithBuild first).
// Pass-through helper around the typed client.TestReport so tests can
// build whatever shape they need.
func (j *JenkinsServer) WithTestReport(jobFullName string, num int, report client.TestReport) *JenkinsServer {
	j.mu.Lock()
	defer j.mu.Unlock()
	bs := j.builds[jobFullName]
	if bs == nil {
		j.t.Fatalf("WithTestReport: build %s#%d not registered", jobFullName, num)
	}
	st := bs[num]
	if st == nil {
		j.t.Fatalf("WithTestReport: build %s#%d not registered", jobFullName, num)
	}
	cp := report
	st.testReport = &cp
	return j
}

// WithHTMLReport adds one HTML Publisher report to a build's actions[].
// urlName is the path segment under the build URL; reportName is the
// human-friendly title.
func (j *JenkinsServer) WithHTMLReport(jobFullName string, num int, urlName, reportName string) *JenkinsServer {
	j.mu.Lock()
	defer j.mu.Unlock()
	bs := j.builds[jobFullName]
	if bs == nil {
		j.t.Fatalf("WithHTMLReport: build %s#%d not registered", jobFullName, num)
	}
	st := bs[num]
	if st == nil {
		j.t.Fatalf("WithHTMLReport: build %s#%d not registered", jobFullName, num)
	}
	st.htmlReports = append(st.htmlReports, client.HTMLReport{URLName: urlName, ReportName: reportName})
	return j
}

// WithPipelineNodes attaches the canned Blue Ocean DAG node list to a
// previously registered build.
func (j *JenkinsServer) WithPipelineNodes(jobFullName string, num int, nodes []client.BlueNode) *JenkinsServer {
	st := j.requireBuild("WithPipelineNodes", jobFullName, num)
	st.pipelineNodes = nodes
	return j
}

// WithPipelineNodeSteps attaches a canned per-node step list.
func (j *JenkinsServer) WithPipelineNodeSteps(jobFullName string, num int, nodeID string, steps []client.BlueStep) *JenkinsServer {
	st := j.requireBuild("WithPipelineNodeSteps", jobFullName, num)
	if st.pipelineSteps == nil {
		st.pipelineSteps = make(map[string][]client.BlueStep)
	}
	st.pipelineSteps[nodeID] = steps
	return j
}

// WithPipelineNodeLog attaches a canned log body for /nodes/<id>/log/.
// X-Text-Size is set from len(text); X-More-Data is always false.
func (j *JenkinsServer) WithPipelineNodeLog(jobFullName string, num int, nodeID, text string) *JenkinsServer {
	st := j.requireBuild("WithPipelineNodeLog", jobFullName, num)
	if st.pipelineNodeLogs == nil {
		st.pipelineNodeLogs = make(map[string]string)
	}
	st.pipelineNodeLogs[nodeID] = text
	return j
}

// WithPipelineStepLog attaches a canned log body for
// /nodes/<nodeID>/steps/<stepID>/log/.
func (j *JenkinsServer) WithPipelineStepLog(jobFullName string, num int, nodeID, stepID, text string) *JenkinsServer {
	st := j.requireBuild("WithPipelineStepLog", jobFullName, num)
	if st.pipelineStepLogs == nil {
		st.pipelineStepLogs = make(map[string]string)
	}
	st.pipelineStepLogs[nodeID+"/"+stepID] = text
	return j
}

// requireBuild returns the buildState pointer or fatals the test.
// Used by all fluent attach-to-build helpers (test/HTML report, blue
// ocean) to share the same diagnostic message shape.
func (j *JenkinsServer) requireBuild(method, jobFullName string, num int) *buildState {
	j.mu.Lock()
	defer j.mu.Unlock()
	bs := j.builds[jobFullName]
	if bs == nil {
		j.t.Fatalf("%s: build %s#%d not registered", method, jobFullName, num)
	}
	st := bs[num]
	if st == nil {
		j.t.Fatalf("%s: build %s#%d not registered", method, jobFullName, num)
	}
	return st
}

// WithView registers a Jenkins view at the given folder ("" = root).
func (j *JenkinsServer) WithView(folder, name string) *JenkinsServer {
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, ok := j.views[folder]; !ok {
		j.views[folder] = make(map[string]bool)
	}
	j.views[folder][name] = true
	return j
}

func (j *JenkinsServer) withJobAt(folder, name string, opts JobOpts) *JenkinsServer {
	j.mu.Lock()
	defer j.mu.Unlock()
	if opts.Class == "" {
		opts.Class = FreestyleClass
	}
	full := name
	if folder != "" {
		full = folder + "/" + name
	}
	cp := opts
	j.jobs[full] = &cp
	return j
}

func (j *JenkinsServer) withFolderAt(parent, name string) *JenkinsServer {
	j.mu.Lock()
	defer j.mu.Unlock()
	full := name
	if parent != "" {
		full = parent + "/" + name
	}
	j.folders[full] = true
	j.jobs[full] = &JobOpts{Class: FolderClass}
	return j
}

// childrenOf returns the immediate children of the given folder ("" = root).
func (j *JenkinsServer) childrenOf(folder string) []client.Job {
	prefix := ""
	if folder != "" {
		prefix = folder + "/"
	}
	var out []client.Job
	for full, opts := range j.jobs {
		if !strings.HasPrefix(full, prefix) {
			continue
		}
		rest := strings.TrimPrefix(full, prefix)
		if rest == "" || strings.Contains(rest, "/") {
			continue
		}
		out = append(out, j.jobToClient(rest, full, opts))
	}
	return out
}

func (j *JenkinsServer) jobToClient(name, full string, opts *JobOpts) client.Job {
	job := client.Job{
		Class:    opts.Class,
		Name:     name,
		FullName: full,
		URL:      j.server.URL + "/job/" + jenkinsURLPath(full) + "/",
		Color:    opts.Color,
	}
	if opts.LastBuildNumber > 0 {
		job.LastBuild = &client.Build{
			Number: opts.LastBuildNumber,
			Result: opts.LastBuildResult,
		}
	}
	return job
}

func jenkinsURLPath(full string) string {
	parts := strings.Split(full, "/")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = url.PathEscape(p)
	}
	return strings.Join(out, "/job/")
}

func (j *JenkinsServer) handle(w http.ResponseWriter, r *http.Request) {
	j.mu.Lock()
	j.calls[r.URL.Path]++
	j.mu.Unlock()

	switch {
	case r.URL.Path == "/crumbIssuer/api/json":
		writeJSON(w, client.Crumb{RequestField: "Jenkins-Crumb", Value: "test"})
		return
	case r.URL.Path == "/api/json":
		j.writeFolderJSON(w, "")
		return
	case strings.HasPrefix(r.URL.Path, "/view/") && strings.HasSuffix(r.URL.Path, "/api/json"):
		// /view/{name}/api/json — root view
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/view/"), "/api/json")
		j.writeViewJSON(w, "", name)
		return
	case strings.HasPrefix(r.URL.Path, "/job/"):
		j.handleJobScoped(w, r)
		return
	case strings.HasPrefix(r.URL.Path, "/blue/rest/organizations/jenkins/pipelines/"):
		j.handleBlueOcean(w, r)
		return
	}
	http.NotFound(w, r)
}

// handleBlueOcean dispatches /blue/rest/organizations/jenkins/pipelines/...
// requests. Decodes the nested "pipelines/<seg>" path back into a job
// fullName, then dispatches by suffix:
//
//	runs/<num>/nodes/                              -> pipeline node list
//	runs/<num>/nodes/<nodeID>/steps/               -> step list
//	runs/<num>/nodes/<nodeID>/log/                 -> aggregated node log
//	runs/<num>/nodes/<nodeID>/steps/<stepID>/log/  -> per-step log
func (j *JenkinsServer) handleBlueOcean(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/blue/rest/organizations/jenkins/pipelines/")
	full, suffix := decodeBluePipelinePath(rest)

	if !strings.HasPrefix(suffix, "runs/") {
		http.NotFound(w, r)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(suffix, "runs/"), "/", 2)
	num, err := strconv.Atoi(parts[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 1 {
		http.NotFound(w, r)
		return
	}
	tail := strings.TrimSuffix(parts[1], "/")

	j.mu.Lock()
	st := j.lookupBuild(full, num)
	j.mu.Unlock()
	if st == nil {
		http.NotFound(w, r)
		return
	}

	switch {
	case tail == "nodes":
		j.mu.Lock()
		nodes := st.pipelineNodes
		j.mu.Unlock()
		if nodes == nil {
			nodes = []client.BlueNode{}
		}
		writeJSON(w, nodes)
	case strings.HasPrefix(tail, "nodes/"):
		j.handleBlueNodeSubresource(w, st, strings.TrimPrefix(tail, "nodes/"))
	default:
		http.NotFound(w, r)
	}
}

// handleBlueNodeSubresource dispatches the per-node Blue Ocean URLs.
// rest is the path AFTER "nodes/" — e.g. "13/steps", "13/log",
// "13/steps/21/log".
func (j *JenkinsServer) handleBlueNodeSubresource(w http.ResponseWriter, st *buildState, rest string) {
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		http.NotFound(w, nil)
		return
	}
	nodeID := parts[0]

	switch {
	case len(parts) == 2 && parts[1] == "steps":
		j.mu.Lock()
		steps := st.pipelineSteps[nodeID]
		j.mu.Unlock()
		if steps == nil {
			steps = []client.BlueStep{}
		}
		writeJSON(w, steps)
	case len(parts) == 2 && parts[1] == "log":
		j.mu.Lock()
		text := st.pipelineNodeLogs[nodeID]
		j.mu.Unlock()
		writeBlueLog(w, text)
	case len(parts) == 4 && parts[1] == "steps" && parts[3] == "log":
		stepID := parts[2]
		j.mu.Lock()
		text := st.pipelineStepLogs[nodeID+"/"+stepID]
		j.mu.Unlock()
		writeBlueLog(w, text)
	default:
		http.NotFound(w, nil)
	}
}

// decodeBluePipelinePath converts "team-a/pipelines/sub/pipelines/deploy/runs/3/nodes/"
// into fullName="team-a/sub/deploy" and suffix="runs/3/nodes". The Blue
// Ocean URL scheme alternates "<segment>/pipelines/<segment>" up to the
// final pipeline name, then optional run/operation suffix.
func decodeBluePipelinePath(rest string) (fullName, suffix string) {
	parts := strings.Split(rest, "/")
	var nameParts []string
	i := 0
	for i < len(parts) {
		nameParts = append(nameParts, parts[i])
		i++
		if i < len(parts) && parts[i] == "pipelines" {
			i++
			continue
		}
		break
	}
	fullName = strings.Join(nameParts, "/")
	if i < len(parts) {
		suffix = strings.Join(parts[i:], "/")
	}
	return
}

// writeBlueLog mirrors the Blue Ocean log endpoint: text body plus
// X-Text-Size header (= len) and X-More-Data: false (the mock never
// streams). Returns 200 with empty body when text is empty so callers
// can distinguish "no log yet" from "node missing".
func writeBlueLog(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Text-Size", strconv.Itoa(len(text)))
	w.Header().Set("X-More-Data", "false")
	_, _ = w.Write([]byte(text))
}

// handleJobScoped dispatches /job/... requests, decoding the embedded path
// up to the suffix that determines the operation (api/json, consoleText,
// logText/progressiveText, view/.../api/json, etc.).
func (j *JenkinsServer) handleJobScoped(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/job/")
	full, suffix := decodeJobPath(rest)

	switch {
	case suffix == "api/json":
		// Either folder listing or job detail; we serve folder listing for
		// any registered folder and a thin job document otherwise.
		j.mu.Lock()
		isFolder := j.folders[full]
		_, isJob := j.jobs[full]
		j.mu.Unlock()
		if isFolder {
			j.writeFolderJSON(w, full)
			return
		}
		if isJob {
			j.writeJobJSON(w, full)
			return
		}
		http.NotFound(w, r)
		return
	case strings.HasPrefix(suffix, "view/") && strings.HasSuffix(suffix, "/api/json"):
		viewName := strings.TrimSuffix(strings.TrimPrefix(suffix, "view/"), "/api/json")
		j.writeViewJSON(w, full, viewName)
		return
	}

	// Build-scoped: <num>/api/json, <num>/consoleText, <num>/logText/progressiveText
	num, buildSuffix, ok := splitBuildSuffix(suffix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch buildSuffix {
	case "api/json":
		j.writeBuildJSON(w, full, num)
	case "testReport/api/json":
		j.writeTestReportJSON(w, full, num)
	case "consoleText":
		j.writeConsoleText(w, full, num)
	case "logText/progressiveText":
		start, _ := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
		j.writeProgressiveText(w, full, num, start)
	default:
		http.NotFound(w, r)
	}
}

// decodeJobPath converts "team-a/job/deploy/5/consoleText" into
// fullName="team-a/deploy" and suffix="5/consoleText". The Jenkins job URL
// scheme alternates "<segment>/job/<segment>" for folders, ending with the
// final job name, then optional build/operation suffix.
func decodeJobPath(rest string) (fullName, suffix string) {
	parts := strings.Split(rest, "/")
	var nameParts []string
	i := 0
	for i < len(parts) {
		nameParts = append(nameParts, parts[i])
		i++
		// Next must be "job" to continue the folder path.
		if i < len(parts) && parts[i] == "job" {
			i++
			continue
		}
		break
	}
	fullName = strings.Join(nameParts, "/")
	if i < len(parts) {
		suffix = strings.Join(parts[i:], "/")
	}
	return
}

// splitBuildSuffix peels off the leading build number from "5/consoleText"
// or "5/logText/progressiveText" or "5/api/json".
func splitBuildSuffix(suffix string) (int, string, bool) {
	if suffix == "" {
		return 0, "", false
	}
	parts := strings.SplitN(suffix, "/", 2)
	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	if len(parts) == 1 {
		return num, "", true
	}
	return num, parts[1], true
}

func (j *JenkinsServer) writeFolderJSON(w http.ResponseWriter, folder string) {
	jobs := j.childrenOf(folder)
	views := j.viewsOf(folder)
	writeJSON(w, struct {
		Jobs  []client.Job  `json:"jobs"`
		Views []client.View `json:"views,omitempty"`
	}{Jobs: jobs, Views: views})
}

// viewsOf returns the views registered for a given folder ("" = root).
func (j *JenkinsServer) viewsOf(folder string) []client.View {
	j.mu.Lock()
	defer j.mu.Unlock()
	owner := j.views[folder]
	if owner == nil {
		return nil
	}
	out := make([]client.View, 0, len(owner))
	for name := range owner {
		out = append(out, client.View{Class: "hudson.model.ListView", Name: name})
	}
	return out
}

func (j *JenkinsServer) writeJobJSON(w http.ResponseWriter, full string) {
	j.mu.Lock()
	opts := j.jobs[full]
	bs := j.builds[full]
	j.mu.Unlock()
	parts := strings.Split(full, "/")
	job := j.jobToClient(parts[len(parts)-1], full, opts)
	// Include any registered builds so client.GetBuilds (which reads
	// /job/{name}/api/json?tree=builds[...]) can find them.
	for num, st := range bs {
		job.Builds = append(job.Builds, client.Build{
			Number:    num,
			Result:    st.opts.Result,
			Building:  st.opts.Building,
			Duration:  st.opts.Duration,
			Timestamp: st.opts.Timestamp,
		})
	}
	writeJSON(w, job)
}

func (j *JenkinsServer) writeViewJSON(w http.ResponseWriter, folder, name string) {
	j.mu.Lock()
	owner := j.views[folder]
	exists := owner != nil && owner[name]
	j.mu.Unlock()
	if !exists {
		http.NotFound(w, nil)
		return
	}
	v := client.View{Class: "hudson.model.ListView", Name: name, Jobs: j.childrenOf(folder)}
	writeJSON(w, v)
}

func (j *JenkinsServer) writeBuildJSON(w http.ResponseWriter, full string, num int) {
	j.mu.Lock()
	st := j.lookupBuild(full, num)
	var htmlActions []map[string]interface{}
	if st != nil && len(st.htmlReports) > 0 {
		htmlActions = make([]map[string]interface{}, 0, len(st.htmlReports))
		for _, r := range st.htmlReports {
			htmlActions = append(htmlActions, map[string]interface{}{
				"_class":     "htmlpublisher.HtmlPublisherTarget$HTMLAction",
				"urlName":    r.URLName,
				"reportName": r.ReportName,
			})
		}
	}
	j.mu.Unlock()
	if st == nil {
		http.NotFound(w, nil)
		return
	}
	// Encode with the typed Build first, then merge in the synthetic
	// htmlpublisher actions[] entries. We sidestep client.Build's typed
	// Actions field (BuildAction with parameters/causes only) so the
	// HtmlPublisher action shape passes through verbatim.
	doc := map[string]interface{}{
		"number":    num,
		"result":    st.opts.Result,
		"building":  st.opts.Building,
		"duration":  st.opts.Duration,
		"timestamp": st.opts.Timestamp,
	}
	if htmlActions != nil {
		doc["actions"] = htmlActions
	}
	writeJSON(w, doc)
}

// writeTestReportJSON serves the JUnit /testReport/api/json document for
// a build. Returns 404 when the build has no test report attached, which
// matches Jenkins' real behavior and is what client.GetTestReport uses
// to detect "no report" without surfacing a hard error.
func (j *JenkinsServer) writeTestReportJSON(w http.ResponseWriter, full string, num int) {
	j.mu.Lock()
	st := j.lookupBuild(full, num)
	var report *client.TestReport
	if st != nil {
		report = st.testReport
	}
	j.mu.Unlock()
	if report == nil {
		http.NotFound(w, nil)
		return
	}
	writeJSON(w, report)
}

func (j *JenkinsServer) writeConsoleText(w http.ResponseWriter, full string, num int) {
	j.mu.Lock()
	st := j.lookupBuild(full, num)
	var buf []byte
	if st != nil {
		buf = append([]byte(nil), st.buf...)
	}
	j.mu.Unlock()
	if st == nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write(buf)
}

// writeProgressiveText implements Jenkins' progressiveText semantics: for
// "live" builds (queued chunks remain), one chunk is delivered per call and
// X-More-Data=true is set; once the queue drains the build is marked as
// finished and X-More-Data=false. The X-Text-Size header always reflects
// the cumulative buffer size after this call.
func (j *JenkinsServer) writeProgressiveText(w http.ResponseWriter, full string, num int, start int64) {
	j.mu.Lock()
	st := j.lookupBuild(full, num)
	if st == nil {
		j.mu.Unlock()
		http.NotFound(w, nil)
		return
	}

	// A "probe" is a request whose start offset is beyond the current
	// buffer (j9s uses start=999999999999 for this in GetBuildConsoleSize).
	// Probes must NOT advance the live queue and must NOT return a body —
	// they just expose the current size + more-data flag via headers.
	probe := start > int64(len(st.buf))
	var body []byte
	if !probe {
		// Deliver one queued chunk per non-probe call and append it to the
		// cumulative buffer. When the last chunk drains, flip the build
		// to non-building so the very next more-data check returns false.
		if len(st.liveChunks) > 0 {
			next := st.liveChunks[0]
			st.liveChunks = st.liveChunks[1:]
			st.buf = append(st.buf, next.body...)
			if len(st.liveChunks) == 0 {
				st.opts.Building = false
				if st.opts.Result == "" {
					st.opts.Result = "SUCCESS"
				}
			}
		}
		if start < 0 {
			start = 0
		}
		if start <= int64(len(st.buf)) {
			body = append([]byte(nil), st.buf[start:]...)
		}
	}

	// more-data is true while either queue is pending OR build is flagged
	// as still building.
	moreData := len(st.liveChunks) > 0 || st.opts.Building
	textSize := int64(len(st.buf))
	j.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Text-Size", strconv.FormatInt(textSize, 10))
	if moreData {
		w.Header().Set("X-More-Data", "true")
	} else {
		w.Header().Set("X-More-Data", "false")
	}
	if !probe {
		_, _ = w.Write(body)
	}
}

func (j *JenkinsServer) lookupBuild(full string, num int) *buildState {
	bs, ok := j.builds[full]
	if !ok {
		return nil
	}
	return bs[num]
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Best-effort.
		http.Error(w, fmt.Sprintf("encode: %v", err), http.StatusInternalServerError)
	}
}
