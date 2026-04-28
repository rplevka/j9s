// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

// ParamsView displays a form for build parameters.
type ParamsView struct {
	*tview.Flex
	app        *App
	form       *tview.Form
	actions    *ui.KeyActions
	jobName    string
	params     []client.ParameterDef
	lastValues map[string]string
	onSubmit   func(params map[string]string)
	onCancel   func()
}

// NewParamsView returns a new parameters form view.
func NewParamsView(app *App, jobName string, params []client.ParameterDef, lastValues map[string]string, onSubmit func(map[string]string), onCancel func()) *ParamsView {
	v := &ParamsView{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		app:        app,
		form:       tview.NewForm(),
		actions:    ui.NewKeyActions(),
		jobName:    jobName,
		params:     params,
		lastValues: lastValues,
		onSubmit:   onSubmit,
		onCancel:   onCancel,
	}

	v.setupForm()
	v.bindKeys()

	// Add title and form
	title := tview.NewTextView().
		SetText(fmt.Sprintf(" Build Parameters for [yellow]%s[-]", jobName)).
		SetDynamicColors(true)
	title.SetBackgroundColor(tcell.ColorDefault)

	v.AddItem(title, 1, 0, false)
	v.AddItem(v.form, 0, 1, true)

	return v
}

// Name returns the view name.
func (v *ParamsView) Name() string {
	return fmt.Sprintf("Params[%s]", v.jobName)
}

// Hints returns the view hints.
func (v *ParamsView) Hints() model.MenuHints {
	return model.MenuHints{
		{Mnemonic: "Enter", Description: "Submit", Visible: true},
		{Mnemonic: "Esc", Description: "Cancel", Visible: true},
		{Mnemonic: "Tab", Description: "Next Field", Visible: true},
	}
}

func (v *ParamsView) setupForm() {
	v.form.SetBackgroundColor(tcell.ColorDefault)
	v.form.SetFieldBackgroundColor(tcell.ColorDarkSlateGray)
	v.form.SetFieldTextColor(tcell.ColorWhite)
	v.form.SetLabelColor(tcell.ColorAqua)
	v.form.SetButtonBackgroundColor(tcell.ColorDarkCyan)
	v.form.SetButtonTextColor(tcell.ColorWhite)

	for _, param := range v.params {
		// Get default/last value
		defaultVal := ""
		if param.DefaultParameterValue != nil && param.DefaultParameterValue.Value != nil {
			defaultVal = fmt.Sprintf("%v", param.DefaultParameterValue.Value)
		}
		if lastVal, ok := v.lastValues[param.Name]; ok {
			defaultVal = lastVal
		}

		// Create appropriate field based on parameter type
		switch {
		case len(param.Choices) > 0:
			// Choice parameter - dropdown
			initialIdx := 0
			for i, choice := range param.Choices {
				if choice == defaultVal {
					initialIdx = i
					break
				}
			}
			v.form.AddDropDown(v.formatLabel(param), param.Choices, initialIdx, nil)

		case strings.Contains(strings.ToLower(param.Class), "boolean"):
			// Boolean parameter - checkbox
			checked := strings.ToLower(defaultVal) == "true"
			v.form.AddCheckbox(v.formatLabel(param), checked, nil)

		case strings.Contains(strings.ToLower(param.Class), "password"):
			// Password parameter
			v.form.AddPasswordField(v.formatLabel(param), defaultVal, 40, '*', nil)

		default:
			// String parameter - text input
			v.form.AddInputField(v.formatLabel(param), defaultVal, 40, nil, nil)
		}
	}

	// Add buttons
	v.form.AddButton("Build", v.submit)
	v.form.AddButton("Cancel", v.cancel)

	// Focus the Build button by default so users can quickly rebuild with
	// pre-filled values by hitting Enter. tview.Form.SetFocus indexes form
	// items first, then buttons, so len(items) == first button == "Build".
	v.form.SetFocus(len(v.params))
}

func (v *ParamsView) formatLabel(param client.ParameterDef) string {
	label := param.Name
	if param.Description != "" {
		// Truncate long descriptions
		desc := param.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		label = fmt.Sprintf("%s (%s)", param.Name, desc)
	}
	return label
}

func (v *ParamsView) bindKeys() {
	v.form.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		switch evt.Key() {
		case tcell.KeyEscape:
			v.cancel()
			return nil
		case tcell.KeyCtrlS:
			v.submit()
			return nil
		}
		return evt
	})
}

func (v *ParamsView) submit() {
	params := v.collectParams()
	if v.onSubmit != nil {
		v.onSubmit(params)
	}
}

func (v *ParamsView) cancel() {
	if v.onCancel != nil {
		v.onCancel()
	}
}

func (v *ParamsView) collectParams() map[string]string {
	params := make(map[string]string)

	for i, param := range v.params {
		item := v.form.GetFormItem(i)
		if item == nil {
			continue
		}

		switch field := item.(type) {
		case *tview.DropDown:
			_, val := field.GetCurrentOption()
			params[param.Name] = val
		case *tview.Checkbox:
			if field.IsChecked() {
				params[param.Name] = "true"
			} else {
				params[param.Name] = "false"
			}
		case *tview.InputField:
			params[param.Name] = field.GetText()
		}
	}

	return params
}

// ShowParamsForm shows a parameter form for triggering a build.
// If the job has no parameters, it triggers the build directly.
func ShowParamsForm(app *App, jobName string, isRebuild bool) {
	go func() {
		// Fetch parameter definitions
		paramDefs, err := app.Client().GetJobParameters(context.Background(), jobName)
		if err != nil {
			app.QueueUpdateDraw(func() {
				app.Flash().Err(err)
			})
			return
		}

		// If no parameters, trigger build directly
		if len(paramDefs) == 0 {
			err := app.Client().TriggerBuild(context.Background(), jobName, nil)
			app.QueueUpdateDraw(func() {
				if err != nil {
					app.Flash().Err(err)
				} else {
					app.Flash().Info(fmt.Sprintf("Build triggered for %s", jobName))
				}
			})
			return
		}

		// Get last build parameters for pre-filling (especially for rebuild)
		var lastParams map[string]string
		if isRebuild {
			lastParams, _ = app.Client().GetLastBuildParameters(context.Background(), jobName)
		}
		if lastParams == nil {
			lastParams = make(map[string]string)
		}

		app.QueueUpdateDraw(func() {
			paramsView := NewParamsView(
				app,
				jobName,
				paramDefs,
				lastParams,
				func(params map[string]string) {
					// Submit callback
					app.Content.Pop()
					go func() {
						err := app.Client().TriggerBuild(context.Background(), jobName, params)
						app.QueueUpdateDraw(func() {
							if err != nil {
								app.Flash().Err(err)
							} else {
								app.Flash().Info(fmt.Sprintf("Build triggered for %s with %d parameters", jobName, len(params)))
							}
							// Refresh current view
							if top := app.Content.Top(); top != nil {
								if refreshable, ok := top.(interface{ Refresh() }); ok {
									refreshable.Refresh()
								}
							}
						})
					}()
				},
				func() {
					// Cancel callback
					app.Content.Pop()
				},
			)
			app.Content.Push(paramsView)
		})
	}()
}
