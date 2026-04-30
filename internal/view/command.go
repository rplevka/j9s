// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/roman-plevka/j9s/internal/cache"
	"github.com/sahilm/fuzzy"
)

// Alias represents a command alias.
type Alias struct {
	Resource string
	Aliases  []string
}

var defaultAliases = []Alias{
	{Resource: "jobs", Aliases: []string{"job", "j"}},
	{Resource: "builds", Aliases: []string{"build", "b"}},
	{Resource: "queue", Aliases: []string{"qu"}},
	{Resource: "nodes", Aliases: []string{"node", "n", "agents", "agent"}},
	{Resource: "users", Aliases: []string{"user", "u"}},
	{Resource: "credentials", Aliases: []string{"cred", "creds", "cr"}},
	{Resource: "plugins", Aliases: []string{"plugin", "pl"}},
	{Resource: "views", Aliases: []string{"view", "v"}},
	{Resource: "contexts", Aliases: []string{"context", "ctx"}},
}

// Special commands like k9s
var quitCommands = map[string]bool{
	"q":    true,
	"q!":   true,
	"qa":   true,
	"quit": true,
	"exit": true,
}

var helpCommands = map[string]bool{
	"?":    true,
	"h":    true,
	"help": true,
}

var cacheCommands = map[string]bool{
	"cache":       true,
	"cache clear": true,
	"cache stats": true,
}

var bookmarkCommands = map[string]bool{
	"bookmark":       true,
	"bookmark clear": true,
	"bm":             true,
	"bm clear":       true,
}

// Command handles command execution.
type Command struct {
	app     *App
	aliases map[string]string
}

// NewCommand returns a new command handler.
func NewCommand(app *App) *Command {
	return &Command{
		app:     app,
		aliases: make(map[string]string),
	}
}

// getCurrentFolderPath returns the current folder path if the top view is a JobsView in a folder.
func (c *Command) getCurrentFolderPath() string {
	top := c.app.Content.Top()
	if top == nil {
		return ""
	}
	// Check if it's a JobsView with a folder path
	if jobsView, ok := top.(*JobsView); ok {
		return jobsView.folderPath
	}
	return ""
}

// Init initializes the command handler.
func (c *Command) Init() error {
	for _, a := range defaultAliases {
		c.aliases[a.Resource] = a.Resource
		for _, alias := range a.Aliases {
			c.aliases[alias] = a.Resource
		}
	}
	return nil
}

// Run executes a command.
func (c *Command) Run(cmd string) {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	if cmd == "" {
		return
	}

	// Check for quit commands
	if quitCommands[cmd] {
		c.app.Stop()
		return
	}

	// Check for help commands
	if helpCommands[cmd] {
		c.app.Flash().Info("Commands: jobs, builds, queue, nodes, views | ctx [<name>], bookmark, url <jenkins-url> | cache [clear|stats] | :q to quit")
		return
	}

	// ctx <name> switches the active Jenkins context.
	if res, ok := c.matchCtxArg(cmd); ok {
		c.switchContext(res)
		return
	}

	// Check for cache commands
	if strings.HasPrefix(cmd, "cache") {
		c.handleCacheCommand(cmd)
		return
	}

	// Check for bookmark commands
	if strings.HasPrefix(cmd, "bookmark") || strings.HasPrefix(cmd, "bm") {
		c.handleBookmarkCommand(cmd)
		return
	}

	// Check for url command
	if strings.HasPrefix(cmd, "url ") {
		c.handleURLCommand(cmd)
		return
	}

	// Check for direct resource match
	if res, ok := c.aliases[cmd]; ok {
		c.gotoResource(res)
		return
	}

	// Path-based navigation: `<resource> <path>` for jobs/builds/logs/views.
	// Reuses the bookmark-style createViewFromPath but pushes the new view
	// onto the stack (without clearing) so Esc returns to the previous view.
	if res, sub, ok := c.matchResourcePath(cmd); ok {
		c.navigatePromptPath(res + "/" + sub)
		return
	}

	// Try fuzzy match
	resources := make([]string, 0, len(defaultAliases))
	for _, a := range defaultAliases {
		resources = append(resources, a.Resource)
	}

	matches := fuzzy.Find(cmd, resources)
	if len(matches) > 0 {
		c.gotoResource(matches[0].Str)
		return
	}

	c.app.Flash().Warn("Unknown command: " + cmd)
}

