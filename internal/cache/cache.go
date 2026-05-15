// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package cache

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rplevka/j9s/internal/config"
)

// BuildMeta represents cached build metadata.
type BuildMeta struct {
	JobName     string    `json:"jobName"`
	BuildNumber int       `json:"buildNumber"`
	Result      string    `json:"result"`
	Building    bool      `json:"building"`
	Timestamp   int64     `json:"timestamp"`
	Duration    int64     `json:"duration"`
	CachedAt    time.Time `json:"cachedAt"`
}

// Cache manages build logs and metadata caching.
type Cache struct {
	dir           string
	retentionDays int
	maxSizeMB     int
	enabled       bool
}

// New creates a new cache instance.
func New(cfg config.CacheConfig) (*Cache, error) {
	dir := config.CacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	c := &Cache{
		dir:           dir,
		retentionDays: cfg.RetentionDays,
		maxSizeMB:     cfg.MaxSizeMB,
		enabled:       cfg.Enabled,
	}

	if c.retentionDays <= 0 {
		c.retentionDays = 7
	}
	if c.maxSizeMB <= 0 {
		c.maxSizeMB = 500
	}

	return c, nil
}

// IsEnabled returns true if caching is enabled.
func (c *Cache) IsEnabled() bool {
	return c.enabled
}

// cacheKey generates a unique key for a build.
func cacheKey(contextName, jobName string, buildNumber int) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s/%s/%d", contextName, jobName, buildNumber)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// logPath returns the path to the cached log file.
func (c *Cache) logPath(contextName, jobName string, buildNumber int) string {
	key := cacheKey(contextName, jobName, buildNumber)
	return filepath.Join(c.dir, key+".log.gz")
}

// metaPath returns the path to the cached metadata file.
func (c *Cache) metaPath(contextName, jobName string, buildNumber int) string {
	key := cacheKey(contextName, jobName, buildNumber)
	return filepath.Join(c.dir, key+".meta.json")
}

// HasLog checks if a log is cached.
func (c *Cache) HasLog(contextName, jobName string, buildNumber int) bool {
	if !c.enabled {
		return false
	}
	_, err := os.Stat(c.logPath(contextName, jobName, buildNumber))
	return err == nil
}

// GetLog retrieves a cached log.
func (c *Cache) GetLog(contextName, jobName string, buildNumber int) (string, error) {
	if !c.enabled {
		return "", fmt.Errorf("cache disabled")
	}

	path := c.logPath(contextName, jobName, buildNumber)
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	data, err := io.ReadAll(gz)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// PutLog stores a log in the cache.
func (c *Cache) PutLog(contextName, jobName string, buildNumber int, log string) error {
	if !c.enabled {
		return nil
	}

	path := c.logPath(contextName, jobName, buildNumber)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	_, err = gz.Write([]byte(log))
	return err
}

// GetMeta retrieves cached build metadata.
func (c *Cache) GetMeta(contextName, jobName string, buildNumber int) (*BuildMeta, error) {
	if !c.enabled {
		return nil, fmt.Errorf("cache disabled")
	}

	path := c.metaPath(contextName, jobName, buildNumber)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var meta BuildMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// PutMeta stores build metadata in the cache.
func (c *Cache) PutMeta(contextName string, meta *BuildMeta) error {
	if !c.enabled {
		return nil
	}

	meta.CachedAt = time.Now()
	path := c.metaPath(contextName, meta.JobName, meta.BuildNumber)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// IsBuildComplete checks if a build is complete (not building).
func (c *Cache) IsBuildComplete(contextName, jobName string, buildNumber int) bool {
	meta, err := c.GetMeta(contextName, jobName, buildNumber)
	if err != nil {
		return false
	}
	return !meta.Building && meta.Result != ""
}

// Clear removes all cached data.
func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := os.Remove(filepath.Join(c.dir, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

// ClearJob removes cached data for a specific job.
func (c *Cache) ClearJob(contextName, jobName string) error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	// We need to check each meta file to find matching jobs
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(c.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var meta BuildMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		if meta.JobName == jobName {
			// Remove both meta and log files
			os.Remove(path)
			logPath := path[:len(path)-10] + ".log.gz" // Replace .meta.json with .log.gz
			os.Remove(logPath)
		}
	}

	return nil
}

// Cleanup removes expired cache entries.
func (c *Cache) Cleanup() error {
	if !c.enabled {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -c.retentionDays)

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(c.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var meta BuildMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		if meta.CachedAt.Before(cutoff) {
			os.Remove(path)
			logPath := path[:len(path)-10] + ".log.gz"
			os.Remove(logPath)
		}
	}

	// Check total size and remove oldest if over limit
	return c.enforceMaxSize()
}

// enforceMaxSize removes oldest entries if cache exceeds max size.
func (c *Cache) enforceMaxSize() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	type fileInfo struct {
		path    string
		size    int64
		modTime time.Time
	}

	var files []fileInfo
	var totalSize int64

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(c.dir, entry.Name())
		files = append(files, fileInfo{path: path, size: info.Size(), modTime: info.ModTime()})
		totalSize += info.Size()
	}

	maxBytes := int64(c.maxSizeMB) * 1024 * 1024
	if totalSize <= maxBytes {
		return nil
	}

	// Sort by modification time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	// Remove oldest files until under limit
	for _, f := range files {
		if totalSize <= maxBytes {
			break
		}
		os.Remove(f.path)
		totalSize -= f.size
	}

	return nil
}

// Stats returns cache statistics.
func (c *Cache) Stats() (int, int64, error) {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return 0, 0, err
	}

	var count int
	var totalSize int64

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			count++
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	return count, totalSize, nil
}
