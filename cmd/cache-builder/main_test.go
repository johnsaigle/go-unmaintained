package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ghclient "github.com/johnsaigle/go-unmaintained/pkg/github"
	"github.com/johnsaigle/go-unmaintained/pkg/popular"
)

func TestBuildUpdatePlanBudgetsAddsAndRefreshesIndependently(t *testing.T) {
	now := time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC)
	ranked := []rankedRepository{
		{packagePath: "github.com/example/one"},
		{packagePath: "github.com/example/two"},
		{packagePath: "github.com/example/three"},
		{packagePath: "github.com/example/four"},
		{packagePath: "github.com/example/five"},
	}
	existing := map[string]popular.Entry{
		"github.com/example/two": {
			Package:      "github.com/example/two",
			CacheBuiltAt: now.Add(-100 * 24 * time.Hour),
		},
		"github.com/example/four": {
			Package:      "github.com/example/four",
			CacheBuiltAt: now.Add(-10 * 24 * time.Hour),
		},
		"github.com/example/five": {
			Package:      "github.com/example/five",
			CacheBuiltAt: now.Add(-100 * 24 * time.Hour),
		},
	}

	plans := buildUpdatePlan(ranked, existing, now, 1, 1, 90)
	want := []updateAction{addEntry, refreshEntry, skipEntry, keepEntry, keepEntry}
	if len(plans) != len(want) {
		t.Fatalf("len(plans) = %d, want %d", len(plans), len(want))
	}
	for i, plan := range plans {
		if plan.action != want[i] {
			t.Errorf("plan[%d].action = %v, want %v", i, plan.action, want[i])
		}
	}
}

func TestBuildUpdatePlanOnlyRetainsRankedRepositories(t *testing.T) {
	now := time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC)
	ranked := []rankedRepository{{packagePath: "github.com/example/ranked"}}
	existing := map[string]popular.Entry{
		"github.com/example/ranked": {
			Package:      "github.com/example/ranked",
			CacheBuiltAt: now,
		},
		"github.com/example/no-longer-ranked": {
			Package:      "github.com/example/no-longer-ranked",
			CacheBuiltAt: now,
		},
	}

	plans := buildUpdatePlan(ranked, existing, now, 10, 10, 90)
	if len(plans) != 1 {
		t.Fatalf("len(plans) = %d, want 1", len(plans))
	}
	if plans[0].repository.packagePath != "github.com/example/ranked" {
		t.Fatalf("planned repository = %q, want ranked repository", plans[0].repository.packagePath)
	}
}

func TestCalculateStats(t *testing.T) {
	now := time.Now()

	entries := []popular.Entry{
		{Status: popular.StatusActive, CacheBuiltAt: now},
		{Status: popular.StatusActive, CacheBuiltAt: now},
		{Status: popular.StatusArchived, CacheBuiltAt: now},
		{Status: popular.StatusInactive, CacheBuiltAt: now},
		{Status: popular.StatusNotFound, CacheBuiltAt: now},
	}
	want := stats{active: 2, archived: 1, inactive: 1, notFound: 1}
	got := calculateStats(entries)
	if got != want {
		t.Errorf("calculateStats() = %+v, want %+v", got, want)
	}
}

func TestCalculateStats_Empty(t *testing.T) {
	got := calculateStats(nil)
	want := stats{}
	if got != want {
		t.Errorf("calculateStats(nil) = %+v, want empty", got)
	}
}

// analyzeRepository — happy path: a recently-updated, non-archived repo
// should be classified as active with the correct package path and lastUpdated.
func TestAnalyzeRepository_Active(t *testing.T) {
	recent := time.Now().Add(-24 * time.Hour)
	commitAt := time.Now().Add(-12 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"archived":false,"default_branch":"main","updated_at":"%s"}`, recent.Format(time.RFC3339))
		case "/repos/o/r/commits":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `[{"commit":{"committer":{"date":"%s"}}}]`, commitAt.Format(time.RFC3339))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := ghclient.NewClientForTest(server.URL)
	cacheBuiltAt := time.Now()
	entry := analyzeRepository(context.Background(), c, "o", "r", 365, cacheBuiltAt)

	if entry.Owner != "o" {
		t.Errorf("Owner = %q, want o", entry.Owner)
	}
	if entry.Repo != "r" {
		t.Errorf("Repo = %q, want r", entry.Repo)
	}
	if entry.Package != "github.com/o/r" {
		t.Errorf("Package = %q, want github.com/o/r", entry.Package)
	}
	if entry.Status != popular.StatusActive {
		t.Errorf("Status = %q, want active", entry.Status)
	}
	if entry.LastUpdated.IsZero() {
		t.Error("LastUpdated was zero, expected a non-zero timestamp")
	}
	if entry.LastUpdated.After(time.Now()) {
		t.Errorf("LastUpdated = %v is in the future", entry.LastUpdated)
	}
	if !entry.CacheBuiltAt.Equal(cacheBuiltAt) {
		t.Errorf("CacheBuiltAt = %v, want %v", entry.CacheBuiltAt, cacheBuiltAt)
	}
}

// analyzeRepository — archived repo: status must be StatusArchived even if recently updated.
func TestAnalyzeRepository_Archived(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/repos/o/r" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"archived":true,"default_branch":"main","updated_at":"2024-01-01T00:00:00Z"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := ghclient.NewClientForTest(server.URL)
	entry := analyzeRepository(context.Background(), c, "o", "r", 365, time.Now())
	if entry.Status != popular.StatusArchived {
		t.Errorf("Status = %q, want archived", entry.Status)
	}
}

// analyzeRepository — not found (404): status must be StatusNotFound.
func TestAnalyzeRepository_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := ghclient.NewClientForTest(server.URL)
	entry := analyzeRepository(context.Background(), c, "o", "missing", 365, time.Now())
	if entry.Status != popular.StatusNotFound {
		t.Errorf("Status = %q, want not_found", entry.Status)
	}
	if entry.Package != "github.com/o/missing" {
		t.Errorf("Package = %q, want github.com/o/missing", entry.Package)
	}
}

// analyzeRepository — STALE repo: not archived but not recently updated.
func TestAnalyzeRepository_Inactive(t *testing.T) {
	oldUpdate := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/o/r":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"archived":false,"default_branch":"main","updated_at":"2020-01-01T00:00:00Z"}`)
		case "/repos/o/r/commits":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `[{"commit":{"committer":{"date":"2020-01-01T00:00:00Z"}}}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	c := ghclient.NewClientForTest(server.URL)
	entry := analyzeRepository(context.Background(), c, "o", "r", 365, time.Now())
	if entry.Status != popular.StatusInactive {
		t.Errorf("Status = %q, want inactive", entry.Status)
	}
	_ = oldUpdate
}
