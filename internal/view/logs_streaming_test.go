// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"strings"
	"testing"

	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/client/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLogsView constructs a minimal LogsView suitable for direct unit
// testing without a running tview.Application. NewLogsView spawns goroutines
// that depend on App.QueueUpdateDraw, so we hand-build the struct here and
// drive appendText synchronously.
func newTestLogsView(jobName string, buildNum int) *LogsView {
	return &LogsView{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		textView:    tview.NewTextView().SetDynamicColors(true),
		jobName:     jobName,
		buildNum:    buildNum,
		autoScroll:  true,
		wrapEnabled: true,
	}
}

// TestLogsView_AppendsLiveStreamChunks replicates the inner loop of
// LogsView.startStreaming synchronously: poll progressiveText starting from
// the previous offset, append each chunk via appendText, until X-More-Data
// flips to false. Asserts the text view contains all chunks in order.
func TestLogsView_AppendsLiveStreamChunks(t *testing.T) {
	chunks := []string{"chunk-a\n", "chunk-b\n", "chunk-c\n"}
	srv := mock.NewJenkinsServer(t).
		WithJob("live", mock.JobOpts{Color: "blue_anime", LastBuildNumber: 5}).
		WithLiveBuild("live", 5, chunks)

	c := mock.NewClient(t, srv)
	v := newTestLogsView("live", 5)

	var (
		offset   int64
		moreData = true
		err      error
		text     string
	)
	for moreData {
		text, offset, moreData, err = c.StreamBuildConsoleOutput(context.Background(), "live", 5, offset)
		require.NoError(t, err)
		if text != "" {
			v.appendText(text)
		}
	}

	got := v.textView.GetText(true)
	for _, c := range chunks {
		assert.Contains(t, got, strings.TrimRight(c, "\n"))
	}
	// Order check: chunk-a precedes chunk-b precedes chunk-c.
	posA := strings.Index(got, "chunk-a")
	posB := strings.Index(got, "chunk-b")
	posC := strings.Index(got, "chunk-c")
	require.True(t, posA < posB && posB < posC, "chunks must appear in stream order; got %q", got)
}

// TestLogsView_GoToBeginningReplaysFullLog mimics the `<0>` flow: after the
// stream has completed, the user requests the full log. LogsView clears
// state and re-fetches via /consoleText, then rebuilds the view by calling
// appendText on the full text. Verifies the cumulative log is identical to
// the original chunks concatenated.
func TestLogsView_GoToBeginningReplaysFullLog(t *testing.T) {
	chunks := []string{"alpha\n", "beta\n", "gamma\n"}
	srv := mock.NewJenkinsServer(t).
		WithJob("live", mock.JobOpts{Color: "blue_anime", LastBuildNumber: 9}).
		WithLiveBuild("live", 9, chunks)

	c := mock.NewClient(t, srv)
	v := newTestLogsView("live", 9)

	// Drain the live stream first so the cumulative buffer is populated.
	var offset int64
	moreData := true
	for moreData {
		text, off, more, err := c.StreamBuildConsoleOutput(context.Background(), "live", 9, offset)
		require.NoError(t, err)
		if text != "" {
			v.appendText(text)
		}
		offset, moreData = off, more
	}

	// `<0>` flow: clear and reload the full log from the start.
	full, err := c.GetBuildConsoleOutputFull(context.Background(), "live", 9)
	require.NoError(t, err)
	assert.Equal(t, "alpha\nbeta\ngamma\n", full)

	v.logLines = nil
	v.textView.Clear()
	v.hasFullLog = true
	v.appendText(full)

	got := v.textView.GetText(true)
	assert.Contains(t, got, "alpha")
	assert.Contains(t, got, "beta")
	assert.Contains(t, got, "gamma")
	assert.Less(t, strings.Index(got, "alpha"), strings.Index(got, "beta"))
	assert.Less(t, strings.Index(got, "beta"), strings.Index(got, "gamma"))
	assert.True(t, v.hasFullLog)
}
