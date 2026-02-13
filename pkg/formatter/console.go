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
			// Only show truly unknown packages, not actively maintained ones
			unknown = append(unknown, result)
		} else {
			// Packages with ReasonActive or other known-good reasons
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
	//nolint:nestif // Formatting logic requires nested conditionals for different display scenarios
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

			// Show retraction warning if applicable
			if result.IsRetracted {
				fmt.Fprintln(w, "   ⚠️  VERSION RETRACTED")
				if result.RetractionReason != "" {
					fmt.Fprintf(w, "   Reason: %s\n", result.RetractionReason)
				}
			}

			// Show repository URL for verification
			if url := GetRepositoryURL(result); url != "" {
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
	//nolint:nestif // Verbose output requires nested conditionals for detailed formatting
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

			// Show retraction warning even for maintained packages
			if result.IsRetracted {
				fmt.Fprintln(w, "   ⚠️  VERSION RETRACTED")
				if result.RetractionReason != "" {
					fmt.Fprintf(w, "   Reason: %s\n", result.RetractionReason)
				}
			}

			// Show URL in verbose mode for verification
			if url := GetRepositoryURL(result); url != "" {
				fmt.Fprintf(w, "   🔗 %s\n", url)
			}
		}
	}

	// Print summary
	fmt.Fprint(w, "\n"+strings.Repeat("═", 50)+"\n")
	fmt.Fprintln(w, "📊 ANALYSIS SUMMARY")
	fmt.Fprint(w, strings.Repeat("═", 50)+"\n")
	fmt.Fprintf(w, "Total dependencies analyzed: %d\n\n", summary.TotalDependencies)

	//nolint:nestif // Summary formatting requires nested conditionals for different counts
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
		fmt.Fprintln(w, "   (Non-GitHub dependencies that couldn't be fully analyzed)")
	}

	if summary.RetractedCount > 0 {
		fmt.Fprintf(w, "\n⚠️  RETRACTED VERSIONS: %d\n", summary.RetractedCount)
		fmt.Fprintln(w, "   (Module authors marked these versions as problematic)")
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
	return DefaultShouldExit(results, f.opts.NoExitCode)
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
