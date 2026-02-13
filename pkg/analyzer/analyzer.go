package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/cache"
	"github.com/johnsaigle/go-unmaintained/pkg/github"
	"github.com/johnsaigle/go-unmaintained/pkg/parser"
	"github.com/johnsaigle/go-unmaintained/pkg/popular"
	"github.com/johnsaigle/go-unmaintained/pkg/providers"
	"github.com/johnsaigle/go-unmaintained/pkg/resolver"
	"github.com/johnsaigle/go-unmaintained/pkg/types"
	"golang.org/x/mod/semver"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// UnmaintainedReason represents why a package is considered unmaintained
type UnmaintainedReason string

const (
	ReasonArchived      UnmaintainedReason = "repository_archived"
	ReasonNotFound      UnmaintainedReason = "package_not_found"
	ReasonStaleInactive UnmaintainedReason = "stale_dependencies_inactive_repo"
	ReasonOutdated      UnmaintainedReason = "outdated_version"
	ReasonUnknown       UnmaintainedReason = "unknown_source"
	ReasonActive        UnmaintainedReason = "active_maintained"
)

// indexedDep represents a dependency with its index for concurrent processing
type indexedDep struct {
	dep   parser.Dependency
	index int
}

// Result represents the analysis result for a single dependency
type Result struct {
	RepoInfo         *types.RepoInfo
	Package          string
	Reason           UnmaintainedReason
	Details          string
	CurrentVersion   string
	LatestVersion    string
	DependencyPath   []string
	DaysSinceUpdate  int
	IsUnmaintained   bool
	IsDirect         bool
	IsRetracted      bool
	RetractionReason string
}

// Config holds configuration for the analyzer
type Config struct {
	Token           string
	MaxAge          time.Duration
	CacheDuration   time.Duration
	ResolverTimeout time.Duration
	Concurrency     int
	Verbose         bool
	CheckOutdated   bool
	NoCache         bool
	ResolveUnknown  bool
	AsyncMode       bool
	ShowProgress    bool
	ShowDepPath     bool
}

// Analyzer performs unmaintained package analysis
type Analyzer struct {
	githubClient  *github.Client
	cache         *cache.Cache
	resolver      *resolver.Resolver
	multiProvider *providers.MultiProvider
	config        Config
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
		_ = cacheInstance.CleanExpired()
		// Non-fatal error, silently ignore
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
	if a.config.AsyncMode {
		return a.analyzeModuleConcurrent(ctx, mod)
	}
	return a.analyzeModuleSequential(ctx, mod)
}

