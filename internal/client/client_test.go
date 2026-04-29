// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package client_test

import (
	"context"
	"testing"

	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/client/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetJobs_FlatAndFolders(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{Color: "blue", LastBuildNumber: 1, LastBuildResult: "SUCCESS"}).
		WithJob("world", mock.JobOpts{Color: "red", LastBuildNumber: 2, LastBuildResult: "FAILURE"}).
		WithFolder("team-a")

	c := mock.NewClient(t, srv)

	jobs, err := c.GetJobs(context.Background())
	require.NoError(t, err)
	assert.Len(t, jobs, 3)

	byName := map[string]struct {
		isFolder bool
		color    string
	}{}
	for _, j := range jobs {
		byName[j.Name] = struct {
			isFolder bool
			color    string
		}{j.IsFolder(), j.Color}
	}
	assert.False(t, byName["hello"].isFolder)
	assert.Equal(t, "blue", byName["hello"].color)
	assert.False(t, byName["world"].isFolder)
	assert.True(t, byName["team-a"].isFolder)
}

func TestClient_GetFolderJobs_NestedFolders(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithFolder("team-a").
		WithJobInFolder("team-a", "deploy", mock.JobOpts{Color: "blue"}).
		WithFolderInFolder("team-a", "sub").
		WithJobInFolder("team-a/sub", "nightly", mock.JobOpts{Color: "yellow"})

	c := mock.NewClient(t, srv)

	teamA, err := c.GetFolderJobs(context.Background(), "team-a")
	require.NoError(t, err)
	assert.Len(t, teamA, 2, "team-a should contain 'deploy' and 'sub' folder")

	sub, err := c.GetFolderJobs(context.Background(), "team-a/sub")
	require.NoError(t, err)
	require.Len(t, sub, 1)
	assert.Equal(t, "nightly", sub[0].Name)
	assert.Equal(t, "yellow", sub[0].Color)
}

