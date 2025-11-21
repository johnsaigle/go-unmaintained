package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
	"github.com/johnsaigle/go-unmaintained/pkg/parser"
)

var (
	// Flags
	targetPath      string
	packageName     string
	token           string
	maxAge          int
	jsonOutput      bool
	githubActions   bool
	verbose         bool
	noCache         bool
	failFast        bool
	tree            bool
	colorOutput     string
	noWarnings      bool
	noExitCode      bool
	checkOutdated   bool
	cacheDurationHr int
	resolveUnknown  bool
	resolverTimeout int
	syncMode        bool
	concurrency     int

	rootCmd = &cobra.Command{
		Use:   "go-unmaintained",
		Short: "Find unmaintained packages in Go projects",
		Long: `go-unmaintained is a CLI tool that automatically identifies unmaintained Go packages 
using heuristics, similar to cargo-unmaintained for the Rust ecosystem.

It analyzes go.mod files and their dependencies to detect packages that may pose 
security or reliability risks due to lack of maintenance.

Features:
• Multi-platform support (GitHub, GitLab, Bitbucket)
• Smart caching for performance
• Concurrent analysis by default for speed
• Intelligent rate limiting
• Clear categorization of results`,
		RunE: runAnalysis,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Target and input flags
	rootCmd.Flags().StringVar(&targetPath, "target", ".", "Path to Go project directory")
	rootCmd.Flags().StringVarP(&packageName, "package", "p", "", "Analyze single package instead of project")

	// Authentication
	rootCmd.Flags().StringVar(&token, "token", "", "GitHub token (can also use PAT env var)")

	// Analysis configuration
	rootCmd.Flags().IntVar(&maxAge, "max-age", 365, "Age in days that a repository must not exceed to be considered current")
	rootCmd.Flags().BoolVar(&checkOutdated, "check-outdated", false, "Check if dependencies are using outdated versions")
	rootCmd.Flags().BoolVar(&resolveUnknown, "resolve-unknown", false, "Try to resolve and check status of non-GitHub dependencies")
	rootCmd.Flags().IntVar(&resolverTimeout, "resolver-timeout", 10, "Timeout in seconds for resolving non-GitHub dependencies")

	// Output options
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON format")
	rootCmd.Flags().BoolVar(&githubActions, "github-actions", false, "Output GitHub Actions annotations format")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed information")
	rootCmd.Flags().BoolVar(&tree, "tree", false, "Show dependency tree paths")
	rootCmd.Flags().StringVar(&colorOutput, "color", "auto", "When to use color: always, auto, or never")
	rootCmd.Flags().BoolVar(&noWarnings, "no-warnings", false, "Do not show warnings")
	rootCmd.Flags().BoolVar(&noExitCode, "no-exit-code", false, "Do not set exit code when unmaintained packages are found")

	// Performance and caching
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Do not cache data on disk")
	rootCmd.Flags().IntVar(&cacheDurationHr, "cache-duration", 24, "Cache duration in hours")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "Exit as soon as an unmaintained package is found")
	rootCmd.Flags().BoolVar(&syncMode, "sync", false, "Disable async mode and use sequential processing (slower)")
	rootCmd.Flags().IntVar(&concurrency, "concurrency", 5, "Number of concurrent requests (default: 5)")
}

func runAnalysis(cmd *cobra.Command, args []string) error {
	// Get GitHub token from environment if not provided
	if token == "" {
		token = os.Getenv("PAT")
		if token == "" {
			return fmt.Errorf("GitHub token is required. Set PAT environment variable or use --token flag")
		}
	}

	// Handle single package analysis
	if packageName != "" {
		return analyzeSinglePackage(packageName)
	}

	// Handle project analysis
	return analyzeProject(targetPath)
}

