package formatter

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
)

// ConsoleFormatter formats output for human-readable console display
type ConsoleFormatter struct {
	opts Options
}

// Format writes results in human-readable console format
func (f *ConsoleFormatter) Format(w io.Writer, results []analyzer.Result, summary analyzer.SummaryStats) error {
	// Separate results into categories
	var unmaintained []analyzer.Result
	var unknown []analyzer.Result
	var maintained []analyzer.Result

	for _, result := range results {
		if result.IsUnmaintained {
			unmaintained = append(unmaintained, result)
		} else if result.Reason == analyzer.ReasonUnknown {
			unknown = append(unknown, result)
		} else {
			maintained = append(maintained, result)
		}
	}

	// Sort unmaintained by severity (most critical first)
	sort.Slice(unmaintained, func(i, j int) bool {
		scoreI := getSeverityScore(unmaintained[i])
		scoreJ := getSeverityScore(unmaintained[j])

		// If same severity, sort alphabetically by package name
		if scoreI == scoreJ {
			return unmaintained[i].Package < unmaintained[j].Package
		}

		return scoreI < scoreJ
	})

	fmt.Fprintln(w, "Dependency Analysis Results:")
	fmt.Fprintln(w, "============================")

	// Show unmaintained packages first (most important)
	if len(unmaintained) > 0 {
		fmt.Fprintf(w, "\n🚨 UNMAINTAINED PACKAGES (%d found):\n", len(unmaintained))
		fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, result := range unmaintained {
			// Show dependency type
			depType := "indirect"
			if result.IsDirect {
				depType = "direct"
			}

			fmt.Fprintf(w, "❌ %s (%s) - %s\n", result.Package, depType, result.Details)

			// Show repository URL for verification
			if url := getRepositoryURL(result); url != "" {
				fmt.Fprintf(w, "   🔗 %s\n", url)
			}

			// Show last activity information with context
			if result.RepoInfo != nil {
				if result.RepoInfo.LastCommitAt != nil {
					daysSinceCommit := int(time.Since(*result.RepoInfo.LastCommitAt).Hours() / 24)
					fmt.Fprintf(w, "   Last commit: %d days ago\n", daysSinceCommit)
				} else if result.DaysSinceUpdate > 0 {
					// Fall back to UpdatedAt if no commit info available
					fmt.Fprintf(w, "   Last activity: %d days ago\n", result.DaysSinceUpdate)
				}

				// For archived repos, note that they're archived
				if result.RepoInfo.IsArchived {
					fmt.Fprintln(w, "   ⚠️  Repository archived (no new commits possible)")
				}
			}

			// Show dependency path for indirect dependencies
			if f.opts.ShowPaths && !result.IsDirect && len(result.DependencyPath) > 0 {
				fmt.Fprintf(w, "   📍 Dependency path: %s\n", strings.Join(result.DependencyPath, " → "))
			}

			if f.opts.FailFast {
				break
			}
		}
	}

	// Show unknown status packages (informational)
	if len(unknown) > 0 {
		fmt.Fprintf(w, "\n❓ UNKNOWN STATUS PACKAGES (%d found):\n", len(unknown))
		fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, result := range unknown {
			fmt.Fprintf(w, "❓ %s - %s\n", result.Package, result.Details)
		}
	}

	// Show maintained packages only in verbose mode
	if f.opts.Verbose && len(maintained) > 0 {
		fmt.Fprintf(w, "\n✅ MAINTAINED PACKAGES (%d found):\n", len(maintained))
		fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, result := range maintained {
			// Show dependency type in verbose mode
			depType := "indirect"
			if result.IsDirect {
				depType = "direct"
			}

			fmt.Fprintf(w, "✅ %s (%s) - %s\n", result.Package, depType, result.Details)

			// Show URL in verbose mode for verification
			if url := getRepositoryURL(result); url != "" {
				fmt.Fprintf(w, "   🔗 %s\n", url)
			}
		}
	}

	// Print summary
	fmt.Fprint(w, "\n"+strings.Repeat("═", 50)+"\n")
	fmt.Fprintln(w, "📊 ANALYSIS SUMMARY")
	fmt.Fprint(w, strings.Repeat("═", 50)+"\n")
	fmt.Fprintf(w, "Total dependencies analyzed: %d\n\n", summary.TotalDependencies)

	if summary.UnmaintainedCount > 0 {
		fmt.Fprintf(w, "🚨 UNMAINTAINED PACKAGES: %d", summary.UnmaintainedCount)
		if summary.DirectUnmaintained > 0 || summary.IndirectUnmaintained > 0 {
			fmt.Fprintf(w, " (%d direct, %d indirect)", summary.DirectUnmaintained, summary.IndirectUnmaintained)
		}
		fmt.Fprintln(w)

		if summary.ArchivedCount > 0 {
			fmt.Fprintf(w, "   📦 Archived repositories: %d\n", summary.ArchivedCount)
		}
		if summary.NotFoundCount > 0 {
			fmt.Fprintf(w, "   🚫 Not found/deleted: %d\n", summary.NotFoundCount)
		}
		if summary.StaleInactiveCount > 0 {
			fmt.Fprintf(w, "   💤 Stale/Inactive: %d\n", summary.StaleInactiveCount)
		}
		if summary.OutdatedCount > 0 {
			fmt.Fprintf(w, "   📅 Outdated versions: %d\n", summary.OutdatedCount)
		}
		fmt.Fprintln(w)
	}

	if summary.UnknownCount > 0 {
		fmt.Fprintf(w, "❓ UNKNOWN STATUS: %d\n", summary.UnknownCount)
		fmt.Fprintln(w, "   (Non-GitHub dependencies that couldn't be fully analyzed)\n")
	}

	maintainedCount := summary.TotalDependencies - summary.UnmaintainedCount - summary.UnknownCount
	if maintainedCount > 0 {
		fmt.Fprintf(w, "✅ MAINTAINED PACKAGES: %d\n", maintainedCount)
		fmt.Fprintln(w, "   (Active repositories with recent updates)")
	}

	return nil
}

