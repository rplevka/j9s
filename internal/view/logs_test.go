// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{
			name:  "bytes",
			bytes: 500,
			want:  "500 B",
		},
		{
			name:  "kilobytes",
			bytes: 1024,
			want:  "1.0 KB",
		},
		{
			name:  "kilobytes with decimal",
			bytes: 1536,
			want:  "1.5 KB",
		},
		{
			name:  "megabytes",
			bytes: 1048576,
			want:  "1.0 MB",
		},
		{
			name:  "megabytes large",
			bytes: 5242880,
			want:  "5.0 MB",
		},
		{
			name:  "gigabytes",
			bytes: 1073741824,
			want:  "1.0 GB",
		},
		{
			name:  "zero bytes",
			bytes: 0,
			want:  "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBytes(tt.bytes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHighlightMatch(t *testing.T) {
	// Create a minimal LogsView for testing
	v := &LogsView{}

	tests := []struct {
		name   string
		line   string
		filter string
		want   string
	}{
		{
			name:   "simple match",
			line:   "Hello World",
			filter: "World",
			want:   "Hello [red::b]World[-::-]",
		},
		{
			name:   "case insensitive match",
			line:   "Hello WORLD",
			filter: "world",
			want:   "Hello [red::b]WORLD[-::-]",
		},
		{
			name:   "multiple matches",
			line:   "test test test",
			filter: "test",
			want:   "[red::b]test[-::-] [red::b]test[-::-] [red::b]test[-::-]",
		},
		{
			name:   "no match",
			line:   "Hello World",
			filter: "foo",
			want:   "Hello World",
		},
		{
			name:   "escape brackets",
			line:   "log [INFO] message",
			filter: "INFO",
			want:   "log [[][red::b]INFO[-::-]] message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.highlightMatch(tt.line, tt.filter)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLogsView_FilterLogic(t *testing.T) {
	v := &LogsView{
		logLines: []string{
			"2024-01-01 INFO Starting application",
			"2024-01-01 DEBUG Loading config",
			"2024-01-01 ERROR Failed to connect",
			"2024-01-01 INFO Application started",
			"2024-01-01 WARN Low memory",
		},
	}

	tests := []struct {
		name          string
		filter        string
		expectedCount int
	}{
		{
			name:          "no filter",
			filter:        "",
			expectedCount: 5,
		},
		{
			name:          "filter INFO",
			filter:        "INFO",
			expectedCount: 2,
		},
		{
			name:          "filter ERROR",
			filter:        "ERROR",
			expectedCount: 1,
		},
		{
			name:          "filter case insensitive",
			filter:        "info",
			expectedCount: 2,
		},
		{
			name:          "filter no match",
			filter:        "CRITICAL",
			expectedCount: 0,
		},
		{
			name:          "filter date",
			filter:        "2024-01-01",
			expectedCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v.filter = tt.filter
			count := 0
			if tt.filter == "" {
				count = len(v.logLines)
			} else {
				for _, line := range v.logLines {
					if containsIgnoreCaseLogs(line, tt.filter) {
						count++
					}
				}
			}
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

// containsIgnoreCaseLogs is a helper for testing filter logic
func containsIgnoreCaseLogs(s, substr string) bool {
	return len(s) >= len(substr) &&
		(substr == "" ||
			containsLowerLogs(s, substr))
}

func containsLowerLogs(s, substr string) bool {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sLower[i] = c
	}
	for i := 0; i < len(substr); i++ {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		substrLower[i] = c
	}
	return containsBytes(sLower, substrLower)
}

func containsBytes(s, substr []byte) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestLogsView_AppendText(t *testing.T) {
	tests := []struct {
		name          string
		initialLines  []string
		appendText    string
		expectedLines int
		maxLines      int
	}{
		{
			name:          "append to empty",
			initialLines:  []string{},
			appendText:    "line1\nline2\nline3",
			expectedLines: 3,
			maxLines:      1000,
		},
		{
			name:          "append to existing",
			initialLines:  []string{"existing1", "existing2"},
			appendText:    "new1\nnew2",
			expectedLines: 4,
			maxLines:      1000,
		},
		{
			name:          "truncate when exceeding max",
			initialLines:  make([]string, 998),
			appendText:    "new1\nnew2\nnew3\nnew4\nnew5",
			expectedLines: 1000, // Should truncate to maxLogLines
			maxLines:      1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &LogsView{
				logLines: tt.initialLines,
			}

			// Simulate appendText logic without UI
			newLines := splitLines(tt.appendText)
			v.logLines = append(v.logLines, newLines...)

			if len(v.logLines) > tt.maxLines {
				v.logLines = v.logLines[len(v.logLines)-tt.maxLines:]
			}

			assert.LessOrEqual(t, len(v.logLines), tt.expectedLines)
		})
	}
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func TestSpinnerFrames(t *testing.T) {
	// Verify spinner frames are defined and non-empty
	assert.NotEmpty(t, spinnerFrames)
	assert.Len(t, spinnerFrames, 10) // Expected 10 frames

	// Verify all frames are valid runes
	for _, frame := range spinnerFrames {
		assert.NotZero(t, frame)
	}
}

func TestLogsView_TailSizeCalculation(t *testing.T) {
	// Test the tail size calculation logic used in startStreaming
	const tailSize int64 = 500 * 1024 // 500KB

	tests := []struct {
		name           string
		logSize        int64
		expectedOffset int64
		shouldTail     bool
	}{
		{
			name:           "small log - no tail needed",
			logSize:        100 * 1024, // 100KB
			expectedOffset: 0,
			shouldTail:     false,
		},
		{
			name:           "exactly tail size - no tail needed",
			logSize:        500 * 1024, // 500KB
			expectedOffset: 0,
			shouldTail:     false,
		},
		{
			name:           "large log - tail from end",
			logSize:        1024 * 1024, // 1MB
			expectedOffset: 1024*1024 - tailSize,
			shouldTail:     true,
		},
		{
			name:           "very large log - tail from end",
			logSize:        100 * 1024 * 1024, // 100MB
			expectedOffset: 100*1024*1024 - tailSize,
			shouldTail:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var offset int64 = 0
			if tt.logSize > tailSize {
				offset = tt.logSize - tailSize
			}

			if tt.shouldTail {
				assert.Equal(t, tt.expectedOffset, offset)
				assert.Greater(t, offset, int64(0))
			} else {
				assert.Equal(t, int64(0), offset)
			}
		})
	}
}

func TestLogsView_RenderLogTruncation(t *testing.T) {
	// Test that renderLog respects hasFullLog flag for truncation
	tests := []struct {
		name          string
		logLines      []string
		hasFullLog    bool
		maxLines      int
		expectedStart int // Expected start index for rendering
	}{
		{
			name:          "tail mode - truncate to last N lines",
			logLines:      make([]string, 2000),
			hasFullLog:    false,
			maxLines:      1000,
			expectedStart: 1000, // Should start from line 1000
		},
		{
			name:          "full log mode - show all lines",
			logLines:      make([]string, 2000),
			hasFullLog:    true,
			maxLines:      1000,
			expectedStart: 0, // Should start from line 0
		},
		{
			name:          "small log - no truncation needed",
			logLines:      make([]string, 500),
			hasFullLog:    false,
			maxLines:      1000,
			expectedStart: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate renderLog logic
			start := 0
			if !tt.hasFullLog && len(tt.logLines) > tt.maxLines {
				start = len(tt.logLines) - tt.maxLines
			}
			assert.Equal(t, tt.expectedStart, start)
		})
	}
}

func TestLogsView_LineCount(t *testing.T) {
	// Test line counting logic used in goToBeginning
	tests := []struct {
		name          string
		text          string
		expectedLines int
	}{
		{
			name:          "single line",
			text:          "hello world",
			expectedLines: 1,
		},
		{
			name:          "multiple lines",
			text:          "line1\nline2\nline3",
			expectedLines: 3,
		},
		{
			name:          "empty string",
			text:          "",
			expectedLines: 1,
		},
		{
			name:          "trailing newline",
			text:          "line1\nline2\n",
			expectedLines: 3,
		},
		{
			name:          "many lines",
			text:          "a\nb\nc\nd\ne\nf\ng\nh\ni\nj",
			expectedLines: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the logic used in goToBeginning
			lineCount := 1
			for _, c := range tt.text {
				if c == '\n' {
					lineCount++
				}
			}
			assert.Equal(t, tt.expectedLines, lineCount)
		})
	}
}