func analyzeProject(projectPath string) error {
	// Parse go.mod file
	mod, err := parser.ParseGoMod(projectPath)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}

	// Always show startup message (not just in verbose mode)
	if !jsonOutput {
		fmt.Printf("📦 Project: %s\n", mod.Path)
		fmt.Printf("🔍 Analyzing %d dependencies", len(mod.Dependencies))

		// Show mode indicator
		if !syncMode {
			fmt.Printf(" (concurrent: %d workers)", concurrency)
		} else {
			fmt.Printf(" (sequential mode)")
		}
		fmt.Println("...")

		if verbose {
			fmt.Printf("   Go version: %s\n", mod.GoVersion)
		}
		fmt.Println()
	}

	// Create analyzer
	config := analyzer.Config{
		MaxAge:          time.Duration(maxAge) * 24 * time.Hour,
		Token:           token,
		Verbose:         verbose,
		CheckOutdated:   checkOutdated,
		NoCache:         noCache,
		CacheDuration:   time.Duration(cacheDurationHr) * time.Hour,
		ResolveUnknown:  resolveUnknown,
		ResolverTimeout: time.Duration(resolverTimeout) * time.Second,
		AsyncMode:       !syncMode,
		Concurrency:     concurrency,
	}

	analyze, err := analyzer.NewAnalyzer(config)
	if err != nil {
		return fmt.Errorf("failed to create analyzer: %w", err)
	}

	// Analyze dependencies
	ctx := context.Background()

	results, err := analyze.AnalyzeModule(ctx, mod)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Fetch dependency paths for indirect unmaintained dependencies
	for i := range results {
		if results[i].IsUnmaintained && !results[i].IsDirect {
			depPath, err := parser.GetDependencyPath(ctx, mod.ProjectPath, results[i].Package)
			if err == nil && len(depPath) > 0 {
				results[i].DependencyPath = depPath
			}
		}
	}

	// Output results
	if jsonOutput {
		return outputJSON(results)
	}

	if githubActions {
		return outputGitHubActions(results)
	}

	return outputConsole(results)
}

