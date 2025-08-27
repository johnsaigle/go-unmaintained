package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/cache"
	"github.com/johnsaigle/go-unmaintained/pkg/github"
	"github.com/johnsaigle/go-unmaintained/pkg/parser"
	"github.com/johnsaigle/go-unmaintained/pkg/providers"
	"github.com/johnsaigle/go-unmaintained/pkg/resolver"
	"golang.org/x/mod/semver"
)

// UnmaintainedReason represents why a package is considered unmaintained
type UnmaintainedReason string

const (
	ReasonArchived      UnmaintainedReason = "repository_archived"
	ReasonNotFound      UnmaintainedReason = "package_not_found"
	ReasonStaleInactive UnmaintainedReason = "stale_dependencies_inactive_repo"
	ReasonOutdated      UnmaintainedReason = "outdated_version"
	ReasonUnknown       UnmaintainedReason = "unknown_source"
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
	MaxAge          time.Duration
	Token           string
	Verbose         bool
	CheckOutdated   bool
	NoCache         bool
	CacheDuration   time.Duration
	ResolveUnknown  bool
	ResolverTimeout time.Duration
}

// Analyzer performs unmaintained package analysis
type Analyzer struct {
	config        Config
	githubClient  *github.Client
	cache         *cache.Cache
	resolver      *resolver.Resolver
	multiProvider *providers.MultiProvider
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

	// Initialize resolver - always available for well-known modules
	// Use longer timeout when not explicitly resolving unknown modules
	resolverTimeout := config.ResolverTimeout
	if !config.ResolveUnknown && resolverTimeout == 0 {
		resolverTimeout = 5 * time.Second // Shorter timeout for auto-resolution
	}
	moduleResolver := resolver.NewResolver(resolverTimeout)

	// Initialize multi-provider for GitLab, Bitbucket, etc.
	multiProvider := providers.NewMultiProvider()

	return &Analyzer{
		config:        config,
		githubClient:  githubClient,
		cache:         cacheInstance,
		resolver:      moduleResolver,
		multiProvider: multiProvider,
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
	moduleInfo := parser.ParseModulePath(dep.Path)
	if !moduleInfo.IsValid {
		result.Details = "Invalid module path format"
		return result, nil
	}

	// Handle non-GitHub dependencies
	if !moduleInfo.IsGitHub {
		result.Reason = ReasonUnknown

		// Check if it's a golang.org/x module - these map to GitHub
		if githubOwner, githubRepo, ok := parser.GetGitHubMapping(dep.Path); ok {
			// Analyze as GitHub repository
			repoInfo, err := a.githubClient.GetRepositoryInfo(ctx, githubOwner, githubRepo)
			if err != nil {
				result.Details = fmt.Sprintf("Failed to fetch Go repository info: %v", err)
				return result, nil
			}

			result.RepoInfo = repoInfo
			result.DaysSinceUpdate = repoInfo.DaysSinceLastActivity()

			if !repoInfo.Exists {
				result.IsUnmaintained = true
				result.Reason = ReasonNotFound
				result.Details = "Go extended package repository not found"
				return result, nil
			}

			if repoInfo.IsArchived {
				result.IsUnmaintained = true
				result.Reason = ReasonArchived
				result.Details = "Go extended package repository is archived"
				return result, nil
			}

			// Check if repository is inactive
			if !repoInfo.IsRepositoryActive(a.config.MaxAge) {
				result.IsUnmaintained = true
				result.Reason = ReasonStaleInactive
				result.Details = fmt.Sprintf("Go extended package inactive for %d days", result.DaysSinceUpdate)
				return result, nil
			}

			result.Details = fmt.Sprintf("Active Go extended package, last updated %d days ago", result.DaysSinceUpdate)
			return result, nil
		}

		// Check if it's a trusted module that should always be considered active
		if parser.IsTrustedGoModule(dep.Path) {
			result.Details = getTrustedModuleStatus(dep.Path)
			return result, nil
		}

		// Check if it's a supported hosting provider (GitLab, Bitbucket)
		if moduleInfo.IsKnownHost && (moduleInfo.Host == "gitlab.com" || moduleInfo.Host == "bitbucket.org") {
			// Try to get repository info from the appropriate provider
			repoInfo, err := a.multiProvider.GetRepositoryInfo(ctx, moduleInfo.Host, moduleInfo.Owner, moduleInfo.Repo)
			if err != nil {
				result.Details = fmt.Sprintf("Failed to fetch %s repository info: %v", moduleInfo.Host, err)
				return result, nil
			}

			if !repoInfo.Exists {
				result.IsUnmaintained = true
				result.Reason = ReasonNotFound
				result.Details = fmt.Sprintf("%s repository not found", strings.Title(strings.Split(moduleInfo.Host, ".")[0]))
				return result, nil
			}

			result.RepoInfo = repoInfo
			result.DaysSinceUpdate = repoInfo.DaysSinceLastActivity()

			if repoInfo.IsArchived {
				result.IsUnmaintained = true
				result.Reason = ReasonArchived
				result.Details = fmt.Sprintf("%s repository is archived", strings.Title(strings.Split(moduleInfo.Host, ".")[0]))
				return result, nil
			}

			// Check if repository is inactive
			if !repoInfo.IsRepositoryActive(a.config.MaxAge) {
				result.IsUnmaintained = true
				result.Reason = ReasonStaleInactive
				result.Details = fmt.Sprintf("%s repository inactive for %d days", strings.Title(strings.Split(moduleInfo.Host, ".")[0]), result.DaysSinceUpdate)
				return result, nil
			}

			result.Details = fmt.Sprintf("Active %s repository, last updated %d days ago", strings.Title(strings.Split(moduleInfo.Host, ".")[0]), result.DaysSinceUpdate)
			return result, nil
		}

		// Try to resolve the module if resolution is enabled OR if it's a well-known module
		if (a.config.ResolveUnknown || moduleInfo.IsKnownHost) && a.resolver != nil {
			resolved := a.resolver.ResolveModule(ctx, dep.Path)
			if resolved != nil {
				switch resolved.Status {
				case resolver.StatusActive:
					result.Details = fmt.Sprintf("Active non-GitHub dependency (%s): %s", resolved.HostingProvider, resolved.Details)
				case resolver.StatusNotFound:
					result.IsUnmaintained = true
					result.Reason = ReasonNotFound
					result.Details = fmt.Sprintf("Module not found: %s", resolved.Details)
				case resolver.StatusUnavailable:
					result.IsUnmaintained = true
					result.Details = fmt.Sprintf("Module unavailable (%s): %s", resolved.HostingProvider, resolved.Details)
				default:
					result.Details = fmt.Sprintf("Unknown status (%s): %s", resolved.HostingProvider, resolved.Details)
				}
			} else {
				result.Details = fmt.Sprintf("Could not resolve non-GitHub dependency (%s)", moduleInfo.Host)
			}
		} else {
			// Basic handling without resolution
			if moduleInfo.IsKnownHost {
				// Known hosting provider or well-known Go module
				result.Details = fmt.Sprintf("Non-GitHub dependency (%s) - status unknown", moduleInfo.Host)
			} else {
				// Unknown hosting provider
				result.Details = fmt.Sprintf("Unknown hosting provider (%s) - status unknown", moduleInfo.Host)
			}
		}

		// Don't mark as unmaintained unless specifically determined by resolver
		return result, nil
	}

	// Extract GitHub-specific info
	owner := moduleInfo.Owner
	repo := moduleInfo.Repo

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
	UnknownCount       int
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
		} else if result.Reason == ReasonUnknown {
			// Track unknown dependencies separately
			stats.UnknownCount++
		}
	}

	return stats
}

// getTrustedModuleStatus returns appropriate status message for trusted Go modules
func getTrustedModuleStatus(modulePath string) string {
	switch {
	case strings.HasPrefix(modulePath, "google.golang.org/"):
		return "Active Google-maintained Go package (trusted)"
	case strings.HasPrefix(modulePath, "cloud.google.com/"):
		return "Active Google Cloud Go package (trusted)"
	case strings.HasPrefix(modulePath, "k8s.io/"):
		return "Active Kubernetes package (trusted)"
	case strings.HasPrefix(modulePath, "sigs.k8s.io/"):
		return "Active Kubernetes SIG package (trusted)"
	case strings.HasPrefix(modulePath, "go.uber.org/"):
		return "Active Uber-maintained Go package (trusted)"
	default:
		return "Active trusted Go package"
	}
}
