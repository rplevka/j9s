// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// -----------------------------------------------------------------------
// TestSuitesView — first level of the JUnit drill-in (build → suites).
// Picks up /testReport/api/json once and slices the suites locally so
// switching between Suites and Cases is instant.
// -----------------------------------------------------------------------

// TestSuitesView lists the JUnit suites attached to a build.
type TestSuitesView struct {
	*tview.Flex
	app      *App
	table    *ui.Table
	actions  *ui.KeyActions
	jobName  string
	buildNum int
	report   *client.TestReport
}

// NewTestSuitesView returns a new test-suites view for a build.
func NewTestSuitesView(app *App, jobName string, buildNum int) *TestSuitesView {
	v := &TestSuitesView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  jobName,
		buildNum: buildNum,
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.refresh()
	return v
}

// Name returns the view name (used by Crumbs).
func (v *TestSuitesView) Name() string {
	return fmt.Sprintf("Tests[%s#%d]", v.jobName, v.buildNum)
}

// Hints returns the menu hints.
func (v *TestSuitesView) Hints() model.MenuHints { return v.actions.Hints() }

// SetFilter forwards the filter to the table.
func (v *TestSuitesView) SetFilter(filter string) { v.table.Filter(filter) }

// GetJenkinsURL returns the Jenkins web UI URL for this view.
func (v *TestSuitesView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns the internal view path for bookmarking.
// Format: tests/<jobPath>/<buildNum>.
func (v *TestSuitesView) GetViewPath() string {
	return fmt.Sprintf("tests/%s/%d", v.jobName, v.buildNum)
}

// IDs returns the suite names; powers prompt argument completion.
func (v *TestSuitesView) IDs() []string { return v.table.GetRowIDs() }

func (v *TestSuitesView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Cases", v.enterCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
		ui.KeyShiftN:   ui.NewKeyAction("Sort Name", v.sortByNameCmd, true),
		ui.KeyShiftS:   ui.NewKeyAction("Sort Status", v.sortByStatusCmd, true),
		ui.KeyShiftA:   ui.NewKeyAction("Sort Duration", v.sortByDurationCmd, true),
	})
	v.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		if key == tcell.KeyRune {
			key = tcell.Key(evt.Rune())
		}
		if action, ok := v.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

func (v *TestSuitesView) refresh() {
	if v.app.Client() == nil {
		v.app.Flash().Err(fmt.Errorf("not connected to Jenkins"))
		return
	}
	go func() {
		report, err := v.app.Client().GetTestReport(context.Background(), v.jobName, v.buildNum)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
				return
			}
			if report == nil {
				v.app.Flash().Warn("This build has no JUnit test report")
				v.report = &client.TestReport{}
				v.renderSuites()
				return
			}
			v.report = report
			v.renderSuites()
		})
	}()
}

// renderSuites populates the table from v.report. Exposed for direct
// invocation in unit tests so the goroutine in refresh() can be skipped.
func (v *TestSuitesView) renderSuites() {
	v.table.SetHeaders([]string{"SUITE", "PASS", "FAIL", "SKIP", "DURATION"})

	if v.report == nil {
		v.table.SetData(nil)
		v.table.Refresh()
		return
	}

	rows := make([][]string, 0, len(v.report.Suites))
	for _, s := range v.report.Suites {
		var pass, fail, skip int
		for _, c := range s.Cases {
			switch c.Status {
			case "PASSED", "FIXED":
				pass++
			case "FAILED", "REGRESSION":
				fail++
			case "SKIPPED":
				skip++
			}
		}
		rows = append(rows, []string{
			s.Name,
			fmt.Sprintf("[#859900::b]%d[-::-]", pass),
			ifZeroDash(fail, fmt.Sprintf("[#dc322f::b]%d[-::-]", fail)),
			ifZeroDash(skip, fmt.Sprintf("[#b58900::b]%d[-::-]", skip)),
			formatDuration(time.Duration(s.Duration*float64(time.Second))),
		})
	}

	v.table.SetData(rows)
	v.table.SetTitle(fmt.Sprintf("Tests:%s#%d", v.jobName, v.buildNum))
	v.table.Refresh()
}