func outputGitHubActions(results []analyzer.Result) error {
	// Output GitHub Actions workflow commands
	// https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions

	for _, result := range results {
		if result.IsUnmaintained {
			depType := "indirect"
			if result.IsDirect {
				depType = "direct"
			}

			// Determine severity
			severity := "error"
			if result.Reason == "stale_dependencies_inactive_repo" {
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
			fmt.Printf("::%s file=go.mod,title=Unmaintained Dependency::%s\n", severity, message)

			// For indirect dependencies, add additional context
			if !result.IsDirect && len(result.DependencyPath) > 0 {
				pathStr := strings.Join(result.DependencyPath, " → ")
				fmt.Printf("::notice file=go.mod,title=Dependency Path::%s\n", pathStr)
			}
		}
	}

	// Output summary
	summary := analyzer.GetSummary(results)
	if summary.UnmaintainedCount > 0 {
		fmt.Printf("::warning::Found %d unmaintained packages (%d direct, %d indirect)\n",
			summary.UnmaintainedCount, summary.DirectUnmaintained, summary.IndirectUnmaintained)
	} else {
		fmt.Printf("::notice::All dependencies are maintained\n")
	}

	return nil
}

func analyzeSinglePackage(pkg string) error {
	// TODO: Implement single package analysis
	return fmt.Errorf("single package analysis not yet implemented")
}

// JSONOutput represents the JSON output structure
type JSONOutput struct {
	Summary   analyzer.SummaryStats `json:"summary"`
	Results   []JSONResult          `json:"results"`
	Timestamp time.Time             `json:"timestamp"`
	Version   string                `json:"version"`
}

// JSONResult represents a single dependency result in JSON format
type JSONResult struct {
	Package         string        `json:"package"`
	IsUnmaintained  bool          `json:"is_unmaintained"`
	IsDirect        bool          `json:"is_direct"`
	Reason          string        `json:"reason,omitempty"`
	Details         string        `json:"details"`
	CurrentVersion  string        `json:"current_version,omitempty"`
	LatestVersion   string        `json:"latest_version,omitempty"`
	DaysSinceUpdate int           `json:"days_since_update,omitempty"`
	DependencyPath  []string      `json:"dependency_path,omitempty"`
	RepoInfo        *JSONRepoInfo `json:"repo_info,omitempty"`
}

// JSONRepoInfo represents repository information in JSON format
type JSONRepoInfo struct {
	URL            string    `json:"url,omitempty"`
	IsArchived     bool      `json:"is_archived"`
	LastCommitDays int       `json:"last_commit_days,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

func outputJSON(results []analyzer.Result) error {
	summary := analyzer.GetSummary(results)

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

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
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
	case "repository_archived":
		baseScore = 0
	case "package_not_found":
		baseScore = 10
	case "stale_dependencies_inactive_repo":
		baseScore = 20
	case "outdated_version":
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

func outputConsole(results []analyzer.Result) error {
	unmaintainedFound := false

	// Separate results into categories
	var unmaintained []analyzer.Result
	var unknown []analyzer.Result
	var maintained []analyzer.Result

	for _, result := range results {
		if result.IsUnmaintained {
			unmaintained = append(unmaintained, result)
			unmaintainedFound = true
		} else if result.Reason == "unknown_source" {
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

	fmt.Println("Dependency Analysis Results:")
	fmt.Println("============================")

	// Show unmaintained packages first (most important)
	if len(unmaintained) > 0 {
		fmt.Printf("\n🚨 UNMAINTAINED PACKAGES (%d found):\n", len(unmaintained))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, result := range unmaintained {
			// Show dependency type
			depType := "indirect"
			if result.IsDirect {
				depType = "direct"
			}

			fmt.Printf("❌ %s (%s) - %s\n", result.Package, depType, result.Details)

			// Show repository URL for verification
			if url := getRepositoryURL(result); url != "" {
				fmt.Printf("   🔗 %s\n", url)
			}

			// Show last activity information with context
			if result.RepoInfo != nil {
				if result.RepoInfo.LastCommitAt != nil {
					daysSinceCommit := int(time.Since(*result.RepoInfo.LastCommitAt).Hours() / 24)
					fmt.Printf("   Last commit: %d days ago\n", daysSinceCommit)
				} else if result.DaysSinceUpdate > 0 {
					// Fall back to UpdatedAt if no commit info available
					fmt.Printf("   Last activity: %d days ago\n", result.DaysSinceUpdate)
				}

				// For archived repos, note that they're archived (archive date not available from API)
				if result.RepoInfo.IsArchived {
					fmt.Printf("   ⚠️  Repository archived (no new commits possible)\n")
				}
			}

			// Show dependency path for indirect dependencies
			if !result.IsDirect && len(result.DependencyPath) > 0 {
				fmt.Printf("   📍 Dependency path: %s\n", strings.Join(result.DependencyPath, " → "))
			}

			if failFast {
				break
			}
		}
	}

	// Show unknown status packages (informational)
	if len(unknown) > 0 {
		fmt.Printf("\n❓ UNKNOWN STATUS PACKAGES (%d found):\n", len(unknown))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, result := range unknown {
			fmt.Printf("❓ %s - %s\n", result.Package, result.Details)
		}
	}

	// Show maintained packages only in verbose mode
	if verbose && len(maintained) > 0 {
		fmt.Printf("\n✅ MAINTAINED PACKAGES (%d found):\n", len(maintained))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, result := range maintained {
			// Show dependency type in verbose mode
			depType := "indirect"
			if result.IsDirect {
				depType = "direct"
			}

			fmt.Printf("✅ %s (%s) - %s\n", result.Package, depType, result.Details)

			// Show URL in verbose mode for verification
			if url := getRepositoryURL(result); url != "" {
				fmt.Printf("   🔗 %s\n", url)
			}
		}
	}

	// Print summary
	summary := analyzer.GetSummary(results)
	fmt.Print("\n" + strings.Repeat("═", 50) + "\n")
	fmt.Print("📊 ANALYSIS SUMMARY\n")
	fmt.Print(strings.Repeat("═", 50) + "\n")
	fmt.Printf("Total dependencies analyzed: %d\n\n", summary.TotalDependencies)

	if summary.UnmaintainedCount > 0 {
		fmt.Printf("🚨 UNMAINTAINED PACKAGES: %d", summary.UnmaintainedCount)
		if summary.DirectUnmaintained > 0 || summary.IndirectUnmaintained > 0 {
			fmt.Printf(" (%d direct, %d indirect)", summary.DirectUnmaintained, summary.IndirectUnmaintained)
		}
		fmt.Println()

		if summary.ArchivedCount > 0 {
			fmt.Printf("   📦 Archived repositories: %d\n", summary.ArchivedCount)
		}
		if summary.NotFoundCount > 0 {
			fmt.Printf("   🚫 Not found/deleted: %d\n", summary.NotFoundCount)
		}
		if summary.StaleInactiveCount > 0 {
			fmt.Printf("   💤 Stale/Inactive: %d\n", summary.StaleInactiveCount)
		}
		if checkOutdated && summary.OutdatedCount > 0 {
			fmt.Printf("   📅 Outdated versions: %d\n", summary.OutdatedCount)
		}
		fmt.Println()
	}

	if summary.UnknownCount > 0 {
		fmt.Printf("❓ UNKNOWN STATUS: %d\n", summary.UnknownCount)
		fmt.Printf("   (Non-GitHub dependencies that couldn't be fully analyzed)\n\n")
	}

	maintainedCount := summary.TotalDependencies - summary.UnmaintainedCount - summary.UnknownCount
	if maintainedCount > 0 {
		fmt.Printf("✅ MAINTAINED PACKAGES: %d\n", maintainedCount)
		fmt.Printf("   (Active repositories with recent updates)\n")
	}

	// Set exit code
	if unmaintainedFound && !noExitCode {
		os.Exit(1)
	}

	return nil
}
