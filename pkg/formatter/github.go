package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
)

// GitHubActionsFormatter formats output for GitHub Actions annotations
type GitHubActionsFormatter struct {
	opts Options
}

// Format writes results in GitHub Actions annotations format
// https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions
func (f *GitHubActionsFormatter) Format(w io.Writer, results []analyzer.Result, summary analyzer.SummaryStats) error {
	for _, result := range results {
		if !result.IsUnmaintained {
			continue
		}

		depType := "indirect"
		if result.IsDirect {
			depType = "direct"
		}

		// Determine severity
		severity := "error"
		if result.Reason == analyzer.ReasonStaleInactive {
			severity = "warning"
		}

		// Get URL for reference
		url := getRepositoryURL(result)

		// Format message
		message := fmt.Sprintf("%s (%s): %s", result.Package, depType, result.Details)
		if url != "" {
			message += fmt.Sprintf(" - %s", url)
		}

		// Output annotation
		// Format: ::{severity} file={name},line={line},title={title}::{message}
		fmt.Fprintf(w, "::%s file=go.mod,title=Unmaintained Dependency::%s\n", severity, message)

		// For indirect dependencies, add additional context
		if !result.IsDirect && len(result.DependencyPath) > 0 {
			pathStr := strings.Join(result.DependencyPath, " → ")
			fmt.Fprintf(w, "::notice file=go.mod,title=Dependency Path::%s\n", pathStr)
		}
	}

	// Output summary
	if summary.UnmaintainedCount > 0 {
		fmt.Fprintf(w, "::warning::Found %d unmaintained packages (%d direct, %d indirect)\n",
			summary.UnmaintainedCount, summary.DirectUnmaintained, summary.IndirectUnmaintained)
	} else {
		fmt.Fprintln(w, "::notice::All dependencies are maintained")
	}

	return nil
}

// ShouldExit returns the exit code based on results
func (f *GitHubActionsFormatter) ShouldExit(results []analyzer.Result) int {
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
