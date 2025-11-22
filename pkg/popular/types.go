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
	Package         string    `json:"package"`           // e.g., "github.com/gin-gonic/gin"
	Owner           string    `json:"owner"`             // GitHub owner
	Repo            string    `json:"repo"`              // GitHub repo name
	Status          Status    `json:"status"`            // Maintenance status
	DaysSinceUpdate int       `json:"days_since_update"` // Days since last commit/update
	LastUpdated     time.Time `json:"last_updated"`      // When the repo was last updated
	CacheBuiltAt    time.Time `json:"cache_built_at"`    // When this cache entry was created
}

// IsUnmaintained returns true if the package is considered unmaintained
func (e *Entry) IsUnmaintained() bool {
	return e.Status == StatusArchived ||
		e.Status == StatusInactive ||
		e.Status == StatusNotFound
}
