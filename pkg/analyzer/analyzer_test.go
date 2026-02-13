package analyzer

import (
	"testing"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/parser"
	"github.com/johnsaigle/go-unmaintained/pkg/popular"
)

func TestIsVersionOutdated(t *testing.T) {
	a := &Analyzer{}

	tests := []struct {
		name     string
		current  string
		latest   string
		expected bool
	}{
		{"same version", "v1.2.3", "v1.2.3", false},
		{"current is older", "v1.0.0", "v1.2.3", true},
		{"current is newer", "v2.0.0", "v1.2.3", false},
		{"patch behind", "v1.2.0", "v1.2.3", true},
		{"without v prefix", "1.2.3", "1.2.3", false},
		{"current without v, older", "1.0.0", "1.2.3", true},
		{"empty current", "", "v1.2.3", false},
		{"empty latest", "v1.2.3", "", false},
		{"both empty", "", "", false},
		{"invalid current", "notaversion", "v1.2.3", false},
		{"invalid latest", "v1.2.3", "notaversion", false},
		{"major version bump", "v1.9.9", "v2.0.0", true},
		{"pre-release", "v1.2.3-alpha", "v1.2.3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := a.isVersionOutdated(tt.current, tt.latest)
			if result != tt.expected {
				t.Errorf("isVersionOutdated(%q, %q) = %v, want %v",
					tt.current, tt.latest, result, tt.expected)
			}
		})
	}
}

func TestGetSummary(t *testing.T) {
	results := []Result{
		{IsUnmaintained: true, IsDirect: true, Reason: ReasonArchived},
		{IsUnmaintained: true, IsDirect: true, Reason: ReasonNotFound},
		{IsUnmaintained: true, IsDirect: false, Reason: ReasonStaleInactive},
		{IsUnmaintained: true, IsDirect: false, Reason: ReasonOutdated},
		{IsUnmaintained: false, Reason: ReasonActive},
		{IsUnmaintained: false, Reason: ReasonUnknown},
		{IsUnmaintained: false, IsRetracted: true, Reason: ReasonActive},
	}

	summary := GetSummary(results)

	if summary.TotalDependencies != 7 {
		t.Errorf("TotalDependencies = %d, want 7", summary.TotalDependencies)
	}
	if summary.UnmaintainedCount != 4 {
		t.Errorf("UnmaintainedCount = %d, want 4", summary.UnmaintainedCount)
	}
	if summary.DirectUnmaintained != 2 {
		t.Errorf("DirectUnmaintained = %d, want 2", summary.DirectUnmaintained)
	}
	if summary.IndirectUnmaintained != 2 {
		t.Errorf("IndirectUnmaintained = %d, want 2", summary.IndirectUnmaintained)
	}
	if summary.ArchivedCount != 1 {
		t.Errorf("ArchivedCount = %d, want 1", summary.ArchivedCount)
	}
	if summary.NotFoundCount != 1 {
		t.Errorf("NotFoundCount = %d, want 1", summary.NotFoundCount)
	}
	if summary.StaleInactiveCount != 1 {
		t.Errorf("StaleInactiveCount = %d, want 1", summary.StaleInactiveCount)
	}
	if summary.OutdatedCount != 1 {
		t.Errorf("OutdatedCount = %d, want 1", summary.OutdatedCount)
	}
	if summary.UnknownCount != 1 {
		t.Errorf("UnknownCount = %d, want 1", summary.UnknownCount)
	}
	if summary.RetractedCount != 1 {
		t.Errorf("RetractedCount = %d, want 1", summary.RetractedCount)
	}
}

func TestGetSummary_Empty(t *testing.T) {
	summary := GetSummary(nil)
	if summary.TotalDependencies != 0 {
		t.Errorf("TotalDependencies = %d, want 0", summary.TotalDependencies)
	}
	if summary.UnmaintainedCount != 0 {
		t.Errorf("UnmaintainedCount = %d, want 0", summary.UnmaintainedCount)
	}
}

// Note: getSeverityScore is in the formatter package — tested in formatter_test.go

