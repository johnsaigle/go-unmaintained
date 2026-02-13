package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
	"github.com/johnsaigle/go-unmaintained/pkg/types"
)

// testResults returns a standard set of results for testing formatters
func testResults() []analyzer.Result {
	return []analyzer.Result{
		{
			Package:         "github.com/archived/repo",
			IsUnmaintained:  true,
			IsDirect:        true,
			Reason:          analyzer.ReasonArchived,
			Details:         "Repository is archived",
			CurrentVersion:  "v1.0.0",
			DaysSinceUpdate: 500,
			RepoInfo: &types.RepoInfo{
				Exists:     true,
				IsArchived: true,
				URL:        "https://github.com/archived/repo",
			},
		},
		{
			Package:         "github.com/stale/repo",
			IsUnmaintained:  true,
			IsDirect:        false,
			Reason:          analyzer.ReasonStaleInactive,
			Details:         "Repository inactive for 400 days",
			CurrentVersion:  "v2.0.0",
			DaysSinceUpdate: 400,
			DependencyPath:  []string{"myproject", "github.com/middle/dep", "github.com/stale/repo"},
		},
		{
			Package:         "github.com/active/repo",
			IsUnmaintained:  false,
			IsDirect:        true,
			Reason:          analyzer.ReasonActive,
			Details:         "Active repository, last updated 5 days ago",
			CurrentVersion:  "v3.0.0",
			DaysSinceUpdate: 5,
		},
	}
}

func testSummary() analyzer.SummaryStats {
	return analyzer.GetSummary(testResults())
}

func TestNew(t *testing.T) {
	opts := Options{}
	formats := []string{"console", "json", "github-actions", "golangci-lint", ""}

	for _, f := range formats {
		t.Run("format:"+f, func(t *testing.T) {
			fmtr, err := New(f, opts)
			if err != nil {
				t.Fatalf("New(%q) error: %v", f, err)
			}
			if fmtr == nil {
				t.Fatalf("New(%q) returned nil", f)
			}
		})
	}

	// Unknown format should error
	_, err := New("unknown-format", opts)
	if err == nil {
		t.Error("New(unknown-format) should return error")
	}
}

func TestConsoleFormatter_Format(t *testing.T) {
	fmtr, _ := New("console", Options{Verbose: true, ShowPaths: true})
	var buf bytes.Buffer

	results := testResults()
	summary := testSummary()

	err := fmtr.Format(&buf, results, summary)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	output := buf.String()

	// Must contain unmaintained section
	if !strings.Contains(output, "UNMAINTAINED PACKAGES") {
		t.Error("output should contain UNMAINTAINED PACKAGES header")
	}

	// Must contain the archived package
	if !strings.Contains(output, "github.com/archived/repo") {
		t.Error("output should contain archived package")
	}

	// Must contain the stale package
	if !strings.Contains(output, "github.com/stale/repo") {
		t.Error("output should contain stale package")
	}

	// Verbose mode should show maintained packages
	if !strings.Contains(output, "MAINTAINED PACKAGES") {
		t.Error("verbose output should contain MAINTAINED PACKAGES header")
	}

	// Should show dependency path for indirect deps
	if !strings.Contains(output, "Dependency path") {
		t.Error("output should contain dependency path for indirect deps")
	}

	// Summary section
	if !strings.Contains(output, "ANALYSIS SUMMARY") {
		t.Error("output should contain ANALYSIS SUMMARY")
	}
}

func TestConsoleFormatter_NonVerbose(t *testing.T) {
	fmtr, _ := New("console", Options{Verbose: false})
	var buf bytes.Buffer

	err := fmtr.Format(&buf, testResults(), testSummary())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	output := buf.String()

	// Non-verbose should NOT list individual maintained packages with the check mark
	// (the summary section may still mention maintained counts)
	if strings.Contains(output, "github.com/active/repo") {
		t.Error("non-verbose output should not list individual maintained packages")
	}
}

func TestConsoleFormatter_FailFast(t *testing.T) {
	fmtr, _ := New("console", Options{FailFast: true})
	var buf bytes.Buffer

	err := fmtr.Format(&buf, testResults(), testSummary())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	output := buf.String()

	// With FailFast, should only show the first unmaintained package
	// The archived one is more severe and should be sorted first
	if !strings.Contains(output, "github.com/archived/repo") {
		t.Error("fail-fast output should contain first unmaintained package")
	}
	// The stale one should not appear (fail-fast stops after first)
	if strings.Contains(output, "github.com/stale/repo") {
		t.Error("fail-fast output should not contain second unmaintained package")
	}
}

func TestJSONFormatter_Format(t *testing.T) {
	fmtr, _ := New("json", Options{})
	var buf bytes.Buffer

	err := fmtr.Format(&buf, testResults(), testSummary())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Output should be valid JSON
	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check result count
	if len(output.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3", len(output.Results))
	}

	// Check summary
	if output.Summary.UnmaintainedCount != 2 {
		t.Errorf("Summary.UnmaintainedCount = %d, want 2", output.Summary.UnmaintainedCount)
	}

	// Check version field
	if output.Version == "" {
		t.Error("Version should not be empty")
	}

	// Check first result has repo info
	found := false
	for _, r := range output.Results {
		if r.Package == "github.com/archived/repo" {
			found = true
			if !r.IsUnmaintained {
				t.Error("archived repo should be unmaintained")
			}
			if r.RepoInfo == nil {
				t.Error("archived repo should have RepoInfo")
			} else if !r.RepoInfo.IsArchived {
				t.Error("archived repo RepoInfo.IsArchived should be true")
			}
		}
	}
	if !found {
		t.Error("archived repo not found in JSON output")
	}
}

