// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"testing"

	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/client"
	"github.com/rplevka/j9s/internal/client/mock"
	"github.com/rplevka/j9s/internal/config"
	"github.com/rplevka/j9s/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTMLReportsView_RendersFromMockBuildActions feeds two HTML
// Publisher actions through the fake server and asserts both reports
// land in the rendered table in registration order.
func TestHTMLReportsView_RendersFromMockBuildActions(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "SUCCESS"}).
		WithHTMLReport("hello", 1, "pytest_html", "Pytest Report").
		WithHTMLReport("hello", 1, "allure", "Allure")

	c := mock.NewClient(t, srv)
	reports, err := c.GetHTMLReports(context.Background(), "hello", 1)
	require.NoError(t, err)
	require.Len(t, reports, 2)

	v := &HTMLReportsView{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		table:   ui.NewTable(),
		actions: ui.NewKeyActions(),
		reports: reports,
	}
	v.renderReports()

	assert.Equal(t, 2, v.table.RowCount())
	urls := map[string]bool{}
	for i := 0; i < v.table.RowCount(); i++ {
		v.table.Select(i+1, 0)
		urls[v.table.GetSelectedID()] = true
	}
	assert.True(t, urls["pytest_html"])
	assert.True(t, urls["allure"])
}

// TestHTMLReportsView_URLProvider asserts the URL/path pair surfaced
// for <u>: with no row selected, the build's root URL; with a row
// selected, the per-report URL with the urlName slug appended.
func TestHTMLReportsView_URLProvider(t *testing.T) {
	cfg := config.NewConfig()
	cfg.J9s.Contexts = []config.Context{{Name: "test", URL: "https://jenkins.example.com"}}
	cfg.J9s.CurrentContext = "test"
	app := NewApp(cfg)
	app.App.Init()

	v := &HTMLReportsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  "Folder/MyJob",
		buildNum: 3,
	}
	// Empty selection -> build root.
	assert.Equal(t, "reports/Folder/MyJob/3", v.GetViewPath())
	assert.Equal(t, "https://jenkins.example.com/job/Folder/job/MyJob/3/", v.GetJenkinsURL())

	// With one rendered + selected report.
	v.reports = []client.HTMLReport{{URLName: "pytest_html", ReportName: "Pytest"}}
	v.renderReports()
	v.table.Select(1, 0)
	assert.Equal(t, "reports/Folder/MyJob/3/pytest_html", v.GetViewPath())
	assert.Equal(t, "https://jenkins.example.com/job/Folder/job/MyJob/3/pytest_html/", v.GetJenkinsURL())

	var _ URLProvider = (*HTMLReportsView)(nil)
}

// TestHTMLReportsView_OpenCmd_UsesOpenURLFunc swaps openURLFunc with a
// recorder and asserts the per-report URL is forwarded to it (no real
// browser is launched).
func TestHTMLReportsView_OpenCmd_UsesOpenURLFunc(t *testing.T) {
	cfg := config.NewConfig()
	cfg.J9s.Contexts = []config.Context{{Name: "test", URL: "https://jenkins.example.com"}}
	cfg.J9s.CurrentContext = "test"
	app := NewApp(cfg)
	app.App.Init()

	original := openURLFunc
	t.Cleanup(func() { openURLFunc = original })

	var captured string
	openURLFunc = func(url string) error {
		captured = url
		return nil
	}

	v := &HTMLReportsView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  "Folder/MyJob",
		buildNum: 3,
		reports:  []client.HTMLReport{{URLName: "pytest_html", ReportName: "Pytest"}},
	}
	v.renderReports()
	v.table.Select(1, 0)

	v.openCmd(nil)
	assert.Equal(t, "https://jenkins.example.com/job/Folder/job/MyJob/3/pytest_html/", captured)
}
