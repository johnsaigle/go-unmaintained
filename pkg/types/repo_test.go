package types

import (
	"testing"
	"time"
)

func TestIsRepositoryActive(t *testing.T) {
	now := time.Now()
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
	twoYearsAgo := now.Add(-730 * 24 * time.Hour)

	tests := []struct {
		name            string
		exists          bool
		isArchived      bool
		updatedAt       time.Time
		lastCommitAt    *time.Time
		maxAge          time.Duration
		wantActive      bool
	}{
		{
			name:          "nonexistent repo is never active",
			exists:        false,
			updatedAt:     now,
			maxAge:        365 * 24 * time.Hour,
			wantActive:    false,
		},
		{
			name:          "archived repo is never active",
			exists:        true,
			isArchived:    true,
			updatedAt:     now,
			maxAge:        365 * 24 * time.Hour,
			wantActive:    false,
		},
		{
			name:          "recently updated repo is active",
			exists:        true,
			updatedAt:     tenDaysAgo,
			maxAge:        365 * 24 * time.Hour,
			wantActive:    true,
		},
		{
			name:          "repo updated just inside max age",
			exists:        true,
			updatedAt:     now.Add(-364 * 24 * time.Hour),
			maxAge:        365 * 24 * time.Hour,
			wantActive:    true,
		},
		{
			name:          "repo updated beyond max age is inactive",
			exists:        true,
			updatedAt:     twoYearsAgo,
			maxAge:        365 * 24 * time.Hour,
			wantActive:    false,
		},
		{
			name:          "maxAge exactly zero age boundary",
			exists:        true,
			updatedAt:     now,
			maxAge:        0,
			wantActive:    false,
		},
		{
			name:          "last commit newer than updatedAt uses commit time",
			exists:        true,
			updatedAt:     twoYearsAgo,
			lastCommitAt:  &fiveDaysAgo,
			maxAge:        365 * 24 * time.Hour,
			wantActive:    true,
		},
		{
			name:          "last commit older than updatedAt keeps updatedAt",
			exists:        true,
			updatedAt:     fiveDaysAgo,
			lastCommitAt:  &twoYearsAgo,
			maxAge:        365 * 24 * time.Hour,
			wantActive:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &RepoInfo{
				Exists:        tt.exists,
				IsArchived:    tt.isArchived,
				UpdatedAt:     tt.updatedAt,
				LastCommitAt:  tt.lastCommitAt,
			}
			if got := repo.IsRepositoryActive(tt.maxAge); got != tt.wantActive {
				t.Errorf("IsRepositoryActive() = %v, want %v", got, tt.wantActive)
			}
		})
	}
}

func TestIsRepositoryActive_ZeroMaxAge(t *testing.T) {
	// maxAge of zero means the repo has already exceeded its age at this instant
	repo := &RepoInfo{Exists: true, UpdatedAt: time.Now()}
	if got := repo.IsRepositoryActive(0); got {
		t.Errorf("IsRepositoryActive(0) = true, want false for a zero max age")
	}
}

func TestDaysSinceLastActivity(t *testing.T) {
	// Last activity is the later of updatedAt and lastCommitAt
	now := time.Now()
	twoDaysAgo := now.Add(-2 * 24 * time.Hour)
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)

	t.Run("nonexistent repo returns -1", func(t *testing.T) {
		repo := &RepoInfo{Exists: false}
		if got := repo.DaysSinceLastActivity(); got != -1 {
			t.Errorf("DaysSinceLastActivity() = %v, want -1", got)
		}
	})

	t.Run("inexact day count from updatedAt when no commit", func(t *testing.T) {
		repo := &RepoInfo{Exists: true, UpdatedAt: tenDaysAgo}
		if got := repo.DaysSinceLastActivity(); got != 10 {
			t.Errorf("DaysSinceLastActivity() = %v, want 10", got)
		}
	})

	t.Run("uses lastCommitAt when it is newer than updatedAt", func(t *testing.T) {
		repo := &RepoInfo{
			Exists:       true,
			UpdatedAt:    tenDaysAgo,
			LastCommitAt: &twoDaysAgo,
		}
		if got := repo.DaysSinceLastActivity(); got != 2 {
			t.Errorf("DaysSinceLastActivity() = %v, want 2", got)
		}
	})

	t.Run("keeps updatedAt when commit is older", func(t *testing.T) {
		repo := &RepoInfo{
			Exists:       true,
			UpdatedAt:    twoDaysAgo,
			LastCommitAt: &tenDaysAgo,
		}
		if got := repo.DaysSinceLastActivity(); got != 2 {
			t.Errorf("DaysSinceLastActivity() = %v, want 2", got)
		}
	})
}