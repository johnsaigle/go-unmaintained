package cmd

import (
	"context"
	"fmt"
	"os"
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

	rootCmd = &cobra.Command{
		Use:   "go-unmaintained",
		Short: "Find unmaintained packages in Go projects",
		Long: `go-unmaintained is a CLI tool that automatically identifies unmaintained Go packages 
using heuristics, similar to cargo-unmaintained for the Rust ecosystem.

It analyzes go.mod files and their dependencies to detect packages that may pose 
security or reliability risks due to lack of maintenance.`,
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

	fmt.Println("Dependency Analysis Results:")
	fmt.Println("============================")

	for _, result := range results {
		if result.IsUnmaintained {
			unmaintainedFound = true
			fmt.Printf("❌ %s - %s\n", result.Package, result.Details)
			if result.DaysSinceUpdate > 0 {
				fmt.Printf("   Last updated: %d days ago\n", result.DaysSinceUpdate)
			}
		} else if result.Reason == "unknown_source" {
			fmt.Printf("❓ %s - %s\n", result.Package, result.Details)
		} else if verbose {
			fmt.Printf("✅ %s - %s\n", result.Package, result.Details)
		}

		if failFast && result.IsUnmaintained {
			break
		}
	}

	// Print summary
	summary := analyzer.GetSummary(results)
	fmt.Printf("\nSummary:\n")
	fmt.Printf("Total dependencies: %d\n", summary.TotalDependencies)
	fmt.Printf("Unmaintained: %d\n", summary.UnmaintainedCount)
	if summary.UnmaintainedCount > 0 {
		fmt.Printf("  - Archived: %d\n", summary.ArchivedCount)
		fmt.Printf("  - Not found: %d\n", summary.NotFoundCount)
		fmt.Printf("  - Stale/Inactive: %d\n", summary.StaleInactiveCount)
		if checkOutdated {
			fmt.Printf("  - Outdated: %d\n", summary.OutdatedCount)
		}
	}
	if summary.UnknownCount > 0 {
		fmt.Printf("Unknown status: %d\n", summary.UnknownCount)
	}

	// Set exit code
	if unmaintainedFound && !noExitCode {
		os.Exit(1)
	}

	return nil
}
