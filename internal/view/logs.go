// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/roman-plevka/j9s/internal/cache"
	"github.com/roman-plevka/j9s/internal/model"
	"github.com/roman-plevka/j9s/internal/ui"
)

const (
	maxLogLines = 1000   // Max lines to keep in memory
	maxLogSize  = 500000 // Max bytes before truncating
)

// Spinner frames for loading animation
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// LogsView displays build console output.
type LogsView struct {
	*tview.Flex
	app           *App
	textView      *tview.TextView
	actions       *ui.KeyActions
	jobName       string
	buildNum      int
	filter        string
	autoScroll    bool
	wrapEnabled   bool // Line wrap state
	cancelFn      context.CancelFunc
	logLines      []string // Store log lines for filtering
	lastKey       rune     // Track last key for vim-style sequences (gg)
	loading       bool     // Loading state
	spinnerCancel context.CancelFunc
	hasFullLog    bool         // True if we've loaded the complete log
	cache         *cache.Cache // Log cache
	fromCache     bool         // True if logs were loaded from cache
}

// NewLogsView returns a new logs view.
func NewLogsView(app *App, jobName string, buildNum int) *LogsView {
	v := &LogsView{
		Flex:        tview.NewFlex().SetDirection(tview.FlexRow),
		app:         app,
		textView:    tview.NewTextView(),
		actions:     ui.NewKeyActions(),
		jobName:     jobName,
		buildNum:    buildNum,
		autoScroll:  true,
		wrapEnabled: true,
	}

	v.textView.SetDynamicColors(true)
	v.textView.SetScrollable(true)
	v.textView.SetBackgroundColor(tcell.ColorDefault)
	v.textView.SetWrap(v.wrapEnabled)

	v.AddItem(v.textView, 0, 1, true)
	v.bindKeys()
	v.startStreaming()

	return v
}

// Name returns the view name.
func (v *LogsView) Name() string {
	return fmt.Sprintf("Logs[%s#%d]", v.jobName, v.buildNum)
}

// Hints returns the view hints.
func (v *LogsView) Hints() model.MenuHints {
	return v.actions.Hints()
}

func (v *LogsView) bindKeys() {
	// Add all keys with proper visibility for menu hints
	v.actions.Bulk(ui.KeyMap{
		tcell.KeyCtrlC: ui.NewKeyAction("Quit", func(*tcell.EventKey) *tcell.EventKey {
			if v.cancelFn != nil {
				v.cancelFn()
			}
			v.app.Stop()
			return nil
		}, true),
		ui.KeySlash:  ui.NewKeyAction("Filter", v.filterCmd, true),
		ui.KeyShiftG: ui.NewKeyAction("Bottom", v.bottomCmd, true),
		ui.KeyG:      ui.NewKeyAction("Top (gg)", nil, true), // Just for hint, handled in input capture
		ui.KeyW:      ui.NewKeyAction("Wrap", v.toggleWrapCmd, true),
		ui.KeyS:      ui.NewKeyAction("AutoScroll", v.toggleScrollCmd, true),
		ui.KeyC:      ui.NewKeyAction("Clear", v.clearFilterCmd, true),
		ui.KeyQ:      ui.NewKeyAction("Back", v.backCmd, true),
		tcell.KeyEsc: ui.NewKeyAction("Back", v.backCmd, false), // Not visible in menu
		ui.Key0:      ui.NewKeyAction("Tail", v.tailCmd, true),
		ui.Key1:      ui.NewKeyAction("Head", v.headCmd, true),
	})

	v.textView.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		key := evt.Key()
		r := evt.Rune()

		// Handle horizontal scrolling with left/right arrows when wrap is disabled
		// Use step size of 10 columns for faster scrolling (similar to k9s)
		const hScrollStep = 10
		if !v.wrapEnabled {
			switch key {
			case tcell.KeyLeft:
				row, col := v.textView.GetScrollOffset()
				newCol := col - hScrollStep
				if newCol < 0 {
					newCol = 0
				}
				v.textView.ScrollTo(row, newCol)
				return nil
			case tcell.KeyRight:
				row, col := v.textView.GetScrollOffset()
				v.textView.ScrollTo(row, col+hScrollStep)
				return nil
			}
		}

		// Vim-like horizontal navigation (work regardless of wrap mode)
		if key == tcell.KeyRune {
			switch r {
			case '^', '0': // Beginning of line
				row, _ := v.textView.GetScrollOffset()
				v.textView.ScrollTo(row, 0)
				return nil
			case '$': // End of line (scroll far right)
				row, _ := v.textView.GetScrollOffset()
				v.textView.ScrollTo(row, 500) // Large value to scroll to end
				return nil
			case 'h': // Left (vim style)
				if !v.wrapEnabled {
					row, col := v.textView.GetScrollOffset()
					newCol := col - hScrollStep
					if newCol < 0 {
						newCol = 0
					}
					v.textView.ScrollTo(row, newCol)
					return nil
				}
			case 'l': // Right (vim style)
				if !v.wrapEnabled {
					row, col := v.textView.GetScrollOffset()
					v.textView.ScrollTo(row, col+hScrollStep)
					return nil
				}
			case '{': // Previous paragraph/empty line
				row, col := v.textView.GetScrollOffset()
				newRow := row - 10
				if newRow < 0 {
					newRow = 0
				}
				v.textView.ScrollTo(newRow, col)
				return nil
			case '}': // Next paragraph/empty line
				row, col := v.textView.GetScrollOffset()
				v.textView.ScrollTo(row+10, col)
				return nil
			case '%': // Jump to matching bracket - scroll half screen
				row, col := v.textView.GetScrollOffset()
				// Simple implementation: jump half the visible height
				v.textView.ScrollTo(row+20, col)
				return nil
			}
		}

		// Handle vim-style gg (go to beginning)
		if key == tcell.KeyRune && r == 'g' {
			if v.lastKey == 'g' {
				// gg - go to very beginning
				v.lastKey = 0
				v.goToBeginning()
				return nil
			}
			v.lastKey = 'g'
			return nil
		}

		// Reset lastKey for any other key
		v.lastKey = 0

		if key == tcell.KeyRune {
			key = tcell.Key(r)
		}
		if action, ok := v.actions.Get(key); ok {
			return action.Action(evt)
		}
		return evt
	})
}

