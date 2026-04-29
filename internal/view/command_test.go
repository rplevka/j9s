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

func (f *fakeIDView) Name() string           { return f.name }
func (f *fakeIDView) Hints() model.MenuHints { return nil }
func (f *fakeIDView) IDs() []string          { return f.ids }

// fakePathView additionally implements PathProvider so tests can simulate
// a nested-folder JobsView whose suggestions must be path-qualified.
type fakePathView struct {
	*fakeIDView
	path string
}

func newFakePathView(name, path string, ids []string) *fakePathView {
	return &fakePathView{fakeIDView: newFakeIDView(name, ids), path: path}
}

func (f *fakePathView) CurrentPath() string { return f.path }

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
	// Initialize the embedded ui.App so flash/menu/crumbs/prompt are
	// non-nil; many code paths (refresh failures, etc.) call app.Flash().
	app.App.Init()
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

	got := formatArgumentSuggestions("ctx ", "", "pr", values)
	want := []string{"ctx preprod", "ctx prod"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Empty arg — surface every non-empty value.
	got = formatArgumentSuggestions("ctx ", "", "", values)
	require.Len(t, got, 3)
	assert.Contains(t, got, "ctx prod")

	// With a non-empty valuePrefix the prefix is woven into both the
	// emitted suggestion AND the prefix-match check against argPrefix.
	got = formatArgumentSuggestions("builds ", "team-a/sub/", "team-a/sub/d", []string{"deploy", "nightly"})
	assert.Equal(t, []string{"builds team-a/sub/deploy"}, got)
}

// TestCommand_SuggestPrefixesFolderPath asserts that when the top view is
// a nested-folder PathProvider, argument suggestions are qualified with
// that folder path so accepting one navigates to the right nested path.
func TestCommand_SuggestPrefixesFolderPath(t *testing.T) {
	c, app := newTestCommand(t, nil)
	app.Content.Push(newFakePathView("Jobs[team-a/sub]", "team-a/sub", []string{"deploy", "nightly"}))

	got := c.Suggest("builds d")
	assert.Equal(t, []string{"builds team-a/sub/deploy"}, got)

	got = c.Suggest("jobs ")
	assert.Equal(t, []string{"jobs team-a/sub/deploy", "jobs team-a/sub/nightly"}, got)

	// Typing the path explicitly still narrows correctly.
	got = c.Suggest("logs team-a/sub/n")
	assert.Equal(t, []string{"logs team-a/sub/nightly"}, got)
}

// TestCommand_SuggestNoPathPrefixWhenAtRoot ensures the root JobsView
// (empty path) keeps the original unqualified suggestions.
func TestCommand_SuggestNoPathPrefixWhenAtRoot(t *testing.T) {
	c, app := newTestCommand(t, nil)
	app.Content.Push(newFakePathView("Jobs", "", []string{"alpha", "beta"}))

	got := c.Suggest("builds a")
	assert.Equal(t, []string{"builds alpha"}, got)
}

// TestCommand_RunNavigatesPathBased asserts that path-taking commands
// invoked via the prompt push the matching nested view onto the content
// stack instead of falling through to fuzzy match (which used to open the
// root view, ignoring the path).
func TestCommand_RunNavigatesPathBased(t *testing.T) {
	t.Run("jobs <folderPath>", func(t *testing.T) {
		c, app := newTestCommand(t, nil)
		c.Run("jobs team-a/sub")
		top := app.Content.Top()
		jv, ok := top.(*JobsView)
		require.True(t, ok, "top view should be *JobsView, got %T", top)
		assert.Equal(t, "team-a/sub", jv.folderPath)
	})

	t.Run("builds <jobPath>", func(t *testing.T) {
		c, app := newTestCommand(t, nil)
		c.Run("builds team-a/sub/deploy")
		top := app.Content.Top()
		bv, ok := top.(*BuildsView)
		require.True(t, ok, "top view should be *BuildsView, got %T", top)
		assert.Equal(t, "team-a/sub/deploy", bv.jobName)
	})

	t.Run("logs <jobPath>/<buildNum>", func(t *testing.T) {
		c, app := newTestCommand(t, nil)
		c.Run("logs team-a/sub/deploy/3")
		top := app.Content.Top()
		lv, ok := top.(*LogsView)
		require.True(t, ok, "top view should be *LogsView, got %T", top)
		assert.Equal(t, "team-a/sub/deploy", lv.jobName)
		assert.Equal(t, 3, lv.buildNum)
	})

	t.Run("views <folderPath>", func(t *testing.T) {
		c, app := newTestCommand(t, nil)
		c.Run("views team-a")
		top := app.Content.Top()
		vv, ok := top.(*ViewsView)
		require.True(t, ok, "top view should be *ViewsView, got %T", top)
		assert.Equal(t, "team-a", vv.folderPath)
	})

	t.Run("tests <jobPath>/<buildNum>", func(t *testing.T) {
		c, app := newTestCommand(t, nil)
		c.Run("tests team-a/sub/deploy/3")
		top := app.Content.Top()
		tv, ok := top.(*TestSuitesView)
		require.True(t, ok, "top view should be *TestSuitesView, got %T", top)
		assert.Equal(t, "team-a/sub/deploy", tv.jobName)
		assert.Equal(t, 3, tv.buildNum)
	})

	t.Run("reports <jobPath>/<buildNum>", func(t *testing.T) {
		c, app := newTestCommand(t, nil)
		c.Run("reports team-a/sub/deploy/3")
		top := app.Content.Top()
		rv, ok := top.(*HTMLReportsView)
		require.True(t, ok, "top view should be *HTMLReportsView, got %T", top)
		assert.Equal(t, "team-a/sub/deploy", rv.jobName)
		assert.Equal(t, 3, rv.buildNum)
	})
}

func TestCommand_MatchResourcePath(t *testing.T) {
	c, _ := newTestCommand(t, nil)

	cases := []struct {
		input    string
		wantRes  string
		wantPath string
		wantOk   bool
	}{
		{"jobs team-a/sub", "jobs", "team-a/sub", true},
		{"j team-a", "jobs", "team-a", true},
		{"builds team-a/sub/deploy", "builds", "team-a/sub/deploy", true},
		{"b deploy", "builds", "deploy", true},
		{"logs team-a/sub/deploy/3", "logs", "team-a/sub/deploy/3", true},
		{"tests team-a/sub/deploy/3", "tests", "team-a/sub/deploy/3", true},
		{"test deploy/1", "tests", "deploy/1", true},
		{"reports team-a/deploy/3", "reports", "team-a/deploy/3", true},
		{"report deploy/1", "reports", "deploy/1", true},
		{"views my-view", "views", "my-view", true},
		{"jobs", "", "", false},
		{"jobs   ", "", "", false},
		{"ctx prod", "", "", false},
		{"cache clear", "", "", false},
	}
	for _, tc := range cases {
		res, path, ok := c.matchResourcePath(tc.input)
		if res != tc.wantRes || path != tc.wantPath || ok != tc.wantOk {
			t.Errorf("matchResourcePath(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.input, res, path, ok, tc.wantRes, tc.wantPath, tc.wantOk)
		}
	}
}
