package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
	"github.com/johnsaigle/go-unmaintained/pkg/formatter"
	"github.com/johnsaigle/go-unmaintained/pkg/parser"
)

var (
	// Flags
	targetPath      string
	packageName     string
	token           string
	maxAge          int
	outputFormat    string
	jsonOutput      bool // Deprecated: use --format=json
	githubActions   bool // Deprecated: use --format=github-actions
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
using heuristics.

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
	rootCmd.Flags().StringVar(&token, "token", "", "GitHub token (can also use PAT env var)")

	// Analysis configuration
	rootCmd.Flags().IntVar(&maxAge, "max-age", 365, "Age in days that a repository must not exceed to be considered current")
	rootCmd.Flags().BoolVar(&checkOutdated, "check-outdated", false, "Check if dependencies are using outdated versions")
	rootCmd.Flags().BoolVar(&resolveUnknown, "resolve-unknown", false, "Try to resolve and check status of non-GitHub dependencies")
	rootCmd.Flags().IntVar(&resolverTimeout, "resolver-timeout", 10, "Timeout in seconds for resolving non-GitHub dependencies")

	// Output options
	rootCmd.Flags().StringVar(&outputFormat, "format", "console", "Output format: console, json, github-actions, golangci-lint")
	rootCmd.Flags().BoolVar(&githubActions, "github-actions", false, "Output GitHub Actions annotations format (deprecated: use --format=github-actions)")
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

	// Determine output format (handle legacy flags)
	format := determineFormat()

	// Always show startup message for non-machine-readable formats
	if format == "console" && !jsonOutput {
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

	// Fetch dependency paths for indirect unmaintained dependencies if tree is enabled
	if tree {
		for i := range results {
			if results[i].IsUnmaintained && !results[i].IsDirect {
				depPath, pathErr := parser.GetDependencyPath(ctx, mod.ProjectPath, results[i].Package)
				if pathErr == nil && len(depPath) > 0 {
					results[i].DependencyPath = depPath
				}
			}
		}
	}

	// Get summary
	summary := analyzer.GetSummary(results)

	// Create formatter
	fmtOpts := formatter.Options{
		Verbose:    verbose,
		ShowPaths:  tree,
		FailFast:   failFast,
		NoExitCode: noExitCode,
	}

	fmtr, err := formatter.New(format, fmtOpts)
	if err != nil {
		return fmt.Errorf("failed to create formatter: %w", err)
	}

	// Format output
	if err := fmtr.Format(os.Stdout, results, summary); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Exit with appropriate code
	os.Exit(fmtr.ShouldExit(results))
	return nil
}

// determineFormat returns the output format based on flags
// Handles legacy flags for backwards compatibility
func determineFormat() string {
	// Legacy flags take precedence for backwards compatibility
	if jsonOutput {
		return "json"
	}
	if githubActions {
		return "github-actions"
	}
	return outputFormat
}

func analyzeSinglePackage(pkg string) error {
	// TODO: Implement single package analysis
	return fmt.Errorf("single package analysis not yet implemented")
}
