// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// DescribeView displays resource details/configuration.
type DescribeView struct {
	*tview.Flex
	app          *App
	textView     *ui.FilterableTextView
	actions      *ui.KeyActions
	resourceType string
	resourceName string
}

// NewDescribeView returns a new describe view.
func NewDescribeView(app *App, resourceType, resourceName string) *DescribeView {
	v := &DescribeView{
		Flex:         tview.NewFlex().SetDirection(tview.FlexRow),
		app:          app,
		textView:     ui.NewFilterableTextView(),
		actions:      ui.NewKeyActions(),
		resourceType: resourceType,
		resourceName: resourceName,
	}

	// Match the framing used by table-based views (Jobs/Builds) and
	// LogsView: aqua border + left-aligned styled title on the Flex,
	// inner padding on the textview so content is not flush with the
	// border. Without this, the describe view rendered as raw text with
	// no chrome, making it visually disconnected from the rest of j9s.
	v.SetBorder(true)
	v.SetBorderColor(tcell.ColorAqua)
	v.SetTitleColor(tcell.ColorAqua)
	v.SetTitleAlign(tview.AlignLeft)
	v.textView.SetBorderPadding(0, 0, 1, 1)
	v.SetTitle(v.styledTitle())

	v.AddItem(v.textView, 0, 1, true)
	v.bindKeys()
	v.refresh()

	return v
}

// styledTitle formats the describe-view title with the same accent
// styling as ui.Table — " Describe(<type>):<resource> ".
func (v *DescribeView) styledTitle() string {
	return fmt.Sprintf(
		" [aqua::b]Describe[white::d](%s)[aqua::b]:%s ",
		v.resourceType, v.resourceName,
	)
}

// Name returns the view name.
func (v *DescribeView) Name() string {
	return fmt.Sprintf("Describe[%s/%s]", v.resourceType, v.resourceName)
}

// Hints returns the view hints.
func (v *DescribeView) Hints() model.MenuHints {
	return v.actions.Hints()
}

// GetJenkinsURL returns the Jenkins web UI URL for the resource being
// described. Implements URLProvider so the global "u" hotkey can copy
// the link from a job- or build-describe view.
func (v *DescribeView) GetJenkinsURL() string {
	ctx, _ := v.app.Config().ActiveContext()
	if ctx == nil {
		return ""
	}
	path := v.GetViewPath()
	if path == "" {
		return ""
	}
	return GenerateJenkinsURL(ctx.URL, path)
}

// GetViewPath returns the internal view path for bookmarking. For a job
// describe this is "jobs/<jobPath>"; for a build describe it is
// "builds/<jobPath>#<buildNum>" — matching the formats produced by
// JobsView / BuildsView so URL generation is consistent.
func (v *DescribeView) GetViewPath() string {
	switch v.resourceType {
	case "job":
		if v.resourceName == "" {
			return ""
		}
		return "jobs/" + v.resourceName
	case "build":
		// resourceName is "<jobPath>#<buildNum>" (set by buildsCmd).
		if v.resourceName == "" || !strings.Contains(v.resourceName, "#") {
			return ""
		}
		return "builds/" + v.resourceName
	}
	return ""
}

func (v *DescribeView) bindKeys() {
	AddGlobalKeys(v.app, v.actions)

	v.actions.Bulk(ui.KeyMap{
		ui.KeyG:      ui.NewKeyAction("Top", v.topCmd, true),
		ui.KeyShiftG: ui.NewKeyAction("Bottom", v.bottomCmd, true),
		ui.KeyN:      ui.NewKeyAction("Next Match", v.nextMatchCmd, true),
		ui.KeyShiftN: ui.NewKeyAction("Prev Match", v.prevMatchCmd, true),
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

// SetFilter sets the search filter and highlights matches.
func (v *DescribeView) SetFilter(filter string) {
	v.textView.SetFilter(filter)
}

func (v *DescribeView) nextMatchCmd(*tcell.EventKey) *tcell.EventKey {
	// Scroll down
	row, _ := v.textView.GetScrollOffset()
	v.textView.ScrollTo(row+5, 0)
	return nil
}

func (v *DescribeView) prevMatchCmd(*tcell.EventKey) *tcell.EventKey {
	// Scroll up
	row, _ := v.textView.GetScrollOffset()
	if row > 5 {
		v.textView.ScrollTo(row-5, 0)
	} else {
		v.textView.ScrollTo(0, 0)
	}
	return nil
}

func (v *DescribeView) refresh() {
	if v.app.Client() == nil {
		return
	}

	go func() {
		var content string
		var err error

		switch v.resourceType {
		case "job":
			content, err = v.app.Client().GetJobConfig(context.Background(), v.resourceName)
		case "build":
			// Parse "jobName#buildNum" format
			parts := strings.Split(v.resourceName, "#")
			if len(parts) == 2 {
				buildNum, _ := strconv.Atoi(parts[1])
				build, e := v.app.Client().GetBuild(context.Background(), parts[0], buildNum)
				if e == nil {
					content = v.formatBuildDetails(build)
				}
				err = e
			}
		default:
			content = fmt.Sprintf("Unknown resource type: %s", v.resourceType)
		}

		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.app.Flash().Err(err)
				v.textView.SetContent(fmt.Sprintf("Error: %v", err))
			} else {
				// Use SetContentWithColors for build details which have color tags
				if v.resourceType == "build" {
					v.textView.SetContentWithColors(content)
				} else {
					v.textView.SetContent(content)
				}
			}
		})
	}()
}