func (c *Command) handleCacheCommand(cmd string) {
	parts := strings.Fields(cmd)

	// Initialize cache
	cacheInst, err := cache.New(c.app.Config().J9s.Cache)
	if err != nil {
		c.app.Flash().Err(fmt.Errorf("cache error: %w", err))
		return
	}

	if len(parts) == 1 {
		// Just "cache" - show stats
		count, size, err := cacheInst.Stats()
		if err != nil {
			c.app.Flash().Err(fmt.Errorf("cache stats error: %w", err))
			return
		}
		c.app.Flash().Info(fmt.Sprintf("Cache: %d builds, %s | Use 'cache clear' to clear", count, formatCacheSize(size)))
		return
	}

	switch parts[1] {
	case "clear":
		if err := cacheInst.Clear(); err != nil {
			c.app.Flash().Err(fmt.Errorf("cache clear error: %w", err))
			return
		}
		c.app.Flash().Info("✓ Cache cleared")
	case "stats":
		count, size, err := cacheInst.Stats()
		if err != nil {
			c.app.Flash().Err(fmt.Errorf("cache stats error: %w", err))
			return
		}
		c.app.Flash().Info(fmt.Sprintf("Cache: %d builds, %s", count, formatCacheSize(size)))
	case "cleanup":
		if err := cacheInst.Cleanup(); err != nil {
			c.app.Flash().Err(fmt.Errorf("cache cleanup error: %w", err))
			return
		}
		c.app.Flash().Info("✓ Cache cleanup complete")
	default:
		c.app.Flash().Warn("Unknown cache command. Use: cache [clear|stats|cleanup]")
	}
}

func formatCacheSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func (c *Command) handleBookmarkCommand(cmd string) {
	parts := strings.Fields(cmd)

	// Get current view path
	top := c.app.Content.Top()
	if top == nil {
		c.app.Flash().Warn("No view to bookmark")
		return
	}

	// Check for clear command
	if len(parts) >= 2 && parts[1] == "clear" {
		if err := c.app.Config().ClearBookmark(); err != nil {
			c.app.Flash().Err(fmt.Errorf("failed to clear bookmark: %w", err))
			return
		}
		c.app.Flash().Info("✓ Bookmark cleared")
		return
	}

	// Get the view's bookmark path
	var bookmarkPath string
	if urlProvider, ok := top.(URLProvider); ok {
		bookmarkPath = urlProvider.GetViewPath()
	} else {
		// Fallback to view name
		if named, ok := top.(interface{ Name() string }); ok {
			bookmarkPath = named.Name()
		} else {
			c.app.Flash().Warn("Cannot bookmark this view")
			return
		}
	}

	// Collect selections from the entire stack for proper back navigation
	selections := c.collectStackSelections()

	if err := c.app.Config().SetBookmarkWithSelections(bookmarkPath, selections); err != nil {
		c.app.Flash().Err(fmt.Errorf("failed to set bookmark: %w", err))
		return
	}
	c.app.Flash().Info(fmt.Sprintf("✓ Bookmark set: %s", bookmarkPath))
}

// collectStackSelections derives selections from the bookmark path.
// For builds/folder/subfolder/my-job, it returns:
// {"jobs": "folder", "jobs/folder": "subfolder", "jobs/folder/subfolder": "my-job"}
func (c *Command) collectStackSelections() map[string]string {
	selections := make(map[string]string)

	// Get the bookmark path from the current top view
	top := c.app.Content.Top()
	if top == nil {
		return selections
	}

	var bookmarkPath string
	if urlProvider, ok := top.(URLProvider); ok {
		bookmarkPath = urlProvider.GetViewPath()
	} else {
		return selections
	}

	// Parse the bookmark path to derive selections
	parts := strings.SplitN(bookmarkPath, "/", 2)
	if len(parts) < 2 {
		return selections
	}

	resource := parts[0]
	subPath := parts[1]

	// For builds/folder/subfolder/my-job, subPath is "folder/subfolder/my-job"
	// We need to derive selections for each folder level
	switch resource {
	case "builds":
		// Split the path into components
		pathParts := strings.Split(subPath, "/")
		// Build selections for each level
		// pathParts = ["folder", "subfolder", "my-job"]
		// jobs -> folder, jobs/folder -> subfolder, jobs/folder/subfolder -> my-job
		for i := 0; i < len(pathParts); i++ {
			var viewPath string
			if i == 0 {
				viewPath = "jobs"
			} else {
				viewPath = "jobs/" + strings.Join(pathParts[:i], "/")
			}
			selections[viewPath] = pathParts[i]
		}
	case "logs":
		// logs/folder/subfolder/my-job/123
		// subPath is "folder/subfolder/my-job/123"
		// We need: jobs -> folder, jobs/folder -> subfolder, jobs/folder/subfolder -> my-job
		// And: builds/folder/subfolder/my-job -> #123
		pathParts := strings.Split(subPath, "/")
		if len(pathParts) >= 2 {
			// All but the last part is the job path, last part is build number
			jobParts := pathParts[:len(pathParts)-1]
			buildNum := pathParts[len(pathParts)-1]

			// Add selections for folder levels
			for i := 0; i < len(jobParts); i++ {
				var viewPath string
				if i == 0 {
					viewPath = "jobs"
				} else {
					viewPath = "jobs/" + strings.Join(jobParts[:i], "/")
				}
				selections[viewPath] = jobParts[i]
			}

			// Add selection for builds view
			jobPath := strings.Join(jobParts, "/")
			selections["builds/"+jobPath] = "#" + buildNum
		}
	case "jobs":
		// For jobs/folder/subfolder, derive selections
		pathParts := strings.Split(subPath, "/")
		for i := 0; i < len(pathParts); i++ {
			var viewPath string
			if i == 0 {
				viewPath = "jobs"
			} else {
				viewPath = "jobs/" + strings.Join(pathParts[:i], "/")
			}
			selections[viewPath] = pathParts[i]
		}
	}

	return selections
}

