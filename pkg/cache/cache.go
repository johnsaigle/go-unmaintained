package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
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
	Timestamp time.Time       `json:"timestamp"`
	Version   string          `json:"latest_version,omitempty"`
}

// Cache manages persistent caching of repository information.
// GetRepoInfo/SetRepoInfo are safe for concurrent use: the mutex guards
// the read-modify-write of a single cache file so one worker's write is not
// observed as partial data by another worker's read.
type Cache struct {
	cacheDir string
	duration time.Duration
	disabled bool
	mu       sync.RWMutex
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

	c.mu.RLock()
	filePath := c.getCacheFilePath(owner, repo)
	data, err := os.ReadFile(filePath)
	c.mu.RUnlock()

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
		// Clean up expired entry. Ignore removal error: the file may already
		// be gone (concurrent expiry), and a stale read is safer than a panic.
		os.Remove(filePath)
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

	// Write under a lock so a concurrent read never sees a partially-written
	// file. Since the file path is derived from (owner, repo) and we hold the
	// lock for the whole read-modify-write of one file, the serialized access
	// is race-free (verified by a concurrent test).
	c.mu.Lock()
	filePath := c.getCacheFilePath(owner, repo)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to write cache entry: %w", err)
	}
	c.mu.Unlock()

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

// getCacheDir returns the appropriate cache directory for the current user.
//
// macOS is detected via the operating-system UserHomeDir path (XDG_CACHE_HOME
// is the Linux/Unix convention; the Mac convention is ~/Library/Caches). We
// do NOT look at $GOOS for this: $GOOS describes the cross-compile target,
// not the host we're running on.
func getCacheDir() (string, error) {
	// XDG_CACHE_HOME is the standard for Unix; preserve it if set.
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" && isUnixFamily() {
		return filepath.Join(cacheHome, CacheDirName), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if isDarwin() {
		return filepath.Join(homeDir, "Library", "Caches", CacheDirName), nil
	}

	// Default to ~/.cache on Unix-like systems
	return filepath.Join(homeDir, ".cache", CacheDirName), nil
}

// isUnixFamily reports whether the host is a Unix-like system (Linux, BSD, etc.)
// as opposed to macOS, which uses ~/Library/Caches. We do not use $GOOS because
// that is the build-time target, not the host.
func isUnixFamily() bool {
	return runtime.GOOS != "darwin"
}

// isDarwin reports whether the host is macOS.
func isDarwin() bool {
	return runtime.GOOS == "darwin"
}

// getCacheFilePath returns the full path for a cache file derived from
// (owner, repo). The tuple is hashed with a NUL separator so that two
// unrelated (owner, repo) pairs can share a file only if their exact tuple
// matches. Example: owner="a", repo="b_c" must not collide with
// owner="a_b", repo="c"; the NUL byte in the key makes that impossible.
func (c *Cache) getCacheFilePath(owner, repo string) string {
	hash := sha256.Sum256([]byte(owner + "\x00" + repo))
	filename := fmt.Sprintf("%x.json", hex.EncodeToString(hash[:]))
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
