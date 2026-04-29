// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// HTMLReportsView lists HTML Publisher reports attached to a build.
// j9s does not render HTML in the TUI — Enter/o launches the report in
// the system browser via openURLFunc, and the global <u> hotkey copies
// the report URL to the clipboard.
//
// Picks up reports surfaced by any plugin whose action carries both
// urlName + reportName (HtmlPublisherTarget, pytest-html, allure
// publishers that piggy-back on htmlpublisher).
type HTMLReportsView struct {
	*tview.Flex
	app      *App
	table    *ui.Table
	actions  *ui.KeyActions
	jobName  string
	buildNum int
	reports  []client.HTMLReport
}

// NewHTMLReportsView returns a new HTML reports view for a build.
func NewHTMLReportsView(app *App, jobName string, buildNum int) *HTMLReportsView {
	v := &HTMLReportsView{
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

// Name returns the view name.
func (v *HTMLReportsView) Name() string {
	return fmt.Sprintf("Reports[%s#%d]", v.jobName, v.buildNum)
}

// Hints returns the menu hints.
func (v *HTMLReportsView) Hints() model.MenuHints { return v.actions.Hints() }

// SetFilter forwards the filter to the table.
func (v *HTMLReportsView) SetFilter(filter string) { v.table.Filter(filter) }

// IDs returns the urlName values; powers prompt argument completion.
func (v *HTMLReportsView) IDs() []string { return v.table.GetRowIDs() }

// GetJenkinsURL returns the Jenkins URL for the currently selected
// report, falling back to the build's root URL when nothing is selected
// (matches "the URL the user is currently looking at").
func (v *HTMLReportsView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, v.GetViewPath())
}

// GetViewPath returns reports/<jobPath>/<buildNum>[/<urlName>] depending
// on whether a row is selected.
func (v *HTMLReportsView) GetViewPath() string {
	if id := v.table.GetSelectedID(); id != "" {
		return fmt.Sprintf("reports/%s/%d/%s", v.jobName, v.buildNum, id)
	}
	return fmt.Sprintf("reports/%s/%d", v.jobName, v.buildNum)
}

func (v *HTMLReportsView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Open", v.openCmd, true),
		ui.KeyO:        ui.NewKeyAction("Open", v.openCmd, true),
		ui.KeyR:        ui.NewKeyAction("Refresh", v.refreshCmd, true),
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

func (v *HTMLReportsView) refresh() {
	if v.app.Client() == nil {
		v.app.Flash().Err(fmt.Errorf("not connected to Jenkins"))
		return
	}
	go func() {
		reports, err := v.app.Client().GetHTMLReports(context.Background(), v.jobName, v.buildNum)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
				return
			}
			v.reports = reports
			v.renderReports()
		})
	}()
}

// renderReports populates the table from v.reports. Exposed for unit
// tests so callers can skip the goroutine in refresh().
func (v *HTMLReportsView) renderReports() {
	v.table.SetHeaders([]string{"URL", "REPORT"})
	rows := make([][]string, 0, len(v.reports))
	for _, r := range v.reports {
		rows = append(rows, []string{r.URLName, r.ReportName})
	}
	v.table.SetData(rows)
	v.table.SetTitle(fmt.Sprintf("Reports:%s#%d", v.jobName, v.buildNum))
	v.table.Refresh()
}

func (v *HTMLReportsView) openCmd(*tcell.EventKey) *tcell.EventKey {
	url := v.GetJenkinsURL()
	if url == "" {
		v.app.Flash().Warn("No URL for this report")
		return nil
	}
	if err := openURLFunc(url); err != nil {
		v.app.Flash().Err(err)
		return nil
	}
	v.app.Flash().Info("Opening " + url)
	return nil
}

func (v *HTMLReportsView) refreshCmd(*tcell.EventKey) *tcell.EventKey {
	v.refresh()
	return nil
}