func (v *DescribeView) formatBuildDetails(build interface{}) string {
	b, ok := build.(*client.Build)
	if !ok {
		return fmt.Sprintf("%+v", build)
	}

	var sb strings.Builder

	// (No "Build #N" header — the framed title at the top of the view
	// already shows " Describe(build):<jobPath>#<num> ".)

	// Basic Info
	sb.WriteString("[yellow::b]Basic Information[white::-]\n")
	sb.WriteString(fmt.Sprintf("  [aqua::b]Name:[-::-]          %s\n", b.FullDisplayName))
	sb.WriteString(fmt.Sprintf("  [aqua::b]Result:[-::-]        %s\n", colorizeResult(b.Result)))
	sb.WriteString(fmt.Sprintf("  [aqua::b]Building:[-::-]      %v\n", b.Building))
	sb.WriteString(fmt.Sprintf("  [aqua::b]Duration:[-::-]      %s\n", formatDuration(time.Duration(b.Duration)*time.Millisecond)))
	sb.WriteString(fmt.Sprintf("  [aqua::b]Started:[-::-]       %s\n", formatBuildTimestamp(b.Timestamp)))
	if b.Description != "" {
		sb.WriteString(fmt.Sprintf("  [aqua::b]Description:[-::-]   %s\n", b.Description))
	}
	sb.WriteString(fmt.Sprintf("  [aqua::b]URL:[-::-]           %s\n", b.URL))
	sb.WriteString("\n")

	// Build Causes
	for _, action := range b.Actions {
		if len(action.Causes) > 0 {
			sb.WriteString("[yellow::b]Build Causes[white::-]\n")
			for _, cause := range action.Causes {
				sb.WriteString(fmt.Sprintf("  • %s", cause.ShortDescription))
				if cause.UserName != "" {
					sb.WriteString(fmt.Sprintf(" (by [green::]%s[-::])", cause.UserName))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
			break
		}
	}

	// Build Parameters
	for _, action := range b.Actions {
		if len(action.Parameters) > 0 {
			sb.WriteString("[yellow::b]Parameters[white::-]\n")
			for _, param := range action.Parameters {
				value := fmt.Sprintf("%v", param.Value)
				if len(value) > 80 {
					value = value[:77] + "..."
				}
				sb.WriteString(fmt.Sprintf("  [aqua::]%s:[-::] %s\n", param.Name, value))
			}
			sb.WriteString("\n")
			break
		}
	}

	// Artifacts
	if len(b.Artifacts) > 0 {
		sb.WriteString("[yellow::b]Artifacts[white::-]\n")
		for _, artifact := range b.Artifacts {
			sb.WriteString(fmt.Sprintf("  📦 %s\n", artifact.FileName))
			sb.WriteString(fmt.Sprintf("     [gray::]%s[-::]\n", artifact.RelativePath))
		}
		sb.WriteString("\n")
	}

	// Change Sets
	for _, cs := range b.ChangeSets {
		if len(cs.Items) > 0 {
			sb.WriteString(fmt.Sprintf("[yellow::b]Changes (%s)[white::-]\n", cs.Kind))
			for i, item := range cs.Items {
				if i >= 10 {
					sb.WriteString(fmt.Sprintf("  ... and %d more changes\n", len(cs.Items)-10))
					break
				}
				// Truncate commit ID
				commitID := item.CommitID
				if len(commitID) > 8 {
					commitID = commitID[:8]
				}
				// Truncate comment to first line
				comment := strings.Split(item.Comment, "\n")[0]
				if len(comment) > 60 {
					comment = comment[:57] + "..."
				}
				sb.WriteString(fmt.Sprintf("  [green::]%s[-::] %s\n", commitID, comment))
				sb.WriteString(fmt.Sprintf("           [gray::]by %s[-::]\n", item.Author.FullName))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatBuildTimestamp(ts int64) string {
	if ts == 0 {
		return "-"
	}
	t := time.UnixMilli(ts)
	return t.Format("2006-01-02 15:04:05")
}

func (v *DescribeView) topCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToTop()
	return nil
}

func (v *DescribeView) bottomCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToBottom()
	return nil
}

func (v *DescribeView) backCmd(*tcell.EventKey) *tcell.EventKey {
	if v.app.Content.CanPop() {
		v.app.Content.Pop()
	}
	return nil
}
