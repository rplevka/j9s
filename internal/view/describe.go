// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// DescribeView displays resource details/configuration.
type DescribeView struct {
	*tview.Flex
	app          *App
	textView     *tview.TextView
	actions      *ui.KeyActions
	resourceType string
	resourceName string
}

// NewDescribeView returns a new describe view.
func NewDescribeView(app *App, resourceType, resourceName string) *DescribeView {
	v := &DescribeView{
		Flex:         tview.NewFlex().SetDirection(tview.FlexRow),
		app:          app,
		textView:     tview.NewTextView(),
		actions:      ui.NewKeyActions(),
		resourceType: resourceType,
		resourceName: resourceName,
	}

	v.textView.SetDynamicColors(true)
	v.textView.SetScrollable(true)
	v.textView.SetBackgroundColor(tcell.ColorDefault)
	v.textView.SetWrap(true)

	v.AddItem(v.textView, 0, 1, true)
	v.bindKeys()
	v.refresh()

	return v
}

// Name returns the view name.
func (v *DescribeView) Name() string {
	return fmt.Sprintf("Describe[%s/%s]", v.resourceType, v.resourceName)
}

// Hints returns the view hints.
func (v *DescribeView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *DescribeView) bindKeys() {
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
				v.textView.SetText(fmt.Sprintf("[red]Error: %v[-]", err))
			} else {
				v.textView.SetText(v.syntaxHighlight(content))
			}
		})
	}()
}

func (v *DescribeView) formatBuildDetails(build interface{}) string {
	// Format build details as YAML-like output
	return fmt.Sprintf("%+v", build)
}

func (v *DescribeView) syntaxHighlight(content string) string {
	// Basic XML syntax highlighting
	content = strings.ReplaceAll(content, "<", "[aqua]<[-]")
	content = strings.ReplaceAll(content, ">", "[aqua]>[-]")
	return content
}

func (v *DescribeView) topCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToBeginning()
	return nil
}

func (v *DescribeView) bottomCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToEnd()
	return nil
}

func (v *DescribeView) backCmd(*tcell.EventKey) *tcell.EventKey {
	if v.app.Content.CanPop() {
		v.app.Content.Pop()
	}
	return nil
}
