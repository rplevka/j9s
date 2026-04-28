// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAgeDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		// Valid age formats
		{
			name:     "seconds",
			input:    "30s",
			expected: 30,
		},
		{
			name:     "minutes",
			input:    "5m",
			expected: 300, // 5 * 60
		},
		{
			name:     "hours",
			input:    "2h",
			expected: 7200, // 2 * 3600
		},
		{
			name:     "days",
			input:    "3d",
			expected: 259200, // 3 * 86400
		},
		{
			name:     "large seconds",
			input:    "120s",
			expected: 120,
		},
		{
			name:     "large days",
			input:    "365d",
			expected: 31536000, // 365 * 86400
		},
		// Invalid formats
		{
			name:     "empty string",
			input:    "",
			expected: -1,
		},
		{
			name:     "dash",
			input:    "-",
			expected: -1,
		},
		{
			name:     "single char",
			input:    "s",
			expected: -1,
		},
		{
			name:     "no unit",
			input:    "123",
			expected: -1,
		},
		{
			name:     "invalid unit",
			input:    "5x",
			expected: -1,
		},
		{
			name:     "non-numeric",
			input:    "abch",
			expected: -1,
		},
		{
			name:     "negative number",
			input:    "-5m",
			expected: -300, // Parses as -5 * 60, but sorting logic treats < 0 as invalid
		},
		{
			name:     "decimal",
			input:    "1.5h",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAgeDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStripColorTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no tags",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "simple color tag",
			input:    "[red]hello[white]",
			expected: "hello",
		},
		{
			name:     "color with attributes",
			input:    "[red::b]bold text[-::-]",
			expected: "bold text",
		},
		{
			name:     "multiple tags",
			input:    "[green]OK[white] - [yellow]Warning[-]",
			expected: "OK - Warning",
		},
		{
			name:     "nested-like tags",
			input:    "[aqua::b]Status[white::d]: [green]Running[-::-]",
			expected: "Status: Running",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only tags",
			input:    "[red][green][blue]",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripColorTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTable_SortByColumn(t *testing.T) {
	// Create a table with test data
	table := NewTable()
	table.SetHeaders([]string{"Name", "Age", "Count"})
	table.SetData([][]string{
		{"Charlie", "5m", "100"},
		{"Alice", "2h", "50"},
		{"Bob", "30s", "200"},
	})

	// Sort by name (column 0) - ascending
	table.SortByColumn(0)
	sortCol, sortAsc := table.GetSortInfo()
	assert.Equal(t, 0, sortCol)
	assert.True(t, sortAsc)

	// Sort by same column again - should toggle to descending
	table.SortByColumn(0)
	sortCol, sortAsc = table.GetSortInfo()
	assert.Equal(t, 0, sortCol)
	assert.False(t, sortAsc)

	// Sort by different column - should reset to ascending
	table.SortByColumn(1)
	sortCol, sortAsc = table.GetSortInfo()
	assert.Equal(t, 1, sortCol)
	assert.True(t, sortAsc)
}

func TestAgeDurationSorting(t *testing.T) {
	// Test that age durations are compared correctly for sorting
	tests := []struct {
		name     string
		a        string
		b        string
		aSmaller bool // true if a < b
	}{
		{
			name:     "seconds vs minutes",
			a:        "30s",
			b:        "5m",
			aSmaller: true,
		},
		{
			name:     "minutes vs hours",
			a:        "5m",
			b:        "2h",
			aSmaller: true,
		},
		{
			name:     "hours vs days",
			a:        "2h",
			b:        "1d",
			aSmaller: true,
		},
		{
			name:     "same unit comparison",
			a:        "30s",
			b:        "60s",
			aSmaller: true,
		},
		{
			name:     "large vs small",
			a:        "1d",
			b:        "30s",
			aSmaller: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aDur := parseAgeDuration(tt.a)
			bDur := parseAgeDuration(tt.b)

			assert.GreaterOrEqual(t, aDur, int64(0), "a should be valid duration")
			assert.GreaterOrEqual(t, bDur, int64(0), "b should be valid duration")

			if tt.aSmaller {
				assert.Less(t, aDur, bDur)
			} else {
				assert.Greater(t, aDur, bDur)
			}
		})
	}
}

func TestTable_SelectByID(t *testing.T) {
	table := NewTable()
	table.SetHeaders([]string{"NAME", "STATUS"})
	table.SetData([][]string{
		{"job-alpha", "SUCCESS"},
		{"job-beta", "FAILURE"},
		{"job-gamma", "RUNNING"},
	})
	table.Refresh()

	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "select existing item",
			id:       "job-beta",
			expected: true,
		},
		{
			name:     "select first item",
			id:       "job-alpha",
			expected: true,
		},
		{
			name:     "select last item",
			id:       "job-gamma",
			expected: true,
		},
		{
			name:     "select non-existent item",
			id:       "job-delta",
			expected: false,
		},
		{
			name:     "select empty id",
			id:       "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := table.SelectByID(tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTable_GetRowIDs(t *testing.T) {
	t.Run("empty table", func(t *testing.T) {
		table := NewTable()
		assert.Empty(t, table.GetRowIDs())
	})

	t.Run("populated table", func(t *testing.T) {
		table := NewTable()
		table.SetHeaders([]string{"NAME", "STATUS"})
		table.SetData([][]string{
			{"alpha", "SUCCESS"},
			{"beta", "FAILURE"},
			{"gamma", "RUNNING"},
		})
		assert.Equal(t, []string{"alpha", "beta", "gamma"}, table.GetRowIDs())
	})

	t.Run("filtered table", func(t *testing.T) {
		table := NewTable()
		table.SetHeaders([]string{"NAME", "STATUS"})
		table.SetData([][]string{
			{"alpha", "SUCCESS"},
			{"beta", "FAILURE"},
			{"alphabet", "RUNNING"},
		})
		table.Filter("alph")
		ids := table.GetRowIDs()
		assert.ElementsMatch(t, []string{"alpha", "alphabet"}, ids)
	})
}
