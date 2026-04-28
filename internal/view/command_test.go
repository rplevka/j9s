// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"reflect"
	"testing"

	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/config"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIDView is a minimal tview.Primitive + model.Component + IDProvider
// stub used to feed argument-suggestion sources from a controlled set.
type fakeIDView struct {
	*tview.Box
	name string
	ids  []string
}

func newFakeIDView(name string, ids []string) *fakeIDView {
	return &fakeIDView{Box: tview.NewBox(), name: name, ids: ids}
}

func (f *fakeIDView) Name() string            { return f.name }
func (f *fakeIDView) Hints() model.MenuHints  { return nil }
func (f *fakeIDView) IDs() []string           { return f.ids }

func newTestCommand(t *testing.T, contexts []string) (*Command, *App) {
	t.Helper()
	cfg := config.NewConfig()
	cfg.J9s.Contexts = make([]config.Context, 0, len(contexts))
	for _, name := range contexts {
		cfg.J9s.Contexts = append(cfg.J9s.Contexts, config.Context{Name: name, URL: "http://" + name})
	}
	if len(contexts) > 0 {
		cfg.J9s.CurrentContext = contexts[0]
	}
	app := NewApp(cfg)
	return NewCommand(app), app
}

func TestCommand_SuggestFirstToken(t *testing.T) {
	c, _ := newTestCommand(t, nil)

	got := c.Suggest("ct")
	assert.Contains(t, got, "ctx")

	got = c.Suggest("jo")
	assert.Contains(t, got, "jobs")
	assert.Contains(t, got, "job")
}

func TestCommand_SuggestCtxArguments(t *testing.T) {
	c, _ := newTestCommand(t, []string{"prod", "preprod", "dev"})

	got := c.Suggest("ctx pr")
	assert.Equal(t, []string{"ctx preprod", "ctx prod"}, got, "should match contexts starting with 'pr', sorted")

	got = c.Suggest("ctx ")
	// All three contexts surfaced (sorted).
	assert.Equal(t, []string{"ctx dev", "ctx preprod", "ctx prod"}, got)

	got = c.Suggest("ctx zzz")
	assert.Empty(t, got)
}

func TestCommand_SuggestUsesCurrentViewIDs(t *testing.T) {
	c, app := newTestCommand(t, nil)
	app.Content.Push(newFakeIDView("Jobs", []string{"alpha", "beta", "gamma"}))

	got := c.Suggest("builds a")
	assert.Equal(t, []string{"builds alpha"}, got)

	got = c.Suggest("jobs ")
	assert.Equal(t, []string{"jobs alpha", "jobs beta", "jobs gamma"}, got)

	got = c.Suggest("logs g")
	assert.Equal(t, []string{"logs gamma"}, got)
}

func TestCommand_SuggestUnknownArgCommandReturnsNil(t *testing.T) {
	c, _ := newTestCommand(t, []string{"prod"})

	// `cache cl` is multi-token but cache isn't an argument-taking command.
	got := c.Suggest("cache cl")
	assert.Nil(t, got)

	// `bookmark something` likewise.
	got = c.Suggest("bookmark foo")
	assert.Nil(t, got)
}

func TestCommand_SuggestEmptyInput(t *testing.T) {
	c, _ := newTestCommand(t, []string{"prod"})
	assert.Nil(t, c.Suggest(""))
}

func TestCommand_MatchCtxArg(t *testing.T) {
	c, _ := newTestCommand(t, nil)

	cases := []struct {
		input  string
		want   string
		wantOk bool
	}{
		{"ctx prod", "prod", true},
		{"context prod", "prod", true},
		{"contexts prod", "prod", true},
		{"ctx", "", false},
		{"ctx prod extra", "", false},
		{"jobs prod", "", false},
	}
	for _, tc := range cases {
		got, ok := c.matchCtxArg(tc.input)
		if got != tc.want || ok != tc.wantOk {
			t.Errorf("matchCtxArg(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.want, tc.wantOk)
		}
	}
}

func TestFormatArgumentSuggestions(t *testing.T) {
	values := []string{"prod", "preprod", "dev", ""}

	got := formatArgumentSuggestions("ctx ", "pr", values)
	want := []string{"ctx preprod", "ctx prod"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Empty arg — surface every non-empty value.
	got = formatArgumentSuggestions("ctx ", "", values)
	require.Len(t, got, 3)
	assert.Contains(t, got, "ctx prod")
}
