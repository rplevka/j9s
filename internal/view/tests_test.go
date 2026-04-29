// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"testing"

	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/client/mock"
	"github.com/roman-plevka/j9s/internal/config"
	"github.com/roman-plevka/j9s/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSuitesView_RendersFromMockReport wires the mock Jenkins server
// through the real client into a hand-constructed TestSuitesView and
// asserts the rendered table matches the suite roster (one row per
// suite, mixed pass/fail/skip aggregations).
func TestSuitesView_RendersFromMockReport(t *testing.T) {
	srv := mock.NewJenkinsServer(t).
		WithJob("hello", mock.JobOpts{LastBuildNumber: 1}).
		WithBuild("hello", 1, mock.BuildOpts{Result: "UNSTABLE"}).
		WithTestReport("hello", 1, client.TestReport{
			Suites: []client.TestSuite{
				{
					Name: "tests.unit", Duration: 0.5,
					Cases: []client.TestCase{
						{Name: "ok_a", Status: "PASSED", ClassName: "tests.unit.A"},
						{Name: "fail_a", Status: "FAILED", ClassName: "tests.unit.A"},
					},
				},
				{
					Name: "tests.integration", Duration: 1.2,
					Cases: []client.TestCase{
						{Name: "skip_b", Status: "SKIPPED", ClassName: "tests.integration.B"},
					},
				},
			},
		})

	c := mock.NewClient(t, srv)
	report, err := c.GetTestReport(context.Background(), "hello", 1)
	require.NoError(t, err)
	require.NotNil(t, report)

	v := &TestSuitesView{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		table:   ui.NewTable(),
		actions: ui.NewKeyActions(),
		report:  report,
	}
	v.renderSuites()

	assert.Equal(t, 2, v.table.RowCount(), "two suites registered")

	names := map[string]bool{}
	for i := 0; i < v.table.RowCount(); i++ {
		v.table.Select(i+1, 0)
		names[v.table.GetSelectedID()] = true
	}
	assert.True(t, names["tests.unit"])
	assert.True(t, names["tests.integration"])
}

// TestCasesView_RendersAllCases asserts the cases table renders one row
// per case with the test name in column 0 (so SelectByID works).
func TestCasesView_RendersAllCases(t *testing.T) {
	suite := &client.TestSuite{
		Name: "tests.unit",
		Cases: []client.TestCase{
			{Name: "a_passes", Status: "PASSED", ClassName: "tests.unit.A", Duration: 0.01},
			{Name: "b_fails", Status: "FAILED", ClassName: "tests.unit.A", Duration: 0.02,
				ErrorDetails: "boom"},
			{Name: "c_skipped", Status: "SKIPPED", ClassName: "tests.unit.A", Skipped: true},
		},
	}

	v := &TestCasesView{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		table:   ui.NewTable(),
		actions: ui.NewKeyActions(),
		suite:   suite,
	}
	v.renderCases()

	assert.Equal(t, 3, v.table.RowCount())
	names := map[string]bool{}
	for i := 0; i < v.table.RowCount(); i++ {
		v.table.Select(i+1, 0)
		names[v.table.GetSelectedID()] = true
	}
	assert.True(t, names["a_passes"])
	assert.True(t, names["b_fails"])
	assert.True(t, names["c_skipped"])
}

// TestSuitesView_URLProvider asserts the URL/path pair surfaced for <u>
// from the build-level testReport overview.
func TestSuitesView_URLProvider(t *testing.T) {
	cfg := config.NewConfig()
	cfg.J9s.Contexts = []config.Context{{Name: "test", URL: "https://jenkins.example.com"}}
	cfg.J9s.CurrentContext = "test"
	app := NewApp(cfg)
	app.App.Init()

	v := &TestSuitesView{app: app, jobName: "Folder/MyJob", buildNum: 3}

	assert.Equal(t, "tests/Folder/MyJob/3", v.GetViewPath())
	assert.Equal(t, "https://jenkins.example.com/job/Folder/job/MyJob/3/testReport/", v.GetJenkinsURL())

	var _ URLProvider = (*TestSuitesView)(nil)
}

// TestCasesView_URLProvider mirrors the suites-level URL contract: the
// per-suite URL falls back to the testReport overview because the
// JUnit-plugin URL space uses className-derived paths, not suite name.
func TestCasesView_URLProvider(t *testing.T) {
	cfg := config.NewConfig()
	cfg.J9s.Contexts = []config.Context{{Name: "test", URL: "https://jenkins.example.com"}}
	cfg.J9s.CurrentContext = "test"
	app := NewApp(cfg)
	app.App.Init()

	v := &TestCasesView{app: app, jobName: "Folder/MyJob", buildNum: 3,
		suite: &client.TestSuite{Name: "tests.unit"}}

	assert.Equal(t, "tests/Folder/MyJob/3", v.GetViewPath())
	assert.Equal(t, "https://jenkins.example.com/job/Folder/job/MyJob/3/testReport/", v.GetJenkinsURL())

	var _ URLProvider = (*TestCasesView)(nil)
}

// TestCaseDetailView_URLProvider exercises the className → package/class
// derivation used to construct the case-level Jenkins URL.
func TestCaseDetailView_URLProvider(t *testing.T) {
	cfg := config.NewConfig()
	cfg.J9s.Contexts = []config.Context{{Name: "test", URL: "https://jenkins.example.com"}}
	cfg.J9s.CurrentContext = "test"
	app := NewApp(cfg)
	app.App.Init()

	cases := []struct {
		name     string
		tcase    *client.TestCase
		wantPath string
		wantURL  string
	}{
		{
			name: "package + class + test",
			tcase: &client.TestCase{
				Name: "test_a", ClassName: "tests.unit.WidgetTest",
			},
			wantPath: "tests/Folder/MyJob/3/tests.unit/WidgetTest/test_a",
			wantURL:  "https://jenkins.example.com/job/Folder/job/MyJob/3/testReport/tests.unit/WidgetTest/test_a/",
		},
		{
			name:     "class only (no package)",
			tcase:    &client.TestCase{Name: "test_b", ClassName: "Bare"},
			wantPath: "tests/Folder/MyJob/3/(root)/Bare/test_b",
			wantURL:  "https://jenkins.example.com/job/Folder/job/MyJob/3/testReport/(root)/Bare/test_b/",
		},
		{
			name:     "missing class falls back to overview",
			tcase:    &client.TestCase{Name: "test_c"},
			wantPath: "tests/Folder/MyJob/3",
			wantURL:  "https://jenkins.example.com/job/Folder/job/MyJob/3/testReport/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := &TestCaseDetailView{app: app, jobName: "Folder/MyJob", buildNum: 3, tcase: tc.tcase}
			assert.Equal(t, tc.wantPath, v.GetViewPath())
			assert.Equal(t, tc.wantURL, v.GetJenkinsURL())
		})
	}

	var _ URLProvider = (*TestCaseDetailView)(nil)
}

// TestSplitClassName guards the dotted-name → (package, class) split
// used to construct case-level URLs.
func TestSplitClassName(t *testing.T) {
	cases := []struct {
		in       string
		wantPkg  string
		wantCls  string
	}{
		{"tests.unit.WidgetTest", "tests.unit", "WidgetTest"},
		{"WidgetTest", "", "WidgetTest"},
		{"", "", ""},
	}
	for _, c := range cases {
		gotPkg, gotCls := splitClassName(c.in)
		assert.Equal(t, c.wantPkg, gotPkg, "pkg for %q", c.in)
		assert.Equal(t, c.wantCls, gotCls, "class for %q", c.in)
	}
}