func TestGitHubActionsFormatter_Format(t *testing.T) {
	fmtr, _ := New("github-actions", Options{})
	var buf bytes.Buffer

	err := fmtr.Format(&buf, testResults(), testSummary())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	output := buf.String()

	// Should use GitHub Actions annotation format
	if !strings.Contains(output, "::error") {
		t.Error("output should contain ::error annotation for archived repo")
	}

	// Stale/inactive should be warning, not error
	if !strings.Contains(output, "::warning") {
		t.Error("output should contain ::warning annotation for stale repo or summary")
	}

	// Should reference go.mod
	if !strings.Contains(output, "file=go.mod") {
		t.Error("output should reference go.mod file")
	}
}

func TestGolangciLintFormatter_Format(t *testing.T) {
	fmtr, _ := New("golangci-lint", Options{})
	var buf bytes.Buffer

	err := fmtr.Format(&buf, testResults(), testSummary())
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	output := buf.String()

	// Should use golangci-lint format: go.mod:1:1: message (unmaintained)
	if !strings.Contains(output, "go.mod:1:1:") {
		t.Error("output should be in golangci-lint format")
	}
	if !strings.Contains(output, "(unmaintained)") {
		t.Error("output should contain (unmaintained) linter name")
	}

	// Should mention archived
	if !strings.Contains(output, "archived") {
		t.Error("output should mention archived status")
	}
}

func TestGolangciLintFormatter_ShouldExit(t *testing.T) {
	fmtr, _ := New("golangci-lint", Options{})
	// golangci-lint formatter should always return 0
	code := fmtr.ShouldExit(testResults())
	if code != 0 {
		t.Errorf("golangci-lint ShouldExit() = %d, want 0", code)
	}
}

func TestDefaultShouldExit(t *testing.T) {
	unmaintainedResults := []analyzer.Result{
		{IsUnmaintained: true},
		{IsUnmaintained: false},
	}
	allGoodResults := []analyzer.Result{
		{IsUnmaintained: false},
	}

	// With unmaintained packages, should return 1
	if code := DefaultShouldExit(unmaintainedResults, false); code != 1 {
		t.Errorf("DefaultShouldExit(unmaintained, false) = %d, want 1", code)
	}

	// With --no-exit-code, should return 0
	if code := DefaultShouldExit(unmaintainedResults, true); code != 0 {
		t.Errorf("DefaultShouldExit(unmaintained, true) = %d, want 0", code)
	}

	// No unmaintained packages, should return 0
	if code := DefaultShouldExit(allGoodResults, false); code != 0 {
		t.Errorf("DefaultShouldExit(allGood, false) = %d, want 0", code)
	}

	// Empty results, should return 0
	if code := DefaultShouldExit(nil, false); code != 0 {
		t.Errorf("DefaultShouldExit(nil, false) = %d, want 0", code)
	}
}

func TestGetRepositoryURL(t *testing.T) {
	tests := []struct {
		name   string
		result analyzer.Result
		want   string
	}{
		{
			name: "from RepoInfo URL",
			result: analyzer.Result{
				Package:  "github.com/user/repo",
				RepoInfo: &types.RepoInfo{URL: "https://github.com/user/repo"},
			},
			want: "https://github.com/user/repo",
		},
		{
			name:   "from GitHub package path",
			result: analyzer.Result{Package: "github.com/user/repo"},
			want:   "https://github.com/user/repo",
		},
		{
			name:   "from GitLab package path",
			result: analyzer.Result{Package: "gitlab.com/org/project"},
			want:   "https://gitlab.com/org/project",
		},
		{
			name:   "from Bitbucket package path",
			result: analyzer.Result{Package: "bitbucket.org/team/lib"},
			want:   "https://bitbucket.org/team/lib",
		},
		{
			name:   "unknown host returns empty",
			result: analyzer.Result{Package: "example.com/pkg"},
			want:   "",
		},
		{
			name:   "short path returns empty",
			result: analyzer.Result{Package: "github.com"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRepositoryURL(tt.result)
			if got != tt.want {
				t.Errorf("GetRepositoryURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetSeverityScore(t *testing.T) {
	tests := []struct {
		name   string
		result analyzer.Result
		want   int
	}{
		{
			name:   "direct archived (most severe)",
			result: analyzer.Result{IsDirect: true, Reason: analyzer.ReasonArchived},
			want:   0,
		},
		{
			name:   "direct not found",
			result: analyzer.Result{IsDirect: true, Reason: analyzer.ReasonNotFound},
			want:   10,
		},
		{
			name:   "direct stale",
			result: analyzer.Result{IsDirect: true, Reason: analyzer.ReasonStaleInactive},
			want:   20,
		},
		{
			name:   "direct outdated",
			result: analyzer.Result{IsDirect: true, Reason: analyzer.ReasonOutdated},
			want:   30,
		},
		{
			name:   "indirect archived",
			result: analyzer.Result{IsDirect: false, Reason: analyzer.ReasonArchived},
			want:   50,
		},
		{
			name:   "indirect stale",
			result: analyzer.Result{IsDirect: false, Reason: analyzer.ReasonStaleInactive},
			want:   70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSeverityScore(tt.result)
			if got != tt.want {
				t.Errorf("getSeverityScore() = %d, want %d", got, tt.want)
			}
		})
	}

	// Ordering invariant: direct always more severe than indirect for same reason
	directArchived := getSeverityScore(analyzer.Result{IsDirect: true, Reason: analyzer.ReasonArchived})
	indirectArchived := getSeverityScore(analyzer.Result{IsDirect: false, Reason: analyzer.ReasonArchived})
	if directArchived >= indirectArchived {
		t.Error("direct should be more severe (lower score) than indirect for same reason")
	}
}
