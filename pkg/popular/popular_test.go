package popular

import (
	"testing"
	"time"
)

func TestEntryIsUnmaintained(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"active is maintained", StatusActive, false},
		{"archived is unmaintained", StatusArchived, true},
		{"inactive is unmaintained", StatusInactive, true},
		{"not_found is unmaintained", StatusNotFound, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Entry{Status: tt.status}
			if got := e.IsUnmaintained(); got != tt.want {
				t.Errorf("IsUnmaintained() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntryDaysSinceUpdate(t *testing.T) {
	e := &Entry{
		LastUpdated: time.Now().Add(-10 * 24 * time.Hour),
	}
	days := e.DaysSinceUpdate()
	// Allow 1 day tolerance for test timing
	if days < 9 || days > 11 {
		t.Errorf("DaysSinceUpdate() = %d, want ~10", days)
	}
}

func TestEntryDaysSinceCacheBuild(t *testing.T) {
	e := &Entry{
		CacheBuiltAt: time.Now().Add(-5 * 24 * time.Hour),
	}
	days := e.DaysSinceCacheBuild()
	if days < 4 || days > 6 {
		t.Errorf("DaysSinceCacheBuild() = %d, want ~5", days)
	}
}

func TestLookup_EmptyCache(t *testing.T) {
	// The default cache is initialized from the embedded data.
	// With the empty seed [], it should not be loaded.
	// But if the test binary has a populated cache, this still works.
	entry, found := Lookup("nonexistent/package/that/does/not/exist")
	if found {
		t.Errorf("Lookup() found entry for nonexistent package: %+v", entry)
	}
	if entry != nil {
		t.Errorf("Lookup() returned non-nil entry for nonexistent package")
	}
}

func TestSize(t *testing.T) {
	// Size should return 0 or a positive number
	size := Size()
	if size < 0 {
		t.Errorf("Size() = %d, want >= 0", size)
	}
}

func TestStats(t *testing.T) {
	stats := Stats()
	if stats == "" {
		t.Error("Stats() returned empty string")
	}
	// Should either be "not loaded" or include a count
	if !IsLoaded() {
		if stats != "Popular cache: not loaded" {
			t.Errorf("Stats() = %q, want 'Popular cache: not loaded'", stats)
		}
	}
}
