package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/types"
)

const (
	// Default cache duration - 24 hours
	DefaultCacheDuration = 24 * time.Hour
	// Cache directory name
	CacheDirName = ".go-unmaintained-cache"
)

// CacheEntry represents a cached repository analysis result
type CacheEntry struct {
	RepoInfo  *types.RepoInfo `json:"repo_info"`
	Timestamp time.Time        `json:"timestamp"`
	Version   string           `json:"latest_version,omitempty"`
}

// Cache manages persistent caching of repository information
type Cache struct {
	cacheDir string
	duration time.Duration
	disabled bool
}

// NewCache creates a new cache instance
func NewCache(disabled bool, duration time.Duration) (*Cache, error) {
	if disabled {
		return &Cache{disabled: true}, nil
	}

	if duration == 0 {
		duration = DefaultCacheDuration
	}

	// Get user cache directory
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache directory: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		cacheDir: cacheDir,
		duration: duration,
		disabled: false,
	}, nil
}

// GetRepoInfo retrieves cached repository information
func (c *Cache) GetRepoInfo(owner, repo string) (*types.RepoInfo, string, bool) {
	if c.disabled {
		return nil, "", false
	}

	cacheKey := fmt.Sprintf("%s_%s", owner, repo)
	filePath := c.getCacheFilePath(cacheKey)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Invalid cache entry, ignore
		return nil, "", false
	}

	// Check if cache entry is still valid
	if time.Since(entry.Timestamp) > c.duration {
		// Cache expired
		os.Remove(filePath) // Clean up expired entry
		return nil, "", false
	}

	return entry.RepoInfo, entry.Version, true
}

// SetRepoInfo stores repository information in cache
func (c *Cache) SetRepoInfo(owner, repo string, repoInfo *types.RepoInfo, latestVersion string) error {
	if c.disabled {
		return nil
	}

	entry := CacheEntry{
		RepoInfo:  repoInfo,
		Timestamp: time.Now(),
		Version:   latestVersion,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	cacheKey := fmt.Sprintf("%s_%s", owner, repo)
	filePath := c.getCacheFilePath(cacheKey)

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache entry: %w", err)
	}

	return nil
}

// Clear removes all cached entries
func (c *Cache) Clear() error {
	if c.disabled {
		return nil
	}

	return os.RemoveAll(c.cacheDir)
}

// CleanExpired removes expired cache entries
func (c *Cache) CleanExpired() error {
	if c.disabled {
		return nil
	}

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(c.cacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Remove files older than cache duration
		if time.Since(info.ModTime()) > c.duration {
			os.Remove(filePath)
		}
	}

	return nil
}

// getCacheDir returns the appropriate cache directory for the current user
func getCacheDir() (string, error) {
	// Try XDG_CACHE_HOME first (Linux/Unix standard)
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, CacheDirName), nil
	}

	// Try user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// On macOS, use ~/Library/Caches
	if runtime := os.Getenv("GOOS"); runtime == "darwin" {
		return filepath.Join(homeDir, "Library", "Caches", CacheDirName), nil
	}

	// Default to ~/.cache on Unix-like systems
	return filepath.Join(homeDir, ".cache", CacheDirName), nil
}

// getCacheFilePath returns the full path for a cache file
func (c *Cache) getCacheFilePath(key string) string {
	// Hash the key to create a safe filename
	hash := sha256.Sum256([]byte(key))
	filename := fmt.Sprintf("%x.json", hash)
	return filepath.Join(c.cacheDir, filename)
}

// GetStats returns cache statistics
func (c *Cache) GetStats() (int, error) {
	if c.disabled {
		return 0, nil
	}

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			count++
		}
	}

	return count, nil
}
