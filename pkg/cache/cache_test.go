package cache

import (
	"testing"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/types"
)

func TestNewCache_Disabled(t *testing.T) {
	c, err := NewCache(true, 0)
	if err != nil {
		t.Fatalf("NewCache(disabled) error: %v", err)
	}
	if !c.disabled {
		t.Error("expected cache to be disabled")
	}
}

func TestNewCache_Enabled(t *testing.T) {
	// Use XDG_CACHE_HOME to control the cache directory
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := NewCache(false, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}
	if c.disabled {
		t.Error("expected cache to be enabled")
	}
	if c.duration != 1*time.Hour {
		t.Errorf("duration = %v, want 1h", c.duration)
	}
}

func TestNewCache_DefaultDuration(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := NewCache(false, 0)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}
	if c.duration != DefaultCacheDuration {
		t.Errorf("duration = %v, want %v", c.duration, DefaultCacheDuration)
	}
}

func TestCache_SetAndGetRepoInfo(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := NewCache(false, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}

	repoInfo := &types.RepoInfo{
		Exists:     true,
		IsArchived: false,
		URL:        "https://github.com/user/repo",
	}

	// Set
	err = c.SetRepoInfo("user", "repo", repoInfo, "v1.2.3")
	if err != nil {
		t.Fatalf("SetRepoInfo() error: %v", err)
	}

	// Get
	got, version, hit := c.GetRepoInfo("user", "repo")
	if !hit {
		t.Fatal("expected cache hit")
	}
	if got == nil {
		t.Fatal("expected non-nil RepoInfo")
	}
	if got.URL != "https://github.com/user/repo" {
		t.Errorf("URL = %q, want %q", got.URL, "https://github.com/user/repo")
	}
	if version != "v1.2.3" {
		t.Errorf("version = %q, want %q", version, "v1.2.3")
	}
}

func TestCache_Miss(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := NewCache(false, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}

	_, _, hit := c.GetRepoInfo("nonexistent", "repo")
	if hit {
		t.Error("expected cache miss for nonexistent entry")
	}
}

func TestCache_DisabledOperations(t *testing.T) {
	c, _ := NewCache(true, 0)

	// All operations should be no-ops on disabled cache
	info, version, hit := c.GetRepoInfo("user", "repo")
	if hit || info != nil || version != "" {
		t.Error("disabled cache should always miss")
	}

	err := c.SetRepoInfo("user", "repo", &types.RepoInfo{}, "v1.0.0")
	if err != nil {
		t.Errorf("disabled SetRepoInfo should not error: %v", err)
	}

	err = c.Clear()
	if err != nil {
		t.Errorf("disabled Clear should not error: %v", err)
	}

	err = c.CleanExpired()
	if err != nil {
		t.Errorf("disabled CleanExpired should not error: %v", err)
	}

	count, err := c.GetStats()
	if err != nil {
		t.Errorf("disabled GetStats should not error: %v", err)
	}
	if count != 0 {
		t.Errorf("disabled GetStats count = %d, want 0", count)
	}
}

func TestCache_Clear(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := NewCache(false, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}

	// Add an entry
	err = c.SetRepoInfo("user", "repo", &types.RepoInfo{Exists: true}, "v1.0.0")
	if err != nil {
		t.Fatalf("SetRepoInfo() error: %v", err)
	}

	// Verify it exists
	_, _, hit := c.GetRepoInfo("user", "repo")
	if !hit {
		t.Fatal("expected cache hit before clear")
	}

	// Clear
	err = c.Clear()
	if err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	// Should miss after clear — but we need to recreate since the dir is gone
	c2, err := NewCache(false, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache() after clear error: %v", err)
	}
	_, _, hit = c2.GetRepoInfo("user", "repo")
	if hit {
		t.Error("expected cache miss after clear")
	}
}

func TestCache_GetStats(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := NewCache(false, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}

	// Empty cache
	count, err := c.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if count != 0 {
		t.Errorf("empty cache count = %d, want 0", count)
	}

	// Add entries
	_ = c.SetRepoInfo("user1", "repo1", &types.RepoInfo{Exists: true}, "v1.0.0")
	_ = c.SetRepoInfo("user2", "repo2", &types.RepoInfo{Exists: true}, "v2.0.0")

	count, err = c.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if count != 2 {
		t.Errorf("cache count = %d, want 2", count)
	}
}

func TestGetCacheFilePath_Deterministic(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := NewCache(false, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache() error: %v", err)
	}

	path1 := c.getCacheFilePath("user_repo")
	path2 := c.getCacheFilePath("user_repo")
	if path1 != path2 {
		t.Error("getCacheFilePath should be deterministic for same key")
	}

	path3 := c.getCacheFilePath("different_key")
	if path1 == path3 {
		t.Error("getCacheFilePath should produce different paths for different keys")
	}
}