func TestClient_GetBuildConsoleOutputFull(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{Color: "blue", LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS", Console: "line-1\nline-2\nline-3\n"})

	c := mock.NewClient(t, srv)

	got, err := c.GetBuildConsoleOutputFull(context.Background(), "hello", 1)
	require.NoError(t, err)
	assert.Equal(t, "line-1\nline-2\nline-3\n", got)
}

// TestClient_StreamLiveBuild covers the Jenkins progressiveText streaming
// contract: each call returns the new chunk plus an X-More-Data header that
// only flips to "false" after the live chunk queue drains. The cumulative
// log is then re-fetchable from offset 0 (mimicking the j9s "go to
// beginning" / `<0>` flow in LogsView).
func TestClient_StreamLiveBuild(t *testing.T) {
	chunks := []string{"chunk-a\n", "chunk-b\n", "chunk-c\n"}
	srv := mock.NewJenkinsServer(t).
		WithJob("live", mock.JobOpts{Color: "blue_anime", LastBuildNumber: 5}).
		WithLiveBuild("live", 5, chunks)

	c := mock.NewClient(t, srv)

	var (
		got      string
		offset   int64
		moreData bool
		err      error
	)

	// 1st call from offset 0 — delivers chunk-a, more-data=true.
	got, offset, moreData, err = c.StreamBuildConsoleOutput(context.Background(), "live", 5, 0)
	require.NoError(t, err)
	assert.Equal(t, "chunk-a\n", got)
	assert.True(t, moreData)
	assert.EqualValues(t, len("chunk-a\n"), offset)

	// 2nd call from previous offset — delivers chunk-b.
	got, offset, moreData, err = c.StreamBuildConsoleOutput(context.Background(), "live", 5, offset)
	require.NoError(t, err)
	assert.Equal(t, "chunk-b\n", got)
	assert.True(t, moreData)

	// 3rd call — delivers chunk-c, queue drains, more-data flips.
	got, offset, moreData, err = c.StreamBuildConsoleOutput(context.Background(), "live", 5, offset)
	require.NoError(t, err)
	assert.Equal(t, "chunk-c\n", got)
	assert.False(t, moreData, "queue drained; build complete")

	// Replay from offset 0 — full cumulative log returned (this is what
	// LogsView.goToBeginning does).
	full, _, _, err := c.StreamBuildConsoleOutput(context.Background(), "live", 5, 0)
	require.NoError(t, err)
	assert.Equal(t, "chunk-a\nchunk-b\nchunk-c\n", full)
}

// TestClient_GetBuildConsoleSize_LiveBuild verifies that the high-offset
// probe used by LogsView's tail mode returns the cumulative size and the
// streaming flag.
func TestClient_GetBuildConsoleSize_LiveBuild(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("live", mock.JobOpts{Color: "blue_anime", LastBuildNumber: 7}).
		WithLiveBuild("live", 7, []string{"hello\n", "world\n"})

	c := mock.NewClient(t, srv)

	size, more, err := c.GetBuildConsoleSize(context.Background(), "live", 7)
	require.NoError(t, err)
	// Probe (start beyond size) does not advance the live queue, but the
	// buffer still reflects whatever has been delivered (none yet) — size
	// should be 0 and more-data should be true (build still streaming).
	assert.EqualValues(t, 0, size)
	assert.True(t, more)
}

// TestClient_GetTestReport_NestedJob exercises the full JUnit roundtrip:
// the mock returns a TestReport with one suite and three cases of mixed
// status; the client decodes them and the case-level fields (status,
// className, error stack) survive intact.
func TestClient_GetTestReport_NestedJob(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithFolder("team-a").
		WithJobInFolder("team-a", "deploy", mock.JobOpts{LastBuildNumber: 3}).
		WithBuild("team-a/deploy", 3, mock.BuildOpts{Result: "UNSTABLE"}).
		WithTestReport("team-a/deploy", 3, client.TestReport{
			FailCount: 1,
			PassCount: 1,
			SkipCount: 1,
			Suites: []client.TestSuite{{
				Name:     "tests.unit",
				Duration: 0.42,
				Cases: []client.TestCase{
					{ClassName: "tests.unit.A", Name: "test_ok", Status: "PASSED", Duration: 0.1},
					{ClassName: "tests.unit.A", Name: "test_skip", Status: "SKIPPED", Skipped: true, SkippedMessage: "wip"},
					{ClassName: "tests.unit.B", Name: "test_fail", Status: "FAILED",
						ErrorDetails:    "AssertionError: nope",
						ErrorStackTrace: "Traceback...\n  File \"foo.py\""},
				},
			}},
		})

	c := mock.NewClient(t, srv)

	report, err := c.GetTestReport(context.Background(), "team-a/deploy", 3)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, 1, report.FailCount)
	assert.Equal(t, 1, report.PassCount)
	assert.Equal(t, 1, report.SkipCount)
	require.Len(t, report.Suites, 1)
	require.Len(t, report.Suites[0].Cases, 3)

	failed := report.Suites[0].Cases[2]
	assert.Equal(t, "test_fail", failed.Name)
	assert.Equal(t, "FAILED", failed.Status)
	assert.Equal(t, "AssertionError: nope", failed.ErrorDetails)
	assert.Contains(t, failed.ErrorStackTrace, "Traceback")
}

// TestClient_GetTestReport_NoReport asserts the (nil, nil) "no report"
// contract so that view code can distinguish missing reports from
// genuine failures.
func TestClient_GetTestReport_NoReport(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS"})

	c := mock.NewClient(t, srv)

	report, err := c.GetTestReport(context.Background(), "hello", 1)
	require.NoError(t, err)
	assert.Nil(t, report)
}

// TestClient_GetHTMLReports parses the HTML Publisher actions[] entries
// out of a build doc, ignores actions that are missing reportName or
// urlName, and preserves order.
func TestClient_GetHTMLReports(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS"}).
		WithHTMLReport("hello", 1, "pytest_html", "Pytest Report").
		WithHTMLReport("hello", 1, "allure", "Allure Report")

	c := mock.NewClient(t, srv)

	reports, err := c.GetHTMLReports(context.Background(), "hello", 1)
	require.NoError(t, err)
	require.Len(t, reports, 2)
	assert.Equal(t, "pytest_html", reports[0].URLName)
	assert.Equal(t, "Pytest Report", reports[0].ReportName)
	assert.Equal(t, "allure", reports[1].URLName)
}

// TestClient_GetHTMLReports_NoReports returns an empty slice (not nil),
// which is what the HTMLReportsView relies on to render "no reports".
func TestClient_GetHTMLReports_NoReports(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS"})

	c := mock.NewClient(t, srv)

	reports, err := c.GetHTMLReports(context.Background(), "hello", 1)
	require.NoError(t, err)
	assert.Empty(t, reports)
}