// ShouldExit returns the exit code based on results
func (f *ConsoleFormatter) ShouldExit(results []analyzer.Result) int {
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

// getRepositoryURL extracts or constructs a repository URL from the result
func getRepositoryURL(result analyzer.Result) string {
	// Try to use the URL from RepoInfo first
	if result.RepoInfo != nil && result.RepoInfo.URL != "" {
		return result.RepoInfo.URL
	}

	// Try to construct URL from package path for GitHub repos
	if strings.HasPrefix(result.Package, "github.com/") {
		parts := strings.Split(result.Package, "/")
		if len(parts) >= 3 {
			return fmt.Sprintf("https://github.com/%s/%s", parts[1], parts[2])
		}
	}

	// Try for GitLab repos
	if strings.HasPrefix(result.Package, "gitlab.com/") {
		parts := strings.Split(result.Package, "/")
		if len(parts) >= 3 {
			return fmt.Sprintf("https://gitlab.com/%s/%s", parts[1], parts[2])
		}
	}

	// Try for Bitbucket repos
	if strings.HasPrefix(result.Package, "bitbucket.org/") {
		parts := strings.Split(result.Package, "/")
		if len(parts) >= 3 {
			return fmt.Sprintf("https://bitbucket.org/%s/%s", parts[1], parts[2])
		}
	}

	return ""
}

// getSeverityScore returns a score for sorting (lower = more severe)
func getSeverityScore(result analyzer.Result) int {
	// Priority order:
	// 1. Direct + Archived (most critical)
	// 2. Direct + Not Found
	// 3. Direct + Stale/Inactive
	// 4. Direct + Outdated
	// 5. Indirect + Archived
	// 6. Indirect + Not Found
	// 7. Indirect + Stale/Inactive
	// 8. Indirect + Outdated

	baseScore := 0

	// Reason severity
	switch result.Reason {
	case analyzer.ReasonArchived:
		baseScore = 0
	case analyzer.ReasonNotFound:
		baseScore = 10
	case analyzer.ReasonStaleInactive:
		baseScore = 20
	case analyzer.ReasonOutdated:
		baseScore = 30
	default:
		baseScore = 40
	}

	// Add penalty for indirect dependencies
	if !result.IsDirect {
		baseScore += 50
	}

	return baseScore
}