// ifZeroDash returns a single dash when the count is zero so 0-cells stay
// muted, otherwise the styled count text.
func ifZeroDash(n int, styled string) string {
	if n == 0 {
		return "-"
	}
	return styled
}

func (v *TestSuitesView) enterCmd(*tcell.EventKey) *tcell.EventKey {
	suiteName := v.table.GetSelectedID()
	if suiteName == "" || v.report == nil {
		return nil
	}
	for i := range v.report.Suites {
		if v.report.Suites[i].Name == suiteName {
			v.app.Content.Push(NewTestCasesView(v.app, v.jobName, v.buildNum, &v.report.Suites[i]))
			return nil
		}
	}
	return nil
}

func (v *TestSuitesView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}

func (v *TestSuitesView) sortByNameCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(0)
	v.table.Refresh()
	return nil
}

// sortByStatusCmd sorts by FAIL count column (the most useful "status"
// signal at suite level — failures bubble to the top).
func (v *TestSuitesView) sortByStatusCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(2)
	v.table.Refresh()
	return nil
}

func (v *TestSuitesView) sortByDurationCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(4)
	v.table.Refresh()
	return nil
}

// -----------------------------------------------------------------------
// TestCasesView — second level (suite → cases).
// -----------------------------------------------------------------------

// TestCasesView lists the cases of a single JUnit suite.
type TestCasesView struct {
	*tview.Flex
	app      *App
	table    *ui.Table
	actions  *ui.KeyActions
	jobName  string
	buildNum int
	suite    *client.TestSuite
}

// NewTestCasesView returns a new test-cases view for one suite.
func NewTestCasesView(app *App, jobName string, buildNum int, suite *client.TestSuite) *TestCasesView {
	v := &TestCasesView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		table:    ui.NewTable(),
		actions:  ui.NewKeyActions(),
		jobName:  jobName,
		buildNum: buildNum,
		suite:    suite,
	}
	v.AddItem(v.table, 0, 1, true)
	v.bindKeys()
	v.renderCases()
	return v
}

// Name returns the view name.
func (v *TestCasesView) Name() string {
	suite := ""
	if v.suite != nil {
		suite = v.suite.Name
	}
	return fmt.Sprintf("Cases[%s#%d:%s]", v.jobName, v.buildNum, suite)
}

// Hints returns the menu hints.
func (v *TestCasesView) Hints() model.MenuHints { return v.actions.Hints() }

// SetFilter forwards to the table.
func (v *TestCasesView) SetFilter(filter string) { v.table.Filter(filter) }

// GetJenkinsURL returns the Jenkins URL for the build's testReport
// overview. The per-suite URL is only navigable in Jenkins via the
// className-derived path which we expose at the case-detail level.
func (v *TestCasesView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns tests/<jobPath>/<buildNum>.
func (v *TestCasesView) GetViewPath() string {
	return fmt.Sprintf("tests/%s/%d", v.jobName, v.buildNum)
}

// IDs returns the case names; powers prompt argument completion.
func (v *TestCasesView) IDs() []string { return v.table.GetRowIDs() }

func (v *TestCasesView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Detail", v.enterCmd, true),
		ui.KeyShiftN:   ui.NewKeyAction("Sort Name", v.sortByNameCmd, true),
		ui.KeyShiftS:   ui.NewKeyAction("Sort Status", v.sortByStatusCmd, true),
		ui.KeyShiftA:   ui.NewKeyAction("Sort Duration", v.sortByDurationCmd, true),
	})
	v.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		if key == tcell.KeyRune {
			key = tcell.Key(evt.Rune())
		}
		if action, ok := v.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

