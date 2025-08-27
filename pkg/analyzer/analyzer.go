package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/cache"
	"github.com/johnsaigle/go-unmaintained/pkg/github"
	"github.com/johnsaigle/go-unmaintained/pkg/parser"
	"golang.org/x/mod/semver"
)

// UnmaintainedReason represents why a package is considered unmaintained
type UnmaintainedReason string

const (
	ReasonArchived      UnmaintainedReason = "repository_archived"
	ReasonNotFound      UnmaintainedReason = "package_not_found"
	ReasonStaleInactive UnmaintainedReason = "stale_dependencies_inactive_repo"
	ReasonOutdated      UnmaintainedReason = "outdated_version"
)

// Result represents the analysis result for a single dependency
type Result struct {
	Package         string
	IsUnmaintained  bool
	Reason          UnmaintainedReason
	RepoInfo        *github.RepoInfo
	DaysSinceUpdate int
	Details         string
	CurrentVersion  string
	LatestVersion   string
}

// Config holds configuration for the analyzer
type Config struct {
	MaxAge        time.Duration
	Token         string
	Verbose       bool
	CheckOutdated bool
	NoCache       bool
	CacheDuration time.Duration
}

// Analyzer performs unmaintained package analysis
type Analyzer struct {
	config       Config
	githubClient *github.Client
	cache        *cache.Cache
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer(config Config) (*Analyzer, error) {
	githubClient, err := github.NewClient(config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Initialize cache
	cacheInstance, err := cache.NewCache(config.NoCache, config.CacheDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Clean expired cache entries
	if !config.NoCache {
		if err := cacheInstance.CleanExpired(); err != nil && config.Verbose {
			// Non-fatal error, just log in verbose mode
		}
	}

	return &Analyzer{
		config:       config,
		githubClient: githubClient,
		cache:        cacheInstance,
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
		Package:        dep.Path,
		CurrentVersion: dep.Version,
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

	// Try to get repository information from cache first
	var repoInfo *github.RepoInfo
	var latestVersion string

	cachedRepoInfo, cachedVersion, cacheHit := a.cache.GetRepoInfo(owner, repo)
	if cacheHit {
		repoInfo = cachedRepoInfo
		latestVersion = cachedVersion
		if a.config.Verbose {
			result.Details += " (from cache)"
		}
	} else {
		// Get repository information from GitHub API
		var err error
		repoInfo, err = a.githubClient.GetRepositoryInfo(ctx, owner, repo)
		if err != nil {
			return result, fmt.Errorf("failed to get repository info: %w", err)
		}

		// Get latest version if checking for outdated packages
		if a.config.CheckOutdated && repoInfo.Exists && !repoInfo.IsArchived {
			latestVersion, _ = a.githubClient.GetLatestVersion(ctx, owner, repo)
			// Ignore version check errors - will be handled later
		}

		// Cache the results
		if cacheErr := a.cache.SetRepoInfo(owner, repo, repoInfo, latestVersion); cacheErr != nil && a.config.Verbose {
			result.Details += fmt.Sprintf(" (cache write failed: %v)", cacheErr)
		}
	}

	result.RepoInfo = repoInfo
	result.DaysSinceUpdate = repoInfo.DaysSinceLastActivity()
	result.LatestVersion = latestVersion

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

	// Check for outdated versions if enabled and we have version info
	if a.config.CheckOutdated && latestVersion != "" {
		if a.isVersionOutdated(dep.Version, latestVersion) {
			result.IsUnmaintained = true
			result.Reason = ReasonOutdated
			result.Details = fmt.Sprintf("Using outdated version %s (latest: %s)", dep.Version, latestVersion)
			return result, nil
		}
	}

	result.Details = fmt.Sprintf("Active repository, last updated %d days ago", result.DaysSinceUpdate)
	if result.LatestVersion != "" {
		result.Details += fmt.Sprintf(" (version: %s, latest: %s)", dep.Version, result.LatestVersion)
	}
	return result, nil
}

// isVersionOutdated checks if the current version is outdated compared to the latest
func (a *Analyzer) isVersionOutdated(current, latest string) bool {
	if current == "" || latest == "" {
		return false
	}

	// Normalize versions to ensure they start with 'v'
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	if !strings.HasPrefix(latest, "v") {
		latest = "v" + latest
	}

	// Only compare if both are valid semantic versions
	if !semver.IsValid(current) || !semver.IsValid(latest) {
		return false
	}

	// Compare versions: return true if current is less than latest
	return semver.Compare(current, latest) < 0
}

// SummaryStats holds summary statistics
type SummaryStats struct {
	TotalDependencies  int
	UnmaintainedCount  int
	ArchivedCount      int
	NotFoundCount      int
	StaleInactiveCount int
	OutdatedCount      int
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
			case ReasonOutdated:
				stats.OutdatedCount++
			}
		}
	}

	return stats
}