func (c *Command) handleURLCommand(cmd string) {
	// Extract URL from command: "url <jenkins-url>"
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) < 2 || parts[1] == "" {
		c.app.Flash().Warn("Usage: url <jenkins-url>")
		return
	}

	jenkinsURL := strings.TrimSpace(parts[1])
	viewPath, err := ParseJenkinsURL(jenkinsURL, c.app.Config())
	if err != nil {
		c.app.Flash().Err(fmt.Errorf("invalid URL: %w", err))
		return
	}

	// Navigate to the parsed view
	c.navigateToPath(viewPath)
}

// navigateToPath navigates to a view based on a path like "jobs/folder/subfolder" or "builds/jobname"
// This clears the content stack before pushing the new view.
func (c *Command) navigateToPath(path string) {
	view := c.createViewFromPath(path)
	if view == nil {
		return
	}
	c.app.Content.Clear()
	c.app.Content.Push(view)
}

// navigateToBookmark navigates to a bookmark path without clearing the stack.
// This allows the user to press Escape to go back to the previous view.
// It uses saved bookmark selections to restore cursor positions.
func (c *Command) navigateToBookmark(path string) {
	view := c.createViewFromPath(path)
	if view == nil {
		return
	}

	// Get saved selections from config
	selections := c.app.Config().GetBookmarkSelections()

	// Apply selection to the base view (jobs view) using saved selections
	if baseView := c.app.Content.Top(); baseView != nil {
		if selectable, ok := baseView.(Selectable); ok {
			// Get the view path of the base view
			var baseViewPath string
			if urlProvider, ok := baseView.(URLProvider); ok {
				baseViewPath = urlProvider.GetViewPath()
			}
			// Look up the saved selection for this view
			if baseViewPath != "" && selections != nil {
				if selectedID, ok := selections[baseViewPath]; ok {
					selectable.SelectByID(selectedID)
				}
			}
		}
	}

	c.app.Content.Push(view)
}

