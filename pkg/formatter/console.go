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
	c    Colorizer
}

// Format writes results in human-readable console format
func (f *ConsoleFormatter) Format(w io.Writer, results []analyzer.Result, summary analyzer.SummaryStats) error {
	f.c = Colorizer{Enabled: f.opts.Color}

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

	fmt.Fprintln(w, f.c.Bold("Dependency Analysis Results"))
	fmt.Fprintln(w, f.c.Dim(strings.Repeat("═", 27)))

	// Split by dependency type: direct dependencies can be fixed in go.mod,
	// while indirect ones require updating the parent package. Showing them
	// under separate headings makes remediation obvious.
	var direct, indirect []analyzer.Result
	for _, result := range unmaintained {
		if result.IsDirect {
			direct = append(direct, result)
		} else {
			indirect = append(indirect, result)
		}
	}

	sections := []struct {
		title string
		hint  string
		items []analyzer.Result
	}{
		{
			title: "UNMAINTAINED — DIRECT DEPENDENCIES (%d)",
			hint:  "Listed in your go.mod; upgrade or replace these directly.",
			items: direct,
		},
		{
			title: "UNMAINTAINED — INDIRECT DEPENDENCIES (%d)",
			hint:  "Pulled in by your dependencies; fix the parent package.",
			items: indirect,
		},
	}

	shown := 0
	for _, section := range sections {
		if len(section.items) == 0 {
			continue
		}
		// FailFast shows only the first (most severe) unmaintained package
		if f.opts.FailFast && shown > 0 {
			break
		}

		fmt.Fprintln(w)
		fmt.Fprintln(w, f.c.Red(fmt.Sprintf(section.title, len(section.items))))
		fmt.Fprintln(w, f.c.Dim(strings.Repeat("─", 44)))
		fmt.Fprintln(w, f.c.Dim(section.hint))

		for _, result := range section.items {
			if f.opts.FailFast && shown > 0 {
				break
			}
			f.writeUnmaintained(w, result)
			shown++
		}
	}

	// Show unknown status packages (informational)
	if len(unknown) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, f.c.Yellow(fmt.Sprintf("UNKNOWN STATUS (%d)", len(unknown))))
		fmt.Fprintln(w, f.c.Dim(strings.Repeat("─", 44)))
		for _, result := range unknown {
			fmt.Fprintf(w, "\n  %s — %s\n", f.c.Yellow("? "+result.Package), result.Details)
		}
	}

	// Show maintained packages only in verbose mode
	if f.opts.Verbose && len(maintained) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, f.c.Green(fmt.Sprintf("MAINTAINED PACKAGES (%d)", len(maintained))))
		fmt.Fprintln(w, f.c.Dim(strings.Repeat("─", 44)))
		for _, result := range maintained {
			fmt.Fprintf(w, "\n  %s — %s\n", f.c.Green("✓ "+result.Package), result.Details)

			// Show retraction warning even for maintained packages
			if result.IsRetracted {
				fmt.Fprintln(w, "     "+f.c.Yellow("VERSION RETRACTED"))
				if result.RetractionReason != "" {
					fmt.Fprintf(w, "     Reason: %s\n", result.RetractionReason)
				}
			}

			f.writeRepoLine(w, result)
		}
	}

	// Print summary
	fmt.Fprint(w, "\n"+f.c.Dim(strings.Repeat("═", 50))+"\n")
	fmt.Fprintln(w, f.c.Bold("ANALYSIS SUMMARY"))
	fmt.Fprint(w, f.c.Dim(strings.Repeat("═", 50))+"\n")
	fmt.Fprintf(w, "Total dependencies analyzed: %d\n", summary.TotalDependencies)

	if summary.UnmaintainedCount > 0 {
		writeUnmaintainedSummary(w, summary, f.c)
	}

	if summary.UnknownCount > 0 {
		fmt.Fprintf(w, "\n%s\n", f.c.Yellow(fmt.Sprintf("Unknown status: %d", summary.UnknownCount)))
		fmt.Fprintln(w, f.c.Dim("   (Non-GitHub dependencies that couldn't be fully analyzed)"))
	}

	if summary.RetractedCount > 0 {
		fmt.Fprintf(w, "\n%s\n", f.c.Yellow(fmt.Sprintf("Retracted versions: %d", summary.RetractedCount)))
		fmt.Fprintln(w, f.c.Dim("   (Module authors marked these versions as problematic)"))
	}

	maintainedCount := summary.TotalDependencies - summary.UnmaintainedCount - summary.UnknownCount
	if maintainedCount > 0 {
		fmt.Fprintf(w, "\n%s\n", f.c.Green(fmt.Sprintf("Maintained: %d", maintainedCount)))
		fmt.Fprintln(w, f.c.Dim("   (Active repositories with recent updates)"))
	}

	return nil
}

