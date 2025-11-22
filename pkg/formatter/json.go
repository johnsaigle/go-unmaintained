package formatter

import (
	"encoding/json"
	"io"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
)

// JSONFormatter formats output as JSON
type JSONFormatter struct {
	opts Options
}

// JSONOutput represents the JSON output structure
type JSONOutput struct {
	Timestamp time.Time             `json:"timestamp"`
	Version   string                `json:"version"`
	Results   []JSONResult          `json:"results"`
	Summary   analyzer.SummaryStats `json:"summary"`
}

// JSONResult represents a single dependency result in JSON format
type JSONResult struct {
	RepoInfo        *JSONRepoInfo `json:"repo_info,omitempty"`
	Package         string        `json:"package"`
	Reason          string        `json:"reason,omitempty"`
	Details         string        `json:"details"`
	CurrentVersion  string        `json:"current_version,omitempty"`
	LatestVersion   string        `json:"latest_version,omitempty"`
	DependencyPath  []string      `json:"dependency_path,omitempty"`
	DaysSinceUpdate int           `json:"days_since_update,omitempty"`
	IsUnmaintained  bool          `json:"is_unmaintained"`
	IsDirect        bool          `json:"is_direct"`
}

// JSONRepoInfo represents repository information in JSON format
type JSONRepoInfo struct {
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
	URL            string    `json:"url,omitempty"`
	LastCommitDays int       `json:"last_commit_days,omitempty"`
	IsArchived     bool      `json:"is_archived"`
}

// Format writes results in JSON format
func (f *JSONFormatter) Format(w io.Writer, results []analyzer.Result, summary analyzer.SummaryStats) error {
	// Convert results to JSON-friendly format
	jsonResults := make([]JSONResult, len(results))
	for i, result := range results {
		jsonResult := JSONResult{
			Package:         result.Package,
			IsUnmaintained:  result.IsUnmaintained,
			IsDirect:        result.IsDirect,
			Reason:          string(result.Reason),
			Details:         result.Details,
			CurrentVersion:  result.CurrentVersion,
			LatestVersion:   result.LatestVersion,
			DaysSinceUpdate: result.DaysSinceUpdate,
			DependencyPath:  result.DependencyPath,
		}

		// Add repo info if available
		if result.RepoInfo != nil {
			repoInfo := &JSONRepoInfo{
				URL:        result.RepoInfo.URL,
				IsArchived: result.RepoInfo.IsArchived,
				CreatedAt:  result.RepoInfo.CreatedAt,
				UpdatedAt:  result.RepoInfo.UpdatedAt,
			}

			// Calculate days since last commit
			if result.RepoInfo.LastCommitAt != nil {
				repoInfo.LastCommitDays = int(time.Since(*result.RepoInfo.LastCommitAt).Hours() / 24)
			}

			jsonResult.RepoInfo = repoInfo
		}

		jsonResults[i] = jsonResult
	}

	output := JSONOutput{
		Summary:   summary,
		Results:   jsonResults,
		Timestamp: time.Now(),
		Version:   "1.0.0", // Tool version
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// ShouldExit returns the exit code based on results
func (f *JSONFormatter) ShouldExit(results []analyzer.Result) int {
	if f.opts.NoExitCode {
		return 0
	}

	for _, result := range results {
		if result.IsUnmaintained {
			return 1
		}
	}

	return 0
}