// navigateToBookmarkWithStack builds the full view stack for a bookmark.
// For example, bookmark "builds/folder/subfolder/my-job" will create:
// 1. jobs (root jobs view with "folder" selected)
// 2. jobs/folder (with "subfolder" selected)
// 3. jobs/folder/subfolder (with "my-job" selected)
// 4. builds/folder/subfolder/my-job (the bookmark target)
func (c *Command) navigateToBookmarkWithStack(path string) {
	// Get saved selections from config
	selections := c.app.Config().GetBookmarkSelections()

	// Parse the bookmark path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		c.app.Flash().Warn("Invalid bookmark path")
		return
	}

	resource := parts[0]
	var subPath string
	if len(parts) > 1 {
		subPath = parts[1]
	}

	// Build the list of folder paths to create views for
	// For builds/folder/subfolder/job-name, we need: "", "folder", "folder/subfolder"
	var folderPaths []string
	var buildsViewPath string // For logs, we also need to push a builds view

	switch resource {
	case "builds":
		// For builds/folder/subfolder/job-name
		// The job path is folder/subfolder/job-name
		// We need folder views for: "", "folder", "folder/subfolder"
		if subPath != "" {
			// Split the job path to get folder components
			jobParts := strings.Split(subPath, "/")
			// All but the last part are folders
			if len(jobParts) > 1 {
				folderPaths = append(folderPaths, "") // root
				for i := 0; i < len(jobParts)-1; i++ {
					folderPaths = append(folderPaths, strings.Join(jobParts[:i+1], "/"))
				}
			} else {
				// Just job name, only root folder
				folderPaths = append(folderPaths, "")
			}
		} else {
			folderPaths = append(folderPaths, "")
		}
	case "logs":
		// For logs/folder/subfolder/job-name/123
		// subPath is "folder/subfolder/job-name/123"
		// We need folder views for: "", "folder", "folder/subfolder"
		// Then builds view for: folder/subfolder/job-name
		if subPath != "" {
			pathParts := strings.Split(subPath, "/")
			if len(pathParts) >= 2 {
				// All but the last part is the job path
				jobParts := pathParts[:len(pathParts)-1]

				// Build folder paths (all but the job name itself)
				folderPaths = append(folderPaths, "") // root
				for i := 0; i < len(jobParts)-1; i++ {
					folderPaths = append(folderPaths, strings.Join(jobParts[:i+1], "/"))
				}

				// Set the builds view path
				buildsViewPath = strings.Join(jobParts, "/")
			}
		}
		if len(folderPaths) == 0 {
			folderPaths = append(folderPaths, "")
		}
	case "jobs":
		// For jobs/folder/subfolder, we need: "", "folder"
		folderPaths = append(folderPaths, "") // root
		if subPath != "" {
			pathParts := strings.Split(subPath, "/")
			for i := 0; i < len(pathParts)-1; i++ {
				folderPaths = append(folderPaths, strings.Join(pathParts[:i+1], "/"))
			}
		}
	default:
		// For other resources, just push root jobs
		folderPaths = append(folderPaths, "")
	}

	// Create and push all the folder views
	for _, folderPath := range folderPaths {
		var view ResourceViewer
		if folderPath != "" {
			view = NewJobsViewWithPath(c.app, folderPath)
		} else {
			view = NewJobsView(c.app)
		}
		c.app.Content.Push(view)

		// Apply saved selection to this view
		if selections != nil {
			viewPath := view.(URLProvider).GetViewPath()
			if selectedID, ok := selections[viewPath]; ok {
				if selectable, ok := view.(Selectable); ok {
					selectable.SelectByID(selectedID)
				}
			}
		}
	}

	// For logs bookmarks, we also need to push the builds view
	if buildsViewPath != "" {
		buildsView := NewBuildsView(c.app, buildsViewPath)
		c.app.Content.Push(buildsView)

		// Apply saved selection to builds view
		if selections != nil {
			viewPath := buildsView.GetViewPath()
			if selectedID, ok := selections[viewPath]; ok {
				buildsView.SelectByID(selectedID)
			}
		}
	}

	// Create and push the bookmark target view
	targetView := c.createViewFromPath(path)
	if targetView != nil {
		c.app.Content.Push(targetView)
	}
}