// writeUnmaintainedSummary writes the unmaintained breakdown of the summary
func writeUnmaintainedSummary(w io.Writer, summary analyzer.SummaryStats, c Colorizer) {
	count := fmt.Sprintf("Unmaintained: %d", summary.UnmaintainedCount)
	if summary.DirectUnmaintained > 0 || summary.IndirectUnmaintained > 0 {
		count += fmt.Sprintf(" (%d direct, %d indirect)", summary.DirectUnmaintained, summary.IndirectUnmaintained)
	}
	fmt.Fprintf(w, "\n%s\n", c.Red(count))

	if summary.ArchivedCount > 0 {
		fmt.Fprintf(w, "   Archived repositories: %d\n", summary.ArchivedCount)
	}
	if summary.NotFoundCount > 0 {
		fmt.Fprintf(w, "   Not found/deleted: %d\n", summary.NotFoundCount)
	}
	if summary.StaleInactiveCount > 0 {
		fmt.Fprintf(w, "   Stale/inactive: %d\n", summary.StaleInactiveCount)
	}
	if summary.OutdatedCount > 0 {
		fmt.Fprintf(w, "   Outdated versions: %d\n", summary.OutdatedCount)
	}
}

// writeUnmaintained writes a single unmaintained result with its details
func (f *ConsoleFormatter) writeUnmaintained(w io.Writer, result analyzer.Result) {
	fmt.Fprintf(w, "\n  %s — %s\n", f.c.Red("✗ "+result.Package), result.Details)

	// Show retraction warning if applicable
	if result.IsRetracted {
		fmt.Fprintln(w, "     "+f.c.Yellow("VERSION RETRACTED"))
		if result.RetractionReason != "" {
			fmt.Fprintf(w, "     Reason: %s\n", result.RetractionReason)
		}
	}

	f.writeRepoLine(w, result)

	// Show last activity information with context. "No commits" and "last
	// activity" are deliberately labeled differently: a commit is a push to
	// the default branch, while activity also covers releases, issues, and
	// other repository events.
	if result.RepoInfo != nil {
		if result.RepoInfo.LastCommitAt != nil {
			daysSinceCommit := int(time.Since(*result.RepoInfo.LastCommitAt).Hours() / 24)
			fmt.Fprintf(w, "     No commits in %d days\n", daysSinceCommit)
		} else if result.DaysSinceUpdate > 0 {
			fmt.Fprintf(w, "     Last activity: %d days ago (includes non-commit events)\n", result.DaysSinceUpdate)
		}

		// For archived repos, note that they're archived
		if result.RepoInfo.IsArchived {
			fmt.Fprintln(w, "     "+f.c.Red("Archived: no new commits are possible"))
		}
	}

	// Show dependency path for indirect dependencies
	if f.opts.ShowPaths && !result.IsDirect && len(result.DependencyPath) > 0 {
		fmt.Fprintf(w, "     Required by: %s\n", f.c.Dim(strings.Join(result.DependencyPath, " → ")))
	}
}

// writeRepoLine writes the repository reference as a short slug with the full
// URL in parentheses, e.g. "Repo: owner/repo (https://github.com/owner/repo)"
func (f *ConsoleFormatter) writeRepoLine(w io.Writer, result analyzer.Result) {
	url := GetRepositoryURL(result)
	if url == "" {
		return
	}
	fmt.Fprintf(w, "     Repo: %s (%s)\n", f.c.Dim(repoSlug(url)), url)
}

// repoSlug extracts a short "owner/repo" slug from a repository URL
func repoSlug(url string) string {
	trimmed := strings.TrimPrefix(url, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimSuffix(trimmed, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return trimmed
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
