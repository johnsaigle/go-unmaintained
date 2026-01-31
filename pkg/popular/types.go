package popular

import "time"

// Status represents the maintenance status of a package
type Status string

const (
	StatusActive   Status = "active"    // Repository is actively maintained
	StatusArchived Status = "archived"  // Repository is archived
	StatusInactive Status = "inactive"  // Repository is inactive (no recent updates)
	StatusNotFound Status = "not_found" // Repository not found
)

// Entry represents a cached popular package entry
type Entry struct {
	LastUpdated  time.Time `json:"last_updated"`
	CacheBuiltAt time.Time `json:"cache_built_at"`
	Package      string    `json:"package"`
	Owner        string    `json:"owner"`
	Repo         string    `json:"repo"`
	Status       Status    `json:"status"`
}

// IsUnmaintained returns true if the package is considered unmaintained
func (e *Entry) IsUnmaintained() bool {
	return e.Status == StatusArchived ||
		e.Status == StatusInactive ||
		e.Status == StatusNotFound
}

// DaysSinceUpdate returns the number of days since the package was last updated
func (e *Entry) DaysSinceUpdate() int {
	return int(time.Since(e.LastUpdated).Hours() / 24)
}

// DaysSinceCacheBuild returns the number of days since the cache was built
func (e *Entry) DaysSinceCacheBuild() int {
	return int(time.Since(e.CacheBuiltAt).Hours() / 24)
}
