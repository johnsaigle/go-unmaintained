package types

import "time"

// RepoInfo holds repository information in a hosting-provider-agnostic format.
// It is used by analyzer, cache, formatter, and providers packages.
type RepoInfo struct {
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastCommitAt  *time.Time
	Description   string
	DefaultBranch string
	URL           string
	IsArchived    bool
	Exists        bool
}

// IsRepositoryActive checks if a repository has been active within the given duration.
// It uses the latest of UpdatedAt or LastCommitAt.
func (info *RepoInfo) IsRepositoryActive(maxAge time.Duration) bool {
	if !info.Exists {
		return false
	}

	latestActivity := info.UpdatedAt
	if info.LastCommitAt != nil && info.LastCommitAt.After(info.UpdatedAt) {
		latestActivity = *info.LastCommitAt
	}

	return time.Since(latestActivity) <= maxAge
}

// DaysSinceLastActivity returns the number of days since the last repository activity.
// Returns -1 if the repository does not exist.
func (info *RepoInfo) DaysSinceLastActivity() int {
	if !info.Exists {
		return -1
	}

	latestActivity := info.UpdatedAt
	if info.LastCommitAt != nil && info.LastCommitAt.After(info.UpdatedAt) {
		latestActivity = *info.LastCommitAt
	}

	return int(time.Since(latestActivity).Hours() / 24)
}