// goToBeginning fetches and displays the full log from the beginning.
func (v *LogsView) goToBeginning() {
	v.autoScroll = false

	// If we already have full log, just scroll to beginning
	if v.hasFullLog {
		v.textView.ScrollToBeginning()
		v.app.Flash().Info("At beginning of log")
		return
	}

	// Fetch full log from beginning
	if v.app.Client() == nil {
		return
	}

	// Stop the streaming to prevent it from overwriting our full log
	if v.cancelFn != nil {
		v.cancelFn()
	}

	v.loading = true

	go func() {
		// Use a timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Progress callback to show download progress
		var lastUpdate time.Time
		progress := func(bytesRead int64, totalSize int64) {
			// Throttle updates to every 100ms
			if time.Since(lastUpdate) < 100*time.Millisecond {
				return
			}
			lastUpdate = time.Now()
			v.app.QueueUpdateDraw(func() {
				if totalSize > 0 {
					pct := float64(bytesRead) / float64(totalSize) * 100
					v.app.Flash().Info(fmt.Sprintf("⏳ Downloading... %s / %s (%.0f%%)",
						formatBytes(bytesRead), formatBytes(totalSize), pct))
				} else {
					v.app.Flash().Info(fmt.Sprintf("⏳ Downloading... %s", formatBytes(bytesRead)))
				}
			})
		}

		// Use GetBuildConsoleOutputFullWithProgress for complete logs
		text, err := v.app.Client().GetBuildConsoleOutputFullWithProgress(ctx, v.jobName, v.buildNum, progress)

		v.loading = false

		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.app.Flash().Err(err)
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			// Replace log lines with full log
			v.logLines = strings.Split(text, "\n")
			v.hasFullLog = true
			v.renderLog()
			v.textView.ScrollToBeginning()
			v.app.Flash().Info(fmt.Sprintf("✓ Loaded %d lines (%s)", len(v.logLines), formatBytes(int64(len(text)))))
		})
	}()
}

// formatBytes formats bytes into human readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// startSpinner starts the loading spinner animation.
func (v *LogsView) startSpinner(msg string) {
	v.loading = true
	ctx, cancel := context.WithCancel(context.Background())
	v.spinnerCancel = cancel

	go func() {
		frame := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				v.app.QueueUpdateDraw(func() {
					v.app.Flash().Info(fmt.Sprintf("%c %s...", spinnerFrames[frame], msg))
				})
				frame = (frame + 1) % len(spinnerFrames)
			}
		}
	}()
}

