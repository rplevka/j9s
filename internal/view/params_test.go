// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"testing"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/rplevka/j9s/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleParams() []client.ParameterDef {
	return []client.ParameterDef{
		{Name: "BRANCH", Class: "hudson.model.StringParameterDefinition", DefaultParameterValue: &client.ParamValue{Value: "main"}},
		{Name: "ENV", Class: "hudson.model.ChoiceParameterDefinition", Choices: []string{"dev", "prod"}, DefaultParameterValue: &client.ParamValue{Value: "dev"}},
		{Name: "DRY_RUN", Class: "hudson.model.BooleanParameterDefinition", DefaultParameterValue: &client.ParamValue{Value: "false"}},
	}
}

// TestParamsView_DefaultsFocusToBuildButton verifies the dialog opens with
// the Build button pre-focused so a user can submit a rebuild via Enter.
func TestParamsView_DefaultsFocusToBuildButton(t *testing.T) {
	v := NewParamsView(nil, "hello", sampleParams(), nil, func(map[string]string) {}, func() {})

	var got tview.Primitive
	v.form.Focus(func(p tview.Primitive) { got = p })

	buildIdx := v.form.GetButtonIndex("Build")
	require.GreaterOrEqual(t, buildIdx, 0, "Build button must exist")
	want := v.form.GetButton(buildIdx)

	assert.Same(t, want, got, "form should delegate focus to the Build button by default")
}

// TestParamsView_PrePopulatesParameterDefaults asserts that when no
// lastValues are provided (the fresh trigger flow), each field still picks
// up the value from ParameterDef.DefaultParameterValue: the string field
// gets its default, the dropdown lands on the default choice, and the
// boolean checkbox is checked when the default is true. This regression
// test guards against the GetJob tree= query forgetting to expand
// defaultParameterValue.
func TestParamsView_PrePopulatesParameterDefaults(t *testing.T) {
	params := []client.ParameterDef{
		{Name: "BRANCH", Class: "hudson.model.StringParameterDefinition", DefaultParameterValue: &client.ParamValue{Value: "main"}},
		{Name: "ENV", Class: "hudson.model.ChoiceParameterDefinition", Choices: []string{"dev", "prod"}, DefaultParameterValue: &client.ParamValue{Value: "prod"}},
		{Name: "DRY_RUN", Class: "hudson.model.BooleanParameterDefinition", DefaultParameterValue: &client.ParamValue{Value: true}},
	}
	v := NewParamsView(nil, "hello", params, nil, func(map[string]string) {}, func() {})

	branch, ok := v.form.GetFormItem(0).(*tview.InputField)
	require.True(t, ok)
	assert.Equal(t, "main", branch.GetText())

	envDD, ok := v.form.GetFormItem(1).(*tview.DropDown)
	require.True(t, ok)
	idx, opt := envDD.GetCurrentOption()
	assert.Equal(t, 1, idx)
	assert.Equal(t, "prod", opt)

	dryRun, ok := v.form.GetFormItem(2).(*tview.Checkbox)
	require.True(t, ok)
	assert.True(t, dryRun.IsChecked(), "boolean default=true should pre-check the checkbox")
}

// TestParamsView_PrePopulatesLastValues asserts that when lastValues
// override the parameter defaults (the rebuild flow), the form's text
// fields surface those values.
func TestParamsView_PrePopulatesLastValues(t *testing.T) {
	last := map[string]string{"BRANCH": "feature/x"}
	v := NewParamsView(nil, "hello", sampleParams(), last, func(map[string]string) {}, func() {})

	branch, ok := v.form.GetFormItem(0).(*tview.InputField)
	require.True(t, ok, "BRANCH field should be an InputField")
	assert.Equal(t, "feature/x", branch.GetText())
}

// TestParamsView_SubmitCallbackCollectsParams drives ParamsView.submit() and
// asserts the onSubmit callback receives the correctly decoded params for
// each field type (string, choice, bool).
func TestParamsView_SubmitCallbackCollectsParams(t *testing.T) {
	var got map[string]string
	v := NewParamsView(nil, "hello", sampleParams(), nil, func(p map[string]string) { got = p }, func() {})

	// Mutate the input field
	branch := v.form.GetFormItem(0).(*tview.InputField)
	branch.SetText("release/1.2")

	// Switch the dropdown to "prod"
	envDD := v.form.GetFormItem(1).(*tview.DropDown)
	envDD.SetCurrentOption(1)

	// Toggle the checkbox on
	dryRun := v.form.GetFormItem(2).(*tview.Checkbox)
	dryRun.SetChecked(true)

	v.submit()

	require.NotNil(t, got)
	assert.Equal(t, "release/1.2", got["BRANCH"])
	assert.Equal(t, "prod", got["ENV"])
	assert.Equal(t, "true", got["DRY_RUN"])
}

// TestParamsView_EscCancels asserts Esc invokes the cancel callback via
// the form's input capture.
func TestParamsView_EscCancels(t *testing.T) {
	cancelled := false
	v := NewParamsView(nil, "hello", sampleParams(), nil, func(map[string]string) {}, func() { cancelled = true })

	handler := v.form.InputHandler()
	require.NotNil(t, handler)
	handler(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	assert.True(t, cancelled, "Esc should trigger cancel callback")
}

// TestParamsView_CtrlSSubmits asserts Ctrl-S invokes the submit callback.
func TestParamsView_CtrlSSubmits(t *testing.T) {
	submitted := false
	v := NewParamsView(nil, "hello", sampleParams(), nil, func(map[string]string) { submitted = true }, func() {})

	handler := v.form.InputHandler()
	require.NotNil(t, handler)
	handler(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl), func(tview.Primitive) {})

	assert.True(t, submitted, "Ctrl-S should trigger submit callback")
}
