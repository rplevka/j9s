// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenURLFunc_Indirection asserts that callers can inject a fake
// implementation, which is how the HTMLReportsView tests avoid spawning
// a real browser process during go test runs.
func TestOpenURLFunc_Indirection(t *testing.T) {
	original := openURLFunc
	t.Cleanup(func() { openURLFunc = original })

	var captured string
	openURLFunc = func(url string) error {
		captured = url
		return nil
	}

	err := openURLFunc("https://example.com/build/3/pytest_html/")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/build/3/pytest_html/", captured)
}

// TestOpenURL_EmptyURL guards the input validation so the openURL helper
// flashes a sensible error rather than blindly handing an empty string
// to `open`/`xdg-open`.
func TestOpenURL_EmptyURL(t *testing.T) {
	err := openURL("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// TestOpenURLFunc_PropagatesError checks that error returns from the
// fake are surfaced verbatim to the caller (so HTMLReportsView can flash
// them on screen).
func TestOpenURLFunc_PropagatesError(t *testing.T) {
	original := openURLFunc
	t.Cleanup(func() { openURLFunc = original })

	openURLFunc = func(string) error {
		return errors.New("boom")
	}

	err := openURLFunc("https://example.com/")
	require.Error(t, err)
	assert.Equal(t, "boom", err.Error())
}
