package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/github"
	"github.com/johnsaigle/go-unmaintained/pkg/parser"
)

// UnmaintainedReason represents why a package is considered unmaintained
type UnmaintainedReason string

const (
	ReasonArchived      UnmaintainedReason = "repository_archived"
	ReasonNotFound      UnmaintainedReason = "package_not_found"
	ReasonStaleInactive UnmaintainedReason = "stale_dependencies_inactive_repo"
)

// Result represents the analysis result for a single dependency
type Result struct {
	Package         string
	IsUnmaintained  bool
	Reason          UnmaintainedReason
	RepoInfo        *github.RepoInfo
	DaysSinceUpdate int
	Details         string
}

// Config holds configuration for the analyzer
type Config struct {
	MaxAge  time.Duration
	Token   string
	Verbose bool
}

// Analyzer performs unmaintained package analysis
type Analyzer struct {
	config       Config
	githubClient *github.Client
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer(config Config) (*Analyzer, error) {
	githubClient, err := github.NewClient(config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	return &Analyzer{
		config:       config,
		githubClient: githubClient,
	}, nil
}

// AnalyzeModule analyzes all dependencies in a module
func (a *Analyzer) AnalyzeModule(ctx context.Context, mod *parser.Module) ([]Result, error) {
	var results []Result

	for _, dep := range mod.Dependencies {
		result, err := a.AnalyzeDependency(ctx, dep)
		if err != nil {
			// Log error but continue with other dependencies
			result = Result{
				Package:        dep.Path,
				IsUnmaintained: false,
				Details:        fmt.Sprintf("Analysis error: %v", err),
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// AnalyzeDependency analyzes a single dependency
func (a *Analyzer) AnalyzeDependency(ctx context.Context, dep parser.Dependency) (Result, error) {
	result := Result{
		Package: dep.Path,
	}

	// Skip replaced dependencies for now
	if dep.Replace != nil {
		result.Details = "Skipped: has replace directive"
		return result, nil
	}

	// Parse module path to get repository info
	host, owner, repo, err := parser.ParseModulePath(dep.Path)
	if err != nil {
		result.Details = fmt.Sprintf("Invalid module path: %v", err)
		return result, nil
	}

	// Currently only support GitHub
	if host != "github.com" {
		result.Details = "Skipped: non-GitHub repository"
		return result, nil
	}

	// Get repository information
	repoInfo, err := a.githubClient.GetRepositoryInfo(ctx, owner, repo)
	if err != nil {
		return result, fmt.Errorf("failed to get repository info: %w", err)
	}

	result.RepoInfo = repoInfo
	result.DaysSinceUpdate = repoInfo.DaysSinceLastActivity()

	// Apply heuristics
	if !repoInfo.Exists {
		result.IsUnmaintained = true
		result.Reason = ReasonNotFound
		result.Details = "Repository not found"
		return result, nil
	}

	if repoInfo.IsArchived {
		result.IsUnmaintained = true
		result.Reason = ReasonArchived
		result.Details = "Repository is archived"
		return result, nil
	}

	// Check if repository is inactive (simplified heuristic for now)
	if !repoInfo.IsRepositoryActive(a.config.MaxAge) {
		result.IsUnmaintained = true
		result.Reason = ReasonStaleInactive
		result.Details = fmt.Sprintf("Repository inactive for %d days", result.DaysSinceUpdate)
		return result, nil
	}

	result.Details = fmt.Sprintf("Active repository, last updated %d days ago", result.DaysSinceUpdate)
	return result, nil
}

// SummaryStats holds summary statistics
type SummaryStats struct {
	TotalDependencies  int
	UnmaintainedCount  int
	ArchivedCount      int
	NotFoundCount      int
	StaleInactiveCount int
}

// GetSummary returns summary statistics from results
func GetSummary(results []Result) SummaryStats {
	stats := SummaryStats{
		TotalDependencies: len(results),
	}

	for _, result := range results {
		if result.IsUnmaintained {
			stats.UnmaintainedCount++
			switch result.Reason {
			case ReasonArchived:
				stats.ArchivedCount++
			case ReasonNotFound:
				stats.NotFoundCount++
			case ReasonStaleInactive:
				stats.StaleInactiveCount++
			}
		}
	}

	return stats
}