// createViewFromPath creates a view based on a path like "jobs/folder/subfolder" or "builds/jobname"
func (c *Command) createViewFromPath(path string) ResourceViewer {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		c.app.Flash().Warn("Invalid path")
		return nil
	}

	resource := parts[0]
	var subPath string
	if len(parts) > 1 {
		subPath = parts[1]
	}

	var view ResourceViewer
	switch resource {
	case "jobs":
		if subPath != "" {
			view = NewJobsViewWithPath(c.app, subPath)
		} else {
			view = NewJobsView(c.app)
		}
	case "builds":
		view = NewBuildsView(c.app, subPath)
	case "logs":
		// logs/job-name/build-number
		if subPath != "" {
			// Find the last "/" to separate job name from build number
			if idx := strings.LastIndex(subPath, "/"); idx >= 0 {
				jobName := subPath[:idx]
				buildNumStr := subPath[idx+1:]
				if buildNum, err := strconv.Atoi(buildNumStr); err == nil {
					view = NewLogsView(c.app, jobName, buildNum)
				}
			}
		}
		if view == nil {
			c.app.Flash().Warn("Invalid logs path")
			return nil
		}
	case "tests":
		// tests/<jobPath>/<buildNum> — same shape as logs/.
		if subPath != "" {
			if idx := strings.LastIndex(subPath, "/"); idx >= 0 {
				jobName := subPath[:idx]
				if buildNum, err := strconv.Atoi(subPath[idx+1:]); err == nil {
					view = NewTestSuitesView(c.app, jobName, buildNum)
				}
			}
		}
		if view == nil {
			c.app.Flash().Warn("Invalid tests path; expected tests/<jobPath>/<buildNum>")
			return nil
		}
	case "reports":
		// reports/<jobPath>/<buildNum> — same shape as logs/.
		if subPath != "" {
			if idx := strings.LastIndex(subPath, "/"); idx >= 0 {
				jobName := subPath[:idx]
				if buildNum, err := strconv.Atoi(subPath[idx+1:]); err == nil {
					view = NewHTMLReportsView(c.app, jobName, buildNum)
				}
			}
		}
		if view == nil {
			c.app.Flash().Warn("Invalid reports path; expected reports/<jobPath>/<buildNum>")
			return nil
		}
	case "pipeline":
		// pipeline/<jobPath>/<buildNum> — same shape as logs/.
		if subPath != "" {
			if idx := strings.LastIndex(subPath, "/"); idx >= 0 {
				jobName := subPath[:idx]
				if buildNum, err := strconv.Atoi(subPath[idx+1:]); err == nil {
					view = NewPipelineGraphView(c.app, jobName, buildNum)
				}
			}
		}
		if view == nil {
			c.app.Flash().Warn("Invalid pipeline path; expected pipeline/<jobPath>/<buildNum>")
			return nil
		}
	case "views":
		if subPath != "" {
			view = NewViewsViewWithPath(c.app, subPath)
		} else {
			view = NewViewsView(c.app)
		}
	default:
		// Try as a direct resource
		c.gotoResource(resource)
		return nil
	}

	return view
}

func (c *Command) gotoResource(res string) {
	var view ResourceViewer

	switch res {
	case "jobs":
		view = NewJobsView(c.app)
	case "builds":
		c.app.Flash().Warn("Usage: builds <job-name>")
		return
	case "queue":
		view = NewQueueView(c.app)
	case "nodes":
		view = NewNodesView(c.app)
	case "users":
		view = NewUsersView(c.app)
	case "credentials":
		view = NewCredentialsView(c.app)
	case "plugins":
		view = NewPluginsView(c.app)
	case "views":
		// Check if we're currently in a folder context
		folderPath := c.getCurrentFolderPath()
		if folderPath != "" {
			view = NewViewsViewWithPath(c.app, folderPath)
		} else {
			view = NewViewsView(c.app)
		}
	case "contexts":
		view = NewContextsView(c.app)
	default:
		c.app.Flash().Warn("Unknown resource: " + res)
		return
	}

	c.app.Content.Clear()
	c.app.Content.Push(view)
}

// matchResourcePath detects an inline `<resource> <path>` invocation for
// path-taking commands (jobs, builds, logs, views and their aliases).
// Returns the canonical resource name, the path, and true on match.
func (c *Command) matchResourcePath(cmd string) (string, string, bool) {
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	head := parts[0]
	path := strings.TrimSpace(parts[1])
	if path == "" {
		return "", "", false
	}
	switch head {
	case "jobs", "job", "j":
		return "jobs", path, true
	case "builds", "build", "b":
		return "builds", path, true
	case "logs", "log", "l":
		return "logs", path, true
	case "views", "view", "v":
		return "views", path, true
	case "tests", "test":
		return "tests", path, true
	case "reports", "report":
		return "reports", path, true
	case "pipeline", "pipe", "pl", "bo":
		return "pipeline", path, true
	}
	return "", "", false
}

// navigatePromptPath pushes the view referenced by a "<resource>/<path>"
// expression onto the content stack. Failures inside createViewFromPath
// already flash a warning to the user.
func (c *Command) navigatePromptPath(path string) {
	view := c.createViewFromPath(path)
	if view == nil {
		return
	}
	c.app.Content.Push(view)
}

// matchCtxArg detects an inline `ctx <name>` invocation. Returns the
// requested context name and true on match.
func (c *Command) matchCtxArg(cmd string) (string, bool) {
	parts := strings.Fields(cmd)
	if len(parts) != 2 {
		return "", false
	}
	switch parts[0] {
	case "ctx", "context", "contexts":
		return parts[1], true
	}
	return "", false
}

// switchContext switches the active Jenkins context by name and pushes a
// fresh JobsView, mirroring ContextsView.switchCmd.
func (c *Command) switchContext(name string) {
	if err := c.app.SwitchContext(name); err != nil {
		c.app.Flash().Err(err)
		return
	}
	c.app.Flash().Info("Switched to context: " + name)
	c.app.Content.Clear()
	c.app.Content.Push(NewJobsView(c.app))
}

