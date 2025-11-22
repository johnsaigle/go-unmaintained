package formatter

import (
	"fmt"
	"io"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
)

// GolangciLintFormatter formats output for golangci-lint integration
type GolangciLintFormatter struct {
	opts Options
}

// Format writes results in golangci-lint compatible format
// Format: {filename}:{line}:{column}: {message} ({linter})
func (f *GolangciLintFormatter) Format(w io.Writer, results []analyzer.Result, summary analyzer.SummaryStats) error {
	for _, result := range results {
		if !result.IsUnmaintained {
			continue
		}

		// Format: go.mod:1:1: message (linter-name)
		msg := f.formatMessage(result)
		fmt.Fprintf(w, "go.mod:1:1: %s (unmaintained)\n", msg)

		if f.opts.FailFast {
			break
		}
	}
	return nil
}

// formatMessage creates a human-readable message for the issue
func (f *GolangciLintFormatter) formatMessage(result analyzer.Result) string {
	// Build message similar to gomodguard format
	msg := fmt.Sprintf("import of package `%s` is blocked because ", result.Package)

	switch result.Reason {
	case analyzer.ReasonArchived:
		msg += "the module is archived"
	case analyzer.ReasonNotFound:
		msg += "the module was not found"
	case analyzer.ReasonStaleInactive:
		msg += fmt.Sprintf("the module is inactive for %d days", result.DaysSinceUpdate)
	case analyzer.ReasonOutdated:
		msg += fmt.Sprintf("version %s is outdated (latest: %s)", result.CurrentVersion, result.LatestVersion)
	default:
		msg += result.Details
	}

	return msg + "."
}

// ShouldExit returns the exit code
// golangci-lint expects exit code 0 always (issues are reported via stdout)
func (f *GolangciLintFormatter) ShouldExit(results []analyzer.Result) int {
	// Always return 0 for golangci-lint integration
	return 0
}
