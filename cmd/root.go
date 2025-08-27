package cmd

import (
	"context"
	"fmt"
	"os"
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
	rootCmd.Flags().StringVar(&token, "token", "", "GitHub token (can also use GITHUB_TOKEN env var)")

	// Analysis configuration
	rootCmd.Flags().IntVar(&maxAge, "max-age", 365, "Age in days that a repository must not exceed to be considered current")
	rootCmd.Flags().BoolVar(&checkOutdated, "check-outdated", false, "Check if dependencies are using outdated versions")
	rootCmd.Flags().BoolVar(&resolveUnknown, "resolve-unknown", false, "Try to resolve and check status of non-GitHub dependencies")
	rootCmd.Flags().IntVar(&resolverTimeout, "resolver-timeout", 10, "Timeout in seconds for resolving non-GitHub dependencies")

	// Output options
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output JSON format")
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
		token = os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return fmt.Errorf("GitHub token is required. Set GITHUB_TOKEN environment variable or use --token flag")
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

	if verbose {
		fmt.Printf("Analyzing project: %s\n", mod.Path)
		fmt.Printf("Go version: %s\n", mod.GoVersion)
		fmt.Printf("Dependencies: %d\n\n", len(mod.Dependencies))
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

	// Show progress indicator for async mode (default)
	if !syncMode && !jsonOutput {
		fmt.Printf("🚀 Running concurrent analysis with %d workers...\n\n", concurrency)
	}

	results, err := analyze.AnalyzeModule(ctx, mod)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Output results
	if jsonOutput {
		return outputJSON(results)
	}

	return outputConsole(results)
}

func analyzeSinglePackage(pkg string) error {
	// TODO: Implement single package analysis
	return fmt.Errorf("single package analysis not yet implemented")
}

func outputJSON(results []analyzer.Result) error {
	// TODO: Implement JSON output
	return fmt.Errorf("JSON output not yet implemented")
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

	fmt.Println("Dependency Analysis Results:")
	fmt.Println("============================")

	// Show unmaintained packages first (most important)
	if len(unmaintained) > 0 {
		fmt.Printf("\n🚨 UNMAINTAINED PACKAGES (%d found):\n", len(unmaintained))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, result := range unmaintained {
			fmt.Printf("❌ %s - %s\n", result.Package, result.Details)
			if result.DaysSinceUpdate > 0 {
				fmt.Printf("   Last updated: %d days ago\n", result.DaysSinceUpdate)
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
			fmt.Printf("✅ %s - %s\n", result.Package, result.Details)
		}
	}

	// Print summary
	summary := analyzer.GetSummary(results)
	fmt.Print("\n" + strings.Repeat("═", 50) + "\n")
	fmt.Print("📊 ANALYSIS SUMMARY\n")
	fmt.Print(strings.Repeat("═", 50) + "\n")
	fmt.Printf("Total dependencies analyzed: %d\n\n", summary.TotalDependencies)

	if summary.UnmaintainedCount > 0 {
		fmt.Printf("🚨 UNMAINTAINED PACKAGES: %d\n", summary.UnmaintainedCount)
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
