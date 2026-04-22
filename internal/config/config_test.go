// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetBookmark(t *testing.T) {
	// Create temp dir for test config
	tmpDir, err := os.MkdirTemp("", "j9s-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	tests := []struct {
		name     string
		bookmark string
		wantErr  bool
	}{
		{
			name:     "set jobs bookmark",
			bookmark: "jobs/MyFolder",
			wantErr:  false,
		},
		{
			name:     "set builds bookmark",
			bookmark: "builds/MyJob",
			wantErr:  false,
		},
		{
			name:     "set nested folder bookmark",
			bookmark: "jobs/Folder/SubFolder/DeepFolder",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh config
			cfg := NewConfig()
			cfg.J9s.Contexts = []Context{
				{Name: "test-context", URL: "http://jenkins.example.com"},
			}
			cfg.J9s.CurrentContext = "test-context"

			// Save initial config
			err := cfg.Save(configPath)
			require.NoError(t, err)

			// Load it back (to set the path)
			cfg2 := NewConfig()
			err = cfg2.Load(configPath)
			require.NoError(t, err)

			// Set bookmark
			err = cfg2.SetBookmark(tt.bookmark)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify bookmark was set
			assert.Equal(t, tt.bookmark, cfg2.GetBookmark())

			// Reload and verify persistence
			cfg3 := NewConfig()
			err = cfg3.Load(configPath)
			require.NoError(t, err)
			assert.Equal(t, tt.bookmark, cfg3.GetBookmark())
		})
	}
}

func TestClearBookmark(t *testing.T) {
	// Create temp dir for test config
	tmpDir, err := os.MkdirTemp("", "j9s-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config with bookmark
	cfg := NewConfig()
	cfg.J9s.Contexts = []Context{
		{Name: "test-context", URL: "http://jenkins.example.com", Bookmark: "jobs/MyFolder"},
	}
	cfg.J9s.CurrentContext = "test-context"
	err = cfg.Save(configPath)
	require.NoError(t, err)

	// Load and verify bookmark exists
	cfg2 := NewConfig()
	err = cfg2.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "jobs/MyFolder", cfg2.GetBookmark())

	// Clear bookmark
	err = cfg2.ClearBookmark()
	require.NoError(t, err)

	// Verify bookmark was cleared
	assert.Equal(t, "", cfg2.GetBookmark())

	// Reload and verify persistence
	cfg3 := NewConfig()
	err = cfg3.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "", cfg3.GetBookmark())
}

func TestGetBookmark_NoContext(t *testing.T) {
	cfg := NewConfig()
	cfg.J9s.CurrentContext = "nonexistent"

	// Should return empty string when context doesn't exist
	assert.Equal(t, "", cfg.GetBookmark())
}

func TestSetBookmark_NoContext(t *testing.T) {
	// Create temp dir for test config
	tmpDir, err := os.MkdirTemp("", "j9s-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := NewConfig()
	cfg.J9s.CurrentContext = "nonexistent"
	err = cfg.Save(configPath)
	require.NoError(t, err)

	cfg2 := NewConfig()
	err = cfg2.Load(configPath)
	require.NoError(t, err)

	// Should error when context doesn't exist
	err = cfg2.SetBookmark("jobs/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBookmarkPersistence(t *testing.T) {
	// Create temp dir for test config
	tmpDir, err := os.MkdirTemp("", "j9s-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config with multiple contexts
	cfg := NewConfig()
	cfg.J9s.Contexts = []Context{
		{Name: "prod", URL: "http://prod.jenkins.com", Bookmark: "jobs/Production"},
		{Name: "dev", URL: "http://dev.jenkins.com", Bookmark: "jobs/Development"},
	}
	cfg.J9s.CurrentContext = "prod"
	err = cfg.Save(configPath)
	require.NoError(t, err)

	// Load and verify correct bookmark is returned based on current context
	cfg2 := NewConfig()
	err = cfg2.Load(configPath)
	require.NoError(t, err)

	// Should get prod bookmark
	assert.Equal(t, "jobs/Production", cfg2.GetBookmark())

	// Switch context
	err = cfg2.SetActiveContext("dev")
	require.NoError(t, err)

	// Should get dev bookmark
	assert.Equal(t, "jobs/Development", cfg2.GetBookmark())
}