// analyzeModuleSequential processes dependencies one by one (original behavior)
func (a *Analyzer) analyzeModuleSequential(ctx context.Context, mod *parser.Module) ([]Result, error) {
	results := make([]Result, 0, len(mod.Dependencies))

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

		// Get dependency path for indirect unmaintained dependencies
		if a.config.ShowDepPath && result.IsUnmaintained && !result.IsDirect {
			depPath, err := parser.GetDependencyPath(ctx, mod.ProjectPath, dep.Path)
			if err == nil && len(depPath) > 0 {
				result.DependencyPath = depPath
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// analyzeModuleConcurrent processes dependencies concurrently with smart rate limiting
func (a *Analyzer) analyzeModuleConcurrent(ctx context.Context, mod *parser.Module) ([]Result, error) {
	concurrency := a.config.Concurrency
	if concurrency <= 0 {
		concurrency = 5 // Default concurrency
	}

	// Prioritize dependencies: cached first, then GitHub, then others
	var cachedDeps, githubDeps, otherDeps []indexedDep

	for i, dep := range mod.Dependencies {
		moduleInfo := parser.ParseModulePath(dep.Path)
		if !a.config.NoCache {
			// Check if this dependency is likely cached
			if moduleInfo.IsGitHub {
				_, _, cacheHit := a.cache.GetRepoInfo(moduleInfo.Owner, moduleInfo.Repo)
				if cacheHit {
					cachedDeps = append(cachedDeps, indexedDep{dep, i})
					continue
				}
			}
		}

		if moduleInfo.IsGitHub {
			githubDeps = append(githubDeps, indexedDep{dep, i})
		} else {
			otherDeps = append(otherDeps, indexedDep{dep, i})
		}
	}

	// Process in priority order: cached (fast), then others with rate limiting
	results := make([]Result, len(mod.Dependencies))

	// Process cached dependencies first (no rate limiting needed)
	if len(cachedDeps) > 0 {
		a.processBatch(ctx, cachedDeps, results, concurrency*2) // Higher concurrency for cached
	}

	// Process GitHub dependencies with moderate rate limiting
	if len(githubDeps) > 0 {
		a.processBatch(ctx, githubDeps, results, concurrency)
	}

	// Process other dependencies with conservative rate limiting
	if len(otherDeps) > 0 {
		a.processBatch(ctx, otherDeps, results, maxInt(1, concurrency/2)) // Lower concurrency for unknowns
	}

	return results, nil
}

// processBatch processes a batch of dependencies with specified concurrency
func (a *Analyzer) processBatch(ctx context.Context, deps []indexedDep, results []Result, batchConcurrency int) {
	if len(deps) == 0 {
		return
	}

	semaphore := make(chan struct{}, batchConcurrency)

	type indexedResult struct {
		result Result
		index  int
	}
	resultsChan := make(chan indexedResult, len(deps))

	// Start workers for this batch
	for _, idep := range deps {
		go func(indexedDep indexedDep) {
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := a.AnalyzeDependency(ctx, indexedDep.dep)
			if err != nil {
				result = Result{
					Package:        indexedDep.dep.Path,
					IsUnmaintained: false,
					Details:        fmt.Sprintf("Analysis error: %v", err),
				}
			}

			resultsChan <- indexedResult{index: indexedDep.index, result: result}
		}(idep)
	}

	// Collect results
	for range deps {
		indexedRes := <-resultsChan
		results[indexedRes.index] = indexedRes.result
	}
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// AnalyzeDependency analyzes a single dependency by delegating to specialized methods.
func (a *Analyzer) AnalyzeDependency(ctx context.Context, dep parser.Dependency) (Result, error) {
	result := a.initResult(dep)

	// Skip replaced dependencies
	if dep.Replace != nil {
		result.Details = "Skipped: has replace directive"
		return result, nil
	}

	// Try popular cache first
	if entry, found := popular.Lookup(dep.Path); found {
		return a.resultFromPopularEntry(entry, dep), nil
	}

	moduleInfo := parser.ParseModulePath(dep.Path)
	if !moduleInfo.IsValid {
		result.Details = "Invalid module path format"
		return result, nil
	}

	// Route to appropriate handler based on module type
	if !moduleInfo.IsGitHub {
		return a.analyzeNonGitHub(ctx, dep, moduleInfo)
	}

	return a.analyzeGitHub(ctx, dep, moduleInfo)
}

// initResult creates an initial Result with basic fields populated.
func (a *Analyzer) initResult(dep parser.Dependency) Result {
	return Result{
		Package:        dep.Path,
		CurrentVersion: dep.Version,
		IsDirect:       !dep.Indirect,
	}
}

// analyzeNonGitHub handles dependencies from non-GitHub sources (GitLab, Bitbucket, well-known modules, etc.)
func (a *Analyzer) analyzeNonGitHub(ctx context.Context, dep parser.Dependency, moduleInfo *parser.ModuleInfo) (Result, error) {
	result := a.initResult(dep)
	result.Reason = ReasonUnknown

	// Check if it's a golang.org/x module that maps to GitHub
	if githubOwner, githubRepo, ok := parser.GetGitHubMapping(dep.Path); ok {
		return a.analyzeGitHubMapping(ctx, dep, githubOwner, githubRepo)
	}

	// Check if it's a trusted module
	if parser.IsTrustedGoModule(dep.Path) {
		result.Reason = ReasonActive
		result.Details = getTrustedModuleStatus(dep.Path)
		return result, nil
	}

	// Check if it's a supported hosting provider (GitLab, Bitbucket)
	if moduleInfo.IsKnownHost && (moduleInfo.Host == "gitlab.com" || moduleInfo.Host == "bitbucket.org") {
		return a.analyzeThirdPartyProvider(ctx, dep, moduleInfo)
	}

	// Try to resolve via resolver if enabled
	return a.analyzeViaResolver(ctx, dep, moduleInfo)
}

// analyzeGitHubMapping handles golang.org/x modules that map to GitHub.
func (a *Analyzer) analyzeGitHubMapping(ctx context.Context, dep parser.Dependency, owner, repo string) (Result, error) {
	result := a.initResult(dep)

	repoInfo, err := a.githubClient.GetRepositoryInfo(ctx, owner, repo)
	if err != nil {
		result.Details = fmt.Sprintf("Failed to fetch Go repository info: %v", err)
		return result, nil
	}

	return a.applyRepoHeuristics(result, repoInfo, "Go extended package")
}

// analyzeThirdPartyProvider handles GitLab and Bitbucket repositories.
func (a *Analyzer) analyzeThirdPartyProvider(ctx context.Context, dep parser.Dependency, moduleInfo *parser.ModuleInfo) (Result, error) {
	result := a.initResult(dep)

	repoInfo, err := a.multiProvider.GetRepositoryInfo(ctx, moduleInfo.Host, moduleInfo.Owner, moduleInfo.Repo)
	if err != nil {
		result.Details = fmt.Sprintf("Failed to fetch %s repository info: %v", moduleInfo.Host, err)
		return result, nil
	}

	caser := cases.Title(language.English)
	hostName := caser.String(strings.Split(moduleInfo.Host, ".")[0])

	return a.applyRepoHeuristics(result, repoInfo, hostName)
}

// analyzeViaResolver attempts to resolve unknown modules using the resolver.
func (a *Analyzer) analyzeViaResolver(ctx context.Context, dep parser.Dependency, moduleInfo *parser.ModuleInfo) (Result, error) {
	result := a.initResult(dep)

	if (a.config.ResolveUnknown || moduleInfo.IsKnownHost) && a.resolver != nil {
		resolved := a.resolver.ResolveModule(ctx, dep.Path)
		if resolved != nil {
			switch resolved.Status {
			case resolver.StatusActive:
				result.Reason = ReasonActive
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
		if moduleInfo.IsKnownHost {
			result.Details = fmt.Sprintf("Non-GitHub dependency (%s) - status unknown", moduleInfo.Host)
		} else {
			result.Details = fmt.Sprintf("Unknown hosting provider (%s) - status unknown", moduleInfo.Host)
		}
	}

	return result, nil
}

// analyzeGitHub handles standard GitHub repositories with caching.
func (a *Analyzer) analyzeGitHub(ctx context.Context, dep parser.Dependency, moduleInfo *parser.ModuleInfo) (Result, error) {
	result := a.initResult(dep)

	owner := moduleInfo.Owner
	repo := moduleInfo.Repo

	repoInfo, latestVersion, err := a.fetchRepoWithCache(ctx, owner, repo)
	if err != nil {
		return result, err
	}

	result.RepoInfo = repoInfo
	result.DaysSinceUpdate = repoInfo.DaysSinceLastActivity()
	result.LatestVersion = latestVersion

	return a.applyHeuristics(result, dep)
}

// fetchRepoWithCache fetches repository info, using cache if available.
func (a *Analyzer) fetchRepoWithCache(ctx context.Context, owner, repo string) (*types.RepoInfo, string, error) {
	cachedInfo, cachedVersion, cacheHit := a.cache.GetRepoInfo(owner, repo)
	if cacheHit {
		return cachedInfo, cachedVersion, nil
	}

	repoInfo, err := a.githubClient.GetRepositoryInfo(ctx, owner, repo)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get repository info: %w", err)
	}

	latestVersion := ""
	if a.config.CheckOutdated && repoInfo.Exists && !repoInfo.IsArchived {
		latestVersion, _ = a.githubClient.GetLatestVersion(ctx, owner, repo)
	}

	a.cache.SetRepoInfo(owner, repo, repoInfo, latestVersion)
	return repoInfo, latestVersion, nil
}

// applyRepoHeuristics applies standard heuristics to a repoInfo and returns the result.
func (a *Analyzer) applyRepoHeuristics(result Result, repoInfo *types.RepoInfo, source string) (Result, error) {
	result.RepoInfo = repoInfo
	result.DaysSinceUpdate = repoInfo.DaysSinceLastActivity()

	if !repoInfo.Exists {
		result.IsUnmaintained = true
		result.Reason = ReasonNotFound
		result.Details = fmt.Sprintf("%s repository not found", source)
		return result, nil
	}

	if repoInfo.IsArchived {
		result.IsUnmaintained = true
		result.Reason = ReasonArchived
		result.Details = fmt.Sprintf("%s repository is archived", source)
		return result, nil
	}

	if !repoInfo.IsRepositoryActive(a.config.MaxAge) {
		result.IsUnmaintained = true
		result.Reason = ReasonStaleInactive
		result.Details = fmt.Sprintf("%s repository inactive for %d days", source, result.DaysSinceUpdate)
		return result, nil
	}

	result.Reason = ReasonActive
	result.Details = fmt.Sprintf("Active %s repository, last updated %d days ago", source, result.DaysSinceUpdate)
	return result, nil
}

// applyHeuristics applies standard heuristics for GitHub repositories.
func (a *Analyzer) applyHeuristics(result Result, dep parser.Dependency) (Result, error) {
	repoInfo := result.RepoInfo

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

	if !repoInfo.IsRepositoryActive(a.config.MaxAge) {
		result.IsUnmaintained = true
		result.Reason = ReasonStaleInactive
		result.Details = fmt.Sprintf("Repository inactive for %d days", result.DaysSinceUpdate)
		return result, nil
	}

	if a.config.CheckOutdated && result.LatestVersion != "" && a.isVersionOutdated(dep.Version, result.LatestVersion) {
		result.IsUnmaintained = true
		result.Reason = ReasonOutdated
		result.Details = fmt.Sprintf("Using outdated version %s (latest: %s)", dep.Version, result.LatestVersion)
		return result, nil
	}

	result.Reason = ReasonActive
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
	TotalDependencies    int
	UnmaintainedCount    int
	DirectUnmaintained   int
	IndirectUnmaintained int
	ArchivedCount        int
	NotFoundCount        int
	StaleInactiveCount   int
	OutdatedCount        int
	UnknownCount         int
	RetractedCount       int
}

// GetSummary returns summary statistics from results
func GetSummary(results []Result) SummaryStats {
	stats := SummaryStats{
		TotalDependencies: len(results),
	}

	for _, result := range results {
		if result.IsUnmaintained {
			stats.UnmaintainedCount++

			// Track direct vs indirect
			if result.IsDirect {
				stats.DirectUnmaintained++
			} else {
				stats.IndirectUnmaintained++
			}

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

		// Track retracted versions separately (can be maintained or unmaintained)
		if result.IsRetracted {
			stats.RetractedCount++
		}
	}

	return stats
}

// getTrustedModuleStatus returns appropriate status message for trusted Go modules.
// Delegates to types.GetTrustedStatus for the canonical implementation.
func getTrustedModuleStatus(modulePath string) string {
	if status, ok := types.GetTrustedStatus(modulePath); ok {
		return "Active " + status + " (trusted)"
	}
	return "Active trusted Go package"
}

// resultFromPopularEntry converts a popular cache entry to an analysis result
func (a *Analyzer) resultFromPopularEntry(entry *popular.Entry, dep parser.Dependency) Result {
	daysSinceUpdate := entry.DaysSinceUpdate()
	daysSinceCacheBuild := entry.DaysSinceCacheBuild()

	result := Result{
		Package:         dep.Path,
		CurrentVersion:  dep.Version,
		IsDirect:        !dep.Indirect,
		DaysSinceUpdate: daysSinceUpdate,
	}

	cacheAgeInfo := ""
	if daysSinceCacheBuild > 0 {
		cacheAgeInfo = fmt.Sprintf(" (cache built %d days ago)", daysSinceCacheBuild)
	}

	switch entry.Status {
	case popular.StatusArchived:
		result.IsUnmaintained = true
		result.Reason = ReasonArchived
		result.Details = fmt.Sprintf("Repository is archived%s", cacheAgeInfo)
	case popular.StatusInactive:
		result.IsUnmaintained = true
		result.Reason = ReasonStaleInactive
		result.Details = fmt.Sprintf("Repository inactive for %d days%s", daysSinceUpdate, cacheAgeInfo)
	case popular.StatusNotFound:
		result.IsUnmaintained = true
		result.Reason = ReasonNotFound
		result.Details = fmt.Sprintf("Repository not found%s", cacheAgeInfo)
	case popular.StatusActive:
		result.IsUnmaintained = false
		result.Reason = ReasonActive
		result.Details = fmt.Sprintf("Active repository, last updated %d days ago%s", daysSinceUpdate, cacheAgeInfo)
	}

	return result
}

// CheckRetraction checks if a module version is retracted
func (a *Analyzer) CheckRetraction(ctx context.Context, modulePath, version string) (*resolver.RetractionInfo, error) {
	if a.resolver == nil {
		return nil, fmt.Errorf("resolver not initialized")
	}
	return a.resolver.CheckRetraction(ctx, modulePath, version)
}