// Suggest returns command suggestions for autocomplete. If the input has
// no space, suggestions are command names. Once the user types a space the
// source switches to argument values: contexts from config for `ctx`,
// IDs of the current top view for resource navigation commands.
func (c *Command) Suggest(prefix string) []string {
	if prefix == "" {
		return nil
	}

	lower := strings.ToLower(prefix)

	// Argument-aware completion: split on the first space.
	if idx := strings.Index(lower, " "); idx >= 0 {
		cmd := lower[:idx]
		arg := strings.TrimLeft(lower[idx+1:], " ")
		values := c.argumentValues(cmd)
		if len(values) == 0 {
			return nil
		}
		valuePrefix := c.argumentValuePrefix(cmd)
		return formatArgumentSuggestions(prefix[:idx+1], valuePrefix, arg, values)
	}

	// First-token completion (existing behavior).
	allCommands := make([]string, 0)
	for _, a := range defaultAliases {
		allCommands = append(allCommands, a.Resource)
		allCommands = append(allCommands, a.Aliases...)
	}
	for cmd := range quitCommands {
		allCommands = append(allCommands, cmd)
	}
	for cmd := range helpCommands {
		allCommands = append(allCommands, cmd)
	}
	for cmd := range cacheCommands {
		allCommands = append(allCommands, cmd)
	}

	suggestions := make([]string, 0)
	for _, cmd := range allCommands {
		if strings.HasPrefix(cmd, lower) && cmd != lower {
			suggestions = append(suggestions, cmd)
		}
	}

	sort.Strings(suggestions)
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}
	return suggestions
}

// argumentValues returns the candidate argument values for a given command
// token. ctx variants source from the configured Jenkins contexts; the
// resource navigation commands source from the current top view's IDs.
func (c *Command) argumentValues(cmd string) []string {
	switch cmd {
	case "ctx", "context", "contexts":
		cfg := c.app.Config()
		if cfg == nil || cfg.J9s == nil {
			return nil
		}
		out := make([]string, 0, len(cfg.J9s.Contexts))
		for _, ctx := range cfg.J9s.Contexts {
			out = append(out, ctx.Name)
		}
		return out

	case "jobs", "job", "j",
		"builds", "build", "b",
		"logs", "log", "l",
		"views", "view", "v":
		top := c.app.Content.Top()
		if top == nil {
			return nil
		}
		if p, ok := top.(IDProvider); ok {
			return p.IDs()
		}
		return nil
	}
	return nil
}

// argumentValuePrefix returns the path prefix to apply to each argument
// value before suggesting it. For resource navigation commands on a view
// that lives inside a folder hierarchy (PathProvider) the suggestions are
// qualified with that folder path, e.g. "team-a/sub/deploy" instead of a
// bare "deploy". ctx and other commands have no path prefix.
func (c *Command) argumentValuePrefix(cmd string) string {
	switch cmd {
	case "jobs", "job", "j",
		"builds", "build", "b",
		"logs", "log", "l",
		"views", "view", "v":
		top := c.app.Content.Top()
		if top == nil {
			return ""
		}
		if p, ok := top.(PathProvider); ok {
			path := p.CurrentPath()
			if path != "" {
				return path + "/"
			}
		}
	}
	return ""
}

// formatArgumentSuggestions filters values by argPrefix, prepends the
// (optional) value-side path prefix, formats each as
// "<prefixCmd><valuePrefix><value>" so the prompt's HasPrefix-based
// ghost-text logic shows just the completion tail, sorts, and caps the
// list. If argPrefix already starts with the valuePrefix the user is
// typing the qualified path; otherwise we treat argPrefix as a leaf-only
// prefix so a bare `builds d` still matches `team-a/sub/deploy`.
func formatArgumentSuggestions(prefixCmd, valuePrefix, argPrefix string, values []string) []string {
	lowerVP := strings.ToLower(valuePrefix)
	leafPrefix := argPrefix
	if valuePrefix != "" && strings.HasPrefix(argPrefix, lowerVP) {
		leafPrefix = strings.TrimPrefix(argPrefix, lowerVP)
	}

	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if leafPrefix != "" && !strings.HasPrefix(strings.ToLower(v), leafPrefix) {
			continue
		}
		out = append(out, prefixCmd+valuePrefix+v)
	}
	sort.Strings(out)
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}
