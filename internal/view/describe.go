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
				v.textView.SetContent(content)
			}
		})
	}()
}

func (v *DescribeView) formatBuildDetails(build interface{}) string {
	// Format build details as YAML-like output
	return fmt.Sprintf("%+v", build)
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
