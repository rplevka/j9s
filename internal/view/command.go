// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package view

import (
	"fmt"
	"sort"
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
		c.app.Flash().Info("Commands: jobs, builds, queue, nodes, users, credentials, plugins, views, contexts, cache [clear|stats] | :q to quit")
		return
	}

	// Check for cache commands
	if strings.HasPrefix(cmd, "cache") {
		c.handleCacheCommand(cmd)
		return
	}

	// Check for direct resource match
	if res, ok := c.aliases[cmd]; ok {
		c.gotoResource(res)
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

func (c *Command) gotoResource(res string) {
	var view ResourceViewer

	switch res {
	case "jobs":
		view = NewJobsView(c.app)
	case "builds":
		view = NewBuildsView(c.app, "")
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
		view = NewViewsView(c.app)
	case "contexts":
		view = NewContextsView(c.app)
	default:
		c.app.Flash().Warn("Unknown resource: " + res)
		return
	}

	c.app.Content.Clear()
	c.app.Content.Push(view)
}

// Suggest returns command suggestions for autocomplete.
func (c *Command) Suggest(prefix string) []string {
	if prefix == "" {
		return nil
	}

	prefix = strings.ToLower(prefix)

	// Collect all possible commands
	allCommands := make([]string, 0)

	// Add resource commands
	for _, a := range defaultAliases {
		allCommands = append(allCommands, a.Resource)
		allCommands = append(allCommands, a.Aliases...)
	}

	// Add special commands
	for cmd := range quitCommands {
		allCommands = append(allCommands, cmd)
	}
	for cmd := range helpCommands {
		allCommands = append(allCommands, cmd)
	}
	for cmd := range cacheCommands {
		allCommands = append(allCommands, cmd)
	}

	// Find matching commands that start with prefix
	suggestions := make([]string, 0)
	for _, cmd := range allCommands {
		if strings.HasPrefix(cmd, prefix) && cmd != prefix {
			suggestions = append(suggestions, cmd)
		}
	}

	// Sort and dedupe
	sort.Strings(suggestions)
	if len(suggestions) > 5 {
		suggestions = suggestions[:5] // Limit to 5 suggestions
	}

	return suggestions
}