// renderCases populates the table from v.suite.Cases.
func (v *TestCasesView) renderCases() {
	v.table.SetHeaders([]string{"NAME", "STATUS", "CLASS", "DURATION"})
	if v.suite == nil {
		v.table.SetData(nil)
		v.table.Refresh()
		return
	}
	rows := make([][]string, 0, len(v.suite.Cases))
	for _, c := range v.suite.Cases {
		rows = append(rows, []string{
			c.Name,
			colorizeTestStatus(c.Status),
			c.ClassName,
			formatDuration(time.Duration(c.Duration * float64(time.Second))),
		})
	}
	v.table.SetData(rows)
	title := fmt.Sprintf("Cases:%s", v.suite.Name)
	v.table.SetTitle(title)
	v.table.Refresh()
}

func (v *TestCasesView) enterCmd(*tcell.EventKey) *tcell.EventKey {
	caseName := v.table.GetSelectedID()
	if caseName == "" || v.suite == nil {
		return nil
	}
	for i := range v.suite.Cases {
		if v.suite.Cases[i].Name == caseName {
			v.app.Content.Push(NewTestCaseDetailView(v.app, v.jobName, v.buildNum, &v.suite.Cases[i]))
			return nil
		}
	}
	return nil
}

func (v *TestCasesView) sortByNameCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(0)
	v.table.Refresh()
	return nil
}

func (v *TestCasesView) sortByStatusCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(1)
	v.table.Refresh()
	return nil
}

func (v *TestCasesView) sortByDurationCmd(*tcell.EventKey) *tcell.EventKey {
	v.table.SortByColumn(3)
	v.table.Refresh()
	return nil
}

// -----------------------------------------------------------------------
// TestCaseDetailView — third level (case → status/error/stdout/stderr).
// -----------------------------------------------------------------------

// TestCaseDetailView shows the details of a single test case using the
// same framed FilterableTextView chrome as DescribeView.
type TestCaseDetailView struct {
	*tview.Flex
	app      *App
	textView *ui.FilterableTextView
	actions  *ui.KeyActions
	jobName  string
	buildNum int
	tcase    *client.TestCase
}

// NewTestCaseDetailView returns a new detail view for one test case.
func NewTestCaseDetailView(app *App, jobName string, buildNum int, tc *client.TestCase) *TestCaseDetailView {
	v := &TestCaseDetailView{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		app:      app,
		textView: ui.NewFilterableTextView(),
		actions:  ui.NewKeyActions(),
		jobName:  jobName,
		buildNum: buildNum,
		tcase:    tc,
	}

	// Match the framed-title chrome the rest of j9s uses.
	v.SetBorder(true)
	v.SetBorderColor(tcell.ColorAqua)
	v.SetTitleColor(tcell.ColorAqua)
	v.SetTitleAlign(tview.AlignLeft)
	v.textView.SetBorderPadding(0, 0, 1, 1)
	v.SetTitle(v.styledTitle())

	v.AddItem(v.textView, 0, 1, true)
	v.bindKeys()
	v.render()
	return v
}

func (v *TestCaseDetailView) styledTitle() string {
	name := ""
	if v.tcase != nil {
		name = v.tcase.Name
	}
	return fmt.Sprintf(" [aqua::b]TestCase[white::d](%s#%d)[aqua::b]:%s ", v.jobName, v.buildNum, name)
}

// Name returns the view name.
func (v *TestCaseDetailView) Name() string {
	name := ""
	if v.tcase != nil {
		name = v.tcase.Name
	}
	return fmt.Sprintf("TestCase[%s#%d:%s]", v.jobName, v.buildNum, name)
}

// Hints returns the menu hints.
func (v *TestCaseDetailView) Hints() model.MenuHints { return v.actions.Hints() }

// SetFilter forwards to the inner text view.
func (v *TestCaseDetailView) SetFilter(filter string) { v.textView.SetFilter(filter) }