// stopSpinner stops the loading spinner.
func (v *LogsView) stopSpinner() {
	v.loading = false
	if v.spinnerCancel != nil {
		v.spinnerCancel()
		v.spinnerCancel = nil
	}
}

func (v *LogsView) startStreaming() {
	ctx, cancel := context.WithCancel(context.Background())
	v.cancelFn = cancel

	// Initialize cache
	if v.app.Config() != nil {
		c, err := cache.New(v.app.Config().J9s.Cache)
		if err == nil {
			v.cache = c
		}
	}

	go func() {
		// Try to load from cache first for completed builds
		if v.tryLoadFromCache() {
			return
		}

		var offset int64 = 0
		var totalBytes int64 = 0
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Show initial loading message
		v.app.QueueUpdateDraw(func() {
			v.app.Flash().Info("⏳ Loading logs...")
		})

		// Initial fetch
		text, newOffset, moreData, err := v.app.Client().StreamBuildConsoleOutput(
			ctx, v.jobName, v.buildNum, offset,
		)
		if err == nil && text != "" {
			totalBytes += int64(len(text))
			v.app.QueueUpdateDraw(func() {
				v.appendText(text)
				v.app.Flash().Info(fmt.Sprintf("Loaded %s", formatBytes(totalBytes)))
			})
		}
		offset = newOffset

		if !moreData {
			// Build complete, cache the log
			v.cacheLog()
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				text, newOffset, moreData, err := v.app.Client().StreamBuildConsoleOutput(
					ctx, v.jobName, v.buildNum, offset,
				)
				if err != nil {
					continue
				}

				if text != "" {
					totalBytes += int64(len(text))
					v.app.QueueUpdateDraw(func() {
						v.appendText(text)
						if v.autoScroll {
							v.app.Flash().Info(fmt.Sprintf("Streaming... %s", formatBytes(totalBytes)))
						}
					})
				}

				offset = newOffset

				// Stop polling if no more data and build is complete
				if !moreData {
					v.app.QueueUpdateDraw(func() {
						v.app.Flash().Info(fmt.Sprintf("✓ Log complete: %s", formatBytes(totalBytes)))
					})
					// Cache the completed log
					v.cacheLog()
					return
				}
			}
		}
	}()
}

// tryLoadFromCache attempts to load logs from cache for completed builds.
func (v *LogsView) tryLoadFromCache() bool {
	if v.cache == nil || !v.cache.IsEnabled() {
		return false
	}

	ctxName := ""
	if ctx, _ := v.app.Config().ActiveContext(); ctx != nil {
		ctxName = ctx.Name
	}

	// Check if we have cached logs for this build
	if !v.cache.HasLog(ctxName, v.jobName, v.buildNum) {
		return false
	}

	// Check if the build is complete (not still building)
	if !v.cache.IsBuildComplete(ctxName, v.jobName, v.buildNum) {
		return false
	}

	// Load from cache
	log, err := v.cache.GetLog(ctxName, v.jobName, v.buildNum)
	if err != nil {
		return false
	}

	v.app.QueueUpdateDraw(func() {
		v.logLines = strings.Split(log, "\n")
		v.hasFullLog = true
		v.fromCache = true
		v.renderLog()
		v.app.Flash().Info(fmt.Sprintf("📦 Loaded from cache: %d lines (%s)", len(v.logLines), formatBytes(int64(len(log)))))
	})

	return true
}

// cacheLog saves the current log to cache.
func (v *LogsView) cacheLog() {
	if v.cache == nil || !v.cache.IsEnabled() {
		return
	}

	ctxName := ""
	if ctx, _ := v.app.Config().ActiveContext(); ctx != nil {
		ctxName = ctx.Name
	}

	// Get build info to check if complete
	build, err := v.app.Client().GetBuild(context.Background(), v.jobName, v.buildNum)
	if err != nil || build.Building {
		return // Don't cache incomplete builds
	}

	// Save log
	log := strings.Join(v.logLines, "\n")
	if err := v.cache.PutLog(ctxName, v.jobName, v.buildNum, log); err != nil {
		return
	}

	// Save metadata
	meta := &cache.BuildMeta{
		JobName:     v.jobName,
		BuildNumber: v.buildNum,
		Result:      build.Result,
		Building:    build.Building,
		Timestamp:   build.Timestamp,
		Duration:    build.Duration,
	}
	v.cache.PutMeta(ctxName, meta)
}

