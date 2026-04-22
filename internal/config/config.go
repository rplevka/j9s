// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// AppName is the application name.
	AppName = "j9s"

	// DefaultRefreshRate is the default refresh rate.
	DefaultRefreshRate = 2

	// DefaultLogLevel is the default log level.
	DefaultLogLevel = "info"

	// DefaultCommand is the default command to run.
	DefaultCommand = "jobs"
)

var (
	// AppConfigDir is the application config directory.
	AppConfigDir string

	// AppConfigFile is the application config file.
	AppConfigFile string

	// AppLogFile is the application log file.
	AppLogFile string
)

// InitLocs initializes the application locations.
func InitLocs() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	AppConfigDir = filepath.Join(home, ".j9s")
	AppConfigFile = filepath.Join(AppConfigDir, "config.yaml")
	AppLogFile = filepath.Join(AppConfigDir, "j9s.log")

	if err := os.MkdirAll(AppConfigDir, 0755); err != nil {
		return err
	}

	return nil
}

// AuthType represents the authentication type.
type AuthType string

const (
	// AuthTypeToken is the token authentication type.
	AuthTypeToken AuthType = "token"

	// AuthTypePassword is the password authentication type.
	AuthTypePassword AuthType = "password"

	// AuthTypeOAuth is the OAuth authentication type.
	AuthTypeOAuth AuthType = "oauth"
)

// Context represents a Jenkins context.
type Context struct {
	// Name is the context name.
	Name string `yaml:"name"`

	// URL is the Jenkins URL.
	URL string `yaml:"url"`

	// Auth is the authentication configuration.
	Auth AuthConfig `yaml:"auth"`

	// Insecure skips TLS verification.
	Insecure bool `yaml:"insecure,omitempty"`

	// CACert is the path to the CA certificate.
	CACert string `yaml:"caCert,omitempty"`

	// Bookmark is the landing view path for this context (e.g., "builds/my-job").
	Bookmark string `yaml:"bookmark,omitempty"`

	// BookmarkSelections stores what item was selected at each view level.
	// Key is the view path (e.g., "jobs"), value is the selected item ID.
	// This allows proper cursor restoration when navigating back.
	BookmarkSelections map[string]string `yaml:"bookmarkSelections,omitempty"`
}

// AuthConfig represents the authentication configuration.
type AuthConfig struct {
	// Type is the authentication type.
	Type AuthType `yaml:"type"`

	// Username is the username for token/password auth.
	Username string `yaml:"username,omitempty"`

	// Token is the API token for token auth.
	Token string `yaml:"token,omitempty"`

	// Password is the password for password auth.
	Password string `yaml:"password,omitempty"`

	// OAuth configuration for OAuth auth.
	OAuth *OAuthConfig `yaml:"oauth,omitempty"`
}

// OAuthConfig represents OAuth configuration.
type OAuthConfig struct {
	// ClientID is the OAuth client ID.
	ClientID string `yaml:"clientId"`

	// ClientSecret is the OAuth client secret.
	ClientSecret string `yaml:"clientSecret,omitempty"`

	// TokenURL is the OAuth token URL.
	TokenURL string `yaml:"tokenUrl"`

	// AuthURL is the OAuth authorization URL.
	AuthURL string `yaml:"authUrl,omitempty"`

	// Scopes are the OAuth scopes.
	Scopes []string `yaml:"scopes,omitempty"`

	// AccessToken is the cached access token.
	AccessToken string `yaml:"accessToken,omitempty"`

	// RefreshToken is the cached refresh token.
	RefreshToken string `yaml:"refreshToken,omitempty"`
}

// CacheConfig represents cache configuration.
type CacheConfig struct {
	// Enabled enables/disables caching.
	Enabled bool `yaml:"enabled,omitempty"`

	// RetentionDays is the number of days to retain cached data.
	RetentionDays int `yaml:"retentionDays,omitempty"`

	// MaxSizeMB is the maximum cache size in megabytes.
	MaxSizeMB int `yaml:"maxSizeMB,omitempty"`
}

// J9s represents the j9s configuration.
type J9s struct {
	// CurrentContext is the current context name.
	CurrentContext string `yaml:"currentContext"`

	// Contexts is the list of contexts.
	Contexts []Context `yaml:"contexts"`

	// RefreshRate is the refresh rate in seconds.
	RefreshRate float32 `yaml:"refreshRate,omitempty"`

	// LogLevel is the log level.
	LogLevel string `yaml:"logLevel,omitempty"`

	// Headless turns off the header.
	Headless bool `yaml:"headless,omitempty"`

	// Logoless turns off the logo.
	Logoless bool `yaml:"logoless,omitempty"`

	// ReadOnly sets read-only mode.
	ReadOnly bool `yaml:"readOnly,omitempty"`

	// DefaultView is the default view to load.
	DefaultView string `yaml:"defaultView,omitempty"`

	// Cache is the cache configuration.
	Cache CacheConfig `yaml:"cache,omitempty"`
}

// Config represents the application configuration.
type Config struct {
	J9s  *J9s   `yaml:"j9s"`
	path string `yaml:"-"` // Config file path (not serialized)
}