// GetJenkinsURL returns the Jenkins URL for this specific test case.
// Build the URL using the className-derived package/class path that
// Jenkins exposes under /testReport/.
func (v *TestCaseDetailView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns tests/<jobPath>/<buildNum>/<package>/<class>/<test>.
// If the className doesn't contain a package qualifier, falls back to
// just the class + test (matching how Jenkins URLs render packageless
// tests).
func (v *TestCaseDetailView) GetViewPath() string {
	if v.tcase == nil {
		return fmt.Sprintf("tests/%s/%d", v.jobName, v.buildNum)
	}
	pkg, class := splitClassName(v.tcase.ClassName)
	switch {
	case pkg == "" && class == "":
		return fmt.Sprintf("tests/%s/%d", v.jobName, v.buildNum)
	case pkg == "":
		return fmt.Sprintf("tests/%s/%d/(root)/%s/%s", v.jobName, v.buildNum, class, v.tcase.Name)
	default:
		return fmt.Sprintf("tests/%s/%d/%s/%s/%s", v.jobName, v.buildNum, pkg, class, v.tcase.Name)
	}
}

// splitClassName splits a fully qualified class name into (package,
// class) pieces using the last dot. Returns ("", "") for empty input.
func splitClassName(fqcn string) (pkg, class string) {
	if fqcn == "" {
		return "", ""
	}
	idx := strings.LastIndex(fqcn, ".")
	if idx < 0 {
		return "", fqcn
	}
	return fqcn[:idx], fqcn[idx+1:]
}

func (v *TestCaseDetailView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)
	v.actions.Bulk(ui.KeyMap{
		ui.KeyG:      ui.NewKeyAction("Top", v.topCmd, true),
		ui.KeyShiftG: ui.NewKeyAction("Bottom", v.bottomCmd, true),
		tcell.KeyEsc: ui.NewKeyAction("Back", v.backCmd, true),
	})
	v.textView.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		if key == tcell.KeyRune {
			key = tcell.Key(evt.Rune())
		}
		if action, ok := v.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

// render formats the test-case detail. Keeps a clear section structure
// so /-filter highlighting (FilterableTextView's built-in) lands on the
// expected substrings.
func (v *TestCaseDetailView) render() {
	if v.tcase == nil {
		v.textView.SetContentWithColors("[gray]no test case[white]")
		return
	}
	c := v.tcase

	var sb strings.Builder
	sb.WriteString("[yellow::b]Basic Information[white::-]\n")
	sb.WriteString(fmt.Sprintf("  [aqua::b]Name:[-::-]      %s\n", c.Name))
	sb.WriteString(fmt.Sprintf("  [aqua::b]Class:[-::-]     %s\n", c.ClassName))
	sb.WriteString(fmt.Sprintf("  [aqua::b]Status:[-::-]    %s\n", colorizeTestStatus(c.Status)))
	sb.WriteString(fmt.Sprintf("  [aqua::b]Duration:[-::-]  %s\n", formatDuration(time.Duration(c.Duration*float64(time.Second)))))
	if c.SkippedMessage != "" {
		sb.WriteString(fmt.Sprintf("  [aqua::b]Skipped:[-::-]   %s\n", c.SkippedMessage))
	}
	sb.WriteString("\n")

	if c.ErrorDetails != "" {
		sb.WriteString("[yellow::b]Error[white::-]\n")
		sb.WriteString(c.ErrorDetails)
		sb.WriteString("\n\n")
	}
	if c.ErrorStackTrace != "" {
		sb.WriteString("[yellow::b]Stack Trace[white::-]\n")
		sb.WriteString(c.ErrorStackTrace)
		sb.WriteString("\n\n")
	}
	if c.Stdout != "" {
		sb.WriteString("[yellow::b]Stdout[white::-]\n")
		sb.WriteString(c.Stdout)
		sb.WriteString("\n\n")
	}
	if c.Stderr != "" {
		sb.WriteString("[yellow::b]Stderr[white::-]\n")
		sb.WriteString(c.Stderr)
		sb.WriteString("\n\n")
	}

	v.textView.SetContentWithColors(sb.String())
}

func (v *TestCaseDetailView) topCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToTop()
	return nil
}

func (v *TestCaseDetailView) bottomCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToBottom()
	return nil
}

func (v *TestCaseDetailView) backCmd(*tcell.EventKey) *tcell.EventKey {
	if v.app.Content.CanPop() {
		v.app.Content.Pop()
	}
	return nil
}