func (v *LogsView) appendText(text string) {
	// Split into lines and append
	newLines := strings.Split(text, "\n")
	v.logLines = append(v.logLines, newLines...)

	// Truncate if too many lines
	if len(v.logLines) > maxLogLines {
		v.logLines = v.logLines[len(v.logLines)-maxLogLines:]
	}

	// Display with current filter
	v.renderLog()
}

func (v *LogsView) renderLog() {
	v.textView.Clear()

	if v.filter == "" {
		// No filter - just show last N lines
		start := 0
		if len(v.logLines) > maxLogLines {
			start = len(v.logLines) - maxLogLines
		}
		for i := start; i < len(v.logLines); i++ {
			fmt.Fprintln(v.textView, v.logLines[i])
		}
	} else {
		filterLower := strings.ToLower(v.filter)
		for _, line := range v.logLines {
			if strings.Contains(strings.ToLower(line), filterLower) {
				// Highlight matching text
				highlighted := v.highlightMatch(line, v.filter)
				fmt.Fprintln(v.textView, highlighted)
			}
		}
	}

	if v.autoScroll {
		v.textView.ScrollToEnd()
	}
}

// highlightMatch highlights all occurrences of the filter in the line.
// Uses the shared ui.HighlightMatches function for consistent styling.
func (v *LogsView) highlightMatch(line, filter string) string {
	// Escape any existing [ characters to prevent them being parsed as color tags
	escaped := strings.ReplaceAll(line, "[", "[[]")
	return ui.HighlightMatches(escaped, filter)
}

// SetFilter sets the filter and re-renders the log.
func (v *LogsView) SetFilter(filter string) {
	v.filter = filter
	v.renderLog()
}

func (v *LogsView) filterCmd(*tcell.EventKey) *tcell.EventKey {
	// Use the app's filter mode
	v.app.filterCmd(nil)
	return nil
}

func (v *LogsView) clearFilterCmd(*tcell.EventKey) *tcell.EventKey {
	v.filter = ""
	v.renderLog()
	v.app.Flash().Info("Filter cleared")
	return nil
}

func (v *LogsView) topCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToBeginning()
	v.autoScroll = false
	return nil
}

func (v *LogsView) bottomCmd(*tcell.EventKey) *tcell.EventKey {
	v.textView.ScrollToEnd()
	v.autoScroll = true
	return nil
}

// headCmd loads the full log from the beginning (like k9s key '1')
func (v *LogsView) headCmd(*tcell.EventKey) *tcell.EventKey {
	v.goToBeginning()
	return nil
}

// tailCmd scrolls to the end and enables auto-scroll (like k9s key '0')
func (v *LogsView) tailCmd(*tcell.EventKey) *tcell.EventKey {
	v.autoScroll = true
	v.textView.ScrollToEnd()
	v.app.Flash().Info("Following log tail")
	return nil
}

func (v *LogsView) fullScreenCmd(*tcell.EventKey) *tcell.EventKey {
	// TODO: Toggle full screen mode
	return nil
}

func (v *LogsView) toggleWrapCmd(*tcell.EventKey) *tcell.EventKey {
	v.wrapEnabled = !v.wrapEnabled
	v.textView.SetWrap(v.wrapEnabled)
	if v.wrapEnabled {
		v.app.Flash().Info("Line wrap enabled")
	} else {
		v.app.Flash().Info("Line wrap disabled")
	}
	return nil
}

func (v *LogsView) toggleScrollCmd(*tcell.EventKey) *tcell.EventKey {
	v.autoScroll = !v.autoScroll
	if v.autoScroll {
		v.app.Flash().Info("Auto-scroll enabled")
	} else {
		v.app.Flash().Info("Auto-scroll disabled")
	}
	return nil
}

func (v *LogsView) backCmd(*tcell.EventKey) *tcell.EventKey {
	if v.cancelFn != nil {
		v.cancelFn()
	}
	if v.app.Content.CanPop() {
		v.app.Content.Pop()
	}
	return nil
}