// NewConfig returns a new configuration.
func NewConfig() *Config {
	return &Config{
		J9s: &J9s{
			RefreshRate: DefaultRefreshRate,
			LogLevel:    DefaultLogLevel,
			Contexts:    make([]Context, 0),
			Cache: CacheConfig{
				Enabled:       true,
				RetentionDays: 7,
				MaxSizeMB:     500,
			},
		},
	}
}

// CacheDir returns the cache directory path.
func CacheDir() string {
	return filepath.Join(AppConfigDir, "cache")
}

// Load loads the configuration from the given file.
func (c *Config) Load(path string) error {
	c.path = path // Store path for later saves

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c.Save(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if c.J9s == nil {
		c.J9s = &J9s{
			RefreshRate: DefaultRefreshRate,
			LogLevel:    DefaultLogLevel,
			Contexts:    make([]Context, 0),
		}
	}

	return nil
}

// SaveConfig saves the configuration to the stored path.
func (c *Config) SaveConfig() error {
	if c.path == "" {
		return fmt.Errorf("config path not set")
	}
	return c.Save(c.path)
}

// Save saves the configuration to the given file.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Override overrides the configuration with the given flags.
func (c *Config) Override(flags *Flags) {
	if flags == nil {
		return
	}

	if flags.RefreshRate != nil && *flags.RefreshRate != DefaultRefreshRate {
		c.J9s.RefreshRate = *flags.RefreshRate
	}
	if flags.LogLevel != nil && *flags.LogLevel != DefaultLogLevel {
		c.J9s.LogLevel = *flags.LogLevel
	}
	if flags.Headless != nil && *flags.Headless {
		c.J9s.Headless = *flags.Headless
	}
	if flags.Logoless != nil && *flags.Logoless {
		c.J9s.Logoless = *flags.Logoless
	}
	if flags.ReadOnly != nil && *flags.ReadOnly {
		c.J9s.ReadOnly = *flags.ReadOnly
	}
	if flags.Context != nil && *flags.Context != "" {
		c.J9s.CurrentContext = *flags.Context
	}
	if flags.Command != nil && *flags.Command != DefaultCommand {
		c.J9s.DefaultView = *flags.Command
	}
}

// ActiveContext returns the active context.
func (c *Config) ActiveContext() (*Context, error) {
	if c.J9s == nil || len(c.J9s.Contexts) == 0 {
		return nil, errors.New("no contexts configured")
	}

	name := c.J9s.CurrentContext
	if name == "" && len(c.J9s.Contexts) > 0 {
		return &c.J9s.Contexts[0], nil
	}

	for i := range c.J9s.Contexts {
		if c.J9s.Contexts[i].Name == name {
			return &c.J9s.Contexts[i], nil
		}
	}

	return nil, fmt.Errorf("context %q not found", name)
}

// SetActiveContext sets the active context.
func (c *Config) SetActiveContext(name string) error {
	for _, ctx := range c.J9s.Contexts {
		if ctx.Name == name {
			c.J9s.CurrentContext = name
			return nil
		}
	}
	return fmt.Errorf("context %q not found", name)
}

// ContextNames returns the list of context names.
func (c *Config) ContextNames() []string {
	names := make([]string, 0, len(c.J9s.Contexts))
	for _, ctx := range c.J9s.Contexts {
		names = append(names, ctx.Name)
	}
	return names
}

// SetBookmark sets the bookmark for the active context.
func (c *Config) SetBookmark(bookmark string) error {
	for i, ctx := range c.J9s.Contexts {
		if ctx.Name == c.J9s.CurrentContext {
			c.J9s.Contexts[i].Bookmark = bookmark
			return c.SaveConfig()
		}
	}
	return fmt.Errorf("active context %q not found", c.J9s.CurrentContext)
}

// ClearBookmark clears the bookmark for the active context.
func (c *Config) ClearBookmark() error {
	return c.SetBookmark("")
}

// GetBookmark returns the bookmark for the active context.
func (c *Config) GetBookmark() string {
	for _, ctx := range c.J9s.Contexts {
		if ctx.Name == c.J9s.CurrentContext {
			return ctx.Bookmark
		}
	}
	return ""
}

// SetBookmarkSelections sets the bookmark selections for the active context.
func (c *Config) SetBookmarkSelections(selections map[string]string) error {
	for i, ctx := range c.J9s.Contexts {
		if ctx.Name == c.J9s.CurrentContext {
			c.J9s.Contexts[i].BookmarkSelections = selections
			return c.SaveConfig()
		}
	}
	return fmt.Errorf("active context %q not found", c.J9s.CurrentContext)
}

// GetBookmarkSelections returns the bookmark selections for the active context.
func (c *Config) GetBookmarkSelections() map[string]string {
	for _, ctx := range c.J9s.Contexts {
		if ctx.Name == c.J9s.CurrentContext {
			return ctx.BookmarkSelections
		}
	}
	return nil
}

// SetBookmarkWithSelections sets both bookmark path and selections atomically.
func (c *Config) SetBookmarkWithSelections(bookmark string, selections map[string]string) error {
	for i, ctx := range c.J9s.Contexts {
		if ctx.Name == c.J9s.CurrentContext {
			c.J9s.Contexts[i].Bookmark = bookmark
			c.J9s.Contexts[i].BookmarkSelections = selections
			return c.SaveConfig()
		}
	}
	return fmt.Errorf("active context %q not found", c.J9s.CurrentContext)
}