func TestGetTrustedModuleStatus(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"google.golang.org", "google.golang.org/protobuf", "Active Google-maintained Go package (trusted)"},
		{"cloud.google.com", "cloud.google.com/go/storage", "Active Google Cloud Go package (trusted)"},
		{"k8s.io", "k8s.io/api", "Active Kubernetes package (trusted)"},
		{"sigs.k8s.io", "sigs.k8s.io/controller-runtime", "Active Kubernetes SIG package (trusted)"},
		{"go.uber.org", "go.uber.org/zap", "Active Uber-maintained Go package (trusted)"},
		{"unknown trusted", "something.else/pkg", "Active trusted Go package"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTrustedModuleStatus(tt.path)
			if got != tt.want {
				t.Errorf("getTrustedModuleStatus(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestMaxInt(t *testing.T) {
	if maxInt(3, 5) != 5 {
		t.Error("maxInt(3, 5) should be 5")
	}
	if maxInt(5, 3) != 5 {
		t.Error("maxInt(5, 3) should be 5")
	}
	if maxInt(4, 4) != 4 {
		t.Error("maxInt(4, 4) should be 4")
	}
}

func TestResultFromPopularEntry(t *testing.T) {
	a := &Analyzer{config: Config{MaxAge: 365 * 24 * time.Hour}}
	now := time.Now()

	tests := []struct {
		entry            *popular.Entry
		dep              parser.Dependency
		name             string
		wantReason       UnmaintainedReason
		wantUnmaintained bool
	}{
		{
			name: "active package",
			entry: &popular.Entry{
				Package:      "github.com/user/repo",
				Status:       popular.StatusActive,
				LastUpdated:  now.Add(-30 * 24 * time.Hour),
				CacheBuiltAt: now.Add(-1 * 24 * time.Hour),
			},
			dep:              parser.Dependency{Path: "github.com/user/repo", Version: "v1.0.0"},
			wantUnmaintained: false,
			wantReason:       ReasonActive,
		},
		{
			name: "archived package",
			entry: &popular.Entry{
				Package:      "github.com/old/repo",
				Status:       popular.StatusArchived,
				LastUpdated:  now.Add(-500 * 24 * time.Hour),
				CacheBuiltAt: now.Add(-1 * 24 * time.Hour),
			},
			dep:              parser.Dependency{Path: "github.com/old/repo", Version: "v0.1.0"},
			wantUnmaintained: true,
			wantReason:       ReasonArchived,
		},
		{
			name: "inactive package",
			entry: &popular.Entry{
				Package:      "github.com/stale/repo",
				Status:       popular.StatusInactive,
				LastUpdated:  now.Add(-400 * 24 * time.Hour),
				CacheBuiltAt: now.Add(-1 * 24 * time.Hour),
			},
			dep:              parser.Dependency{Path: "github.com/stale/repo", Version: "v2.0.0"},
			wantUnmaintained: true,
			wantReason:       ReasonStaleInactive,
		},
		{
			name: "not found package",
			entry: &popular.Entry{
				Package:      "github.com/gone/repo",
				Status:       popular.StatusNotFound,
				CacheBuiltAt: now.Add(-1 * 24 * time.Hour),
			},
			dep:              parser.Dependency{Path: "github.com/gone/repo", Version: "v1.0.0"},
			wantUnmaintained: true,
			wantReason:       ReasonNotFound,
		},
		{
			name: "indirect dependency marked correctly",
			entry: &popular.Entry{
				Package:      "github.com/dep/repo",
				Status:       popular.StatusActive,
				LastUpdated:  now.Add(-10 * 24 * time.Hour),
				CacheBuiltAt: now.Add(-1 * 24 * time.Hour),
			},
			dep:              parser.Dependency{Path: "github.com/dep/repo", Version: "v1.0.0", Indirect: true},
			wantUnmaintained: false,
			wantReason:       ReasonActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := a.resultFromPopularEntry(tt.entry, tt.dep)
			if result.IsUnmaintained != tt.wantUnmaintained {
				t.Errorf("IsUnmaintained = %v, want %v", result.IsUnmaintained, tt.wantUnmaintained)
			}
			if result.Reason != tt.wantReason {
				t.Errorf("Reason = %v, want %v", result.Reason, tt.wantReason)
			}
			if result.Package != tt.dep.Path {
				t.Errorf("Package = %q, want %q", result.Package, tt.dep.Path)
			}
			if result.CurrentVersion != tt.dep.Version {
				t.Errorf("CurrentVersion = %q, want %q", result.CurrentVersion, tt.dep.Version)
			}
			// Indirect dep should be marked as not-direct
			if tt.dep.Indirect && result.IsDirect {
				t.Error("expected indirect dependency to have IsDirect=false")
			}
		})
	}
}
