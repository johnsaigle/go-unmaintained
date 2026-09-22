package main

import (
	"testing"
	"time"

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
