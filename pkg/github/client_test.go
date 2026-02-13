package github

import (
	"testing"
	"time"
)

func TestRepoInfo_IsRepositoryActive(t *testing.T) {
	now := time.Now()
	maxAge := 365 * 24 * time.Hour

	tests := []struct {
		name     string
		info     RepoInfo
		maxAge   time.Duration
		expected bool
	}{
		{
			name:     "non-existent repo",
			info:     RepoInfo{Exists: false},
			maxAge:   maxAge,
			expected: false,
		},
		{
			name: "recently updated repo",
			info: RepoInfo{
				Exists:    true,
				UpdatedAt: now.Add(-30 * 24 * time.Hour),
			},
			maxAge:   maxAge,
			expected: true,
		},
		{
			name: "stale repo (updated long ago)",
			info: RepoInfo{
				Exists:    true,
				UpdatedAt: now.Add(-500 * 24 * time.Hour),
			},
			maxAge:   maxAge,
			expected: false,
		},
		{
			name: "stale UpdatedAt but recent commit",
			info: RepoInfo{
				Exists:       true,
				UpdatedAt:    now.Add(-500 * 24 * time.Hour),
				LastCommitAt: timePtr(now.Add(-10 * 24 * time.Hour)),
			},
			maxAge:   maxAge,
			expected: true,
		},
		{
			name: "recent UpdatedAt but old commit",
			info: RepoInfo{
				Exists:       true,
				UpdatedAt:    now.Add(-10 * 24 * time.Hour),
				LastCommitAt: timePtr(now.Add(-500 * 24 * time.Hour)),
			},
			maxAge:   maxAge,
			expected: true, // Uses the latest of UpdatedAt/LastCommitAt
		},
		{
			name: "just inside boundary (364 days)",
			info: RepoInfo{
				Exists:    true,
				UpdatedAt: now.Add(-364 * 24 * time.Hour),
			},
			maxAge:   maxAge,
			expected: true,
		},
		{
			name: "just past boundary",
			info: RepoInfo{
				Exists:    true,
				UpdatedAt: now.Add(-366 * 24 * time.Hour),
			},
			maxAge:   maxAge,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.IsRepositoryActive(tt.maxAge)
			if got != tt.expected {
				t.Errorf("IsRepositoryActive(%v) = %v, want %v", tt.maxAge, got, tt.expected)
			}
		})
	}
}

func TestRepoInfo_DaysSinceLastActivity(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		info RepoInfo
		want int
	}{
		{
			name: "non-existent returns -1",
			info: RepoInfo{Exists: false},
			want: -1,
		},
		{
			name: "updated 10 days ago",
			info: RepoInfo{
				Exists:    true,
				UpdatedAt: now.Add(-10 * 24 * time.Hour),
			},
			want: 10,
		},
		{
			name: "commit is more recent than UpdatedAt",
			info: RepoInfo{
				Exists:       true,
				UpdatedAt:    now.Add(-100 * 24 * time.Hour),
				LastCommitAt: timePtr(now.Add(-5 * 24 * time.Hour)),
			},
			want: 5,
		},
		{
			name: "UpdatedAt is more recent than commit",
			info: RepoInfo{
				Exists:       true,
				UpdatedAt:    now.Add(-3 * 24 * time.Hour),
				LastCommitAt: timePtr(now.Add(-50 * 24 * time.Hour)),
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.DaysSinceLastActivity()
			// Allow 1 day tolerance for timing
			if got < tt.want-1 || got > tt.want+1 {
				t.Errorf("DaysSinceLastActivity() = %d, want ~%d", got, tt.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
