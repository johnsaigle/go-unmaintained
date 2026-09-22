package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/go-github/v83/github"
	ghclient "github.com/johnsaigle/go-unmaintained/pkg/github"
	"github.com/johnsaigle/go-unmaintained/pkg/popular"
	"golang.org/x/oauth2"
)

var (
	newEntries     = flag.Int("new-entries", 10, "Maximum number of newly ranked repositories to add")
	refreshEntries = flag.Int("refresh-entries", 10, "Maximum number of stale cached repositories to refresh")
	maxEntries     = flag.Int("max-entries", 500, "Maximum number of ranked repositories to retain (maximum 1000)")
	output         = flag.String("output", "pkg/popular/data/popular-packages.json", "Output file path")
	token          = flag.String("token", "", "GitHub token (required)")
	maxAge         = flag.Int("max-age", 365, "Age in days to consider a repo inactive")
	cacheStaleDays = flag.Int("cache-stale-days", 90, "Number of days before a cached entry is considered stale and should be refreshed")
)

func main() {
	flag.Parse()

	if *token == "" {
		// Try environment variable
		*token = os.Getenv("PAT")
		if *token == "" {
			fmt.Fprintf(os.Stderr, "Error: GitHub token is required. Use --token flag or PAT environment variable\n")
			os.Exit(1)
		}
	}
	if *newEntries < 0 || *refreshEntries < 0 {
		fmt.Fprintln(os.Stderr, "Error: update limits cannot be negative")
		os.Exit(1)
	}
	if *maxEntries < 1 || *maxEntries > 1000 {
		fmt.Fprintln(os.Stderr, "Error: --max-entries must be between 1 and 1000")
		os.Exit(1)
	}
	if *maxAge < 1 || *cacheStaleDays < 1 {
		fmt.Fprintln(os.Stderr, "Error: age limits must be at least one day")
		os.Exit(1)
	}

	fmt.Printf("Building popular packages cache (incremental mode)...\n")
	fmt.Printf("  New entries to add: %d\n", *newEntries)
	fmt.Printf("  Stale entries to refresh: %d\n", *refreshEntries)
	fmt.Printf("  Maximum cache entries: %d\n", *maxEntries)
	fmt.Printf("  Output: %s\n", *output)
	fmt.Printf("  Inactive threshold: %d days\n", *maxAge)
	fmt.Printf("  Cache staleness: %d days\n", *cacheStaleDays)
	fmt.Println()

	entries, repositoriesAnalyzed, err := buildCacheIncremental(
		*token,
		*output,
		*newEntries,
		*refreshEntries,
		*maxEntries,
		*maxAge,
		*cacheStaleDays,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building cache: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total entries in cache: %d (analyzed %d repositories)\n", len(entries), repositoriesAnalyzed)

	// Write to file
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*output, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Cache written to %s\n", *output)

	// Print statistics
	stats := calculateStats(entries)
	fmt.Println()
	fmt.Println("Statistics:")
	if len(entries) == 0 {
		fmt.Println("  Cache is empty")
		return
	}
	fmt.Printf("  Active:       %d (%.1f%%)\n", stats.active, float64(stats.active)/float64(len(entries))*100)
	fmt.Printf("  Archived:     %d (%.1f%%)\n", stats.archived, float64(stats.archived)/float64(len(entries))*100)
	fmt.Printf("  Inactive:     %d (%.1f%%)\n", stats.inactive, float64(stats.inactive)/float64(len(entries))*100)
	fmt.Printf("  Not Found:    %d (%.1f%%)\n", stats.notFound, float64(stats.notFound)/float64(len(entries))*100)
}

type stats struct {
	active   int
	archived int
	inactive int
	notFound int
}

func calculateStats(entries []popular.Entry) stats {
	var s stats
	for _, e := range entries {
		switch e.Status {
		case popular.StatusActive:
			s.active++
		case popular.StatusArchived:
			s.archived++
		case popular.StatusInactive:
			s.inactive++
		case popular.StatusNotFound:
			s.notFound++
		}
	}
	return s
}

type rankedRepository struct {
	packagePath string
	owner       string
	repo        string
}

type updateAction int

const (
	keepEntry updateAction = iota
	addEntry
	refreshEntry
	skipEntry
)

type cachePlanItem struct {
	repository rankedRepository
	existing   popular.Entry
	action     updateAction
}

func buildCacheIncremental(token, outputPath string, newEntries, refreshEntries, maxEntries, maxAge, cacheStaleDays int) ([]popular.Entry, int, error) {
	ctx := context.Background()
	now := time.Now()

	// Load existing cache if it exists
	existingCache := make(map[string]popular.Entry)
	if data, err := os.ReadFile(outputPath); err == nil && len(data) > 0 {
		var entries []popular.Entry
		if err := json.Unmarshal(data, &entries); err == nil {
			for i := range entries {
				existingCache[entries[i].Package] = entries[i]
			}
			fmt.Printf("Loaded existing cache with %d entries\n", len(existingCache))
		}
	} else {
		fmt.Println("No existing cache found, starting fresh")
	}

	// Create GitHub client
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Create our wrapper client for getting repo info
	ghClient, err := ghclient.NewClient(token)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	fmt.Printf("Fetching the top %d Go repositories from GitHub...\n", maxEntries)

	var allRepos []*github.Repository
	perPage := 100
	pages := (maxEntries + perPage - 1) / perPage

	for page := 1; page <= pages; page++ {
		opts := &github.SearchOptions{
			Sort:  "stars",
			Order: "desc",
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: perPage,
			},
		}

		result, resp, err := client.Search.Repositories(ctx, "language:go", opts)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to search repositories (page %d): %w", page, err)
		}

		allRepos = append(allRepos, result.Repositories...)
		fmt.Printf("  Fetched page %d/%d (%d repos so far)\n", page, pages, len(allRepos))

		// Check rate limit
		if resp.Rate.Remaining < 100 {
			fmt.Printf("  Warning: Only %d API calls remaining\n", resp.Rate.Remaining)
		}

		if len(allRepos) >= maxEntries || len(result.Repositories) == 0 {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	if len(allRepos) > maxEntries {
		allRepos = allRepos[:maxEntries]
	}

	rankedRepos := make([]rankedRepository, 0, len(allRepos))
	for _, repo := range allRepos {
		owner := repo.GetOwner().GetLogin()
		repoName := repo.GetName()
		rankedRepos = append(rankedRepos, rankedRepository{
			packagePath: fmt.Sprintf("github.com/%s/%s", owner, repoName),
			owner:       owner,
			repo:        repoName,
		})
	}

	plans := buildUpdatePlan(rankedRepos, existingCache, now, newEntries, refreshEntries, cacheStaleDays)
	entries := make([]popular.Entry, 0, len(plans))
	repositoriesAnalyzed := 0

	fmt.Printf("\nProcessing %d ranked repositories...\n", len(plans))
	for _, plan := range plans {
		switch plan.action {
		case keepEntry:
			entries = append(entries, plan.existing)
		case addEntry:
			fmt.Printf("  + %s (new)\n", plan.repository.packagePath)
			entries = append(entries, analyzeRepository(ctx, ghClient, plan.repository.owner, plan.repository.repo, maxAge, now))
			repositoriesAnalyzed++
			time.Sleep(200 * time.Millisecond)
		case refreshEntry:
			cacheAge := now.Sub(plan.existing.CacheBuiltAt)
			fmt.Printf("  refresh %s (cached %d days ago)\n", plan.repository.packagePath, int(cacheAge.Hours()/24))
			entries = append(entries, analyzeRepository(ctx, ghClient, plan.repository.owner, plan.repository.repo, maxAge, now))
			repositoriesAnalyzed++
			time.Sleep(200 * time.Millisecond)
		case skipEntry:
			// The addition budget is intentionally bounded; a later run will add it.
		}
	}

	return entries, repositoriesAnalyzed, nil
}

func buildUpdatePlan(rankedRepos []rankedRepository, existingCache map[string]popular.Entry, now time.Time, newLimit, refreshLimit, cacheStaleDays int) []cachePlanItem {
	plans := make([]cachePlanItem, 0, len(rankedRepos))
	newCount := 0
	refreshCount := 0
	staleAfter := time.Duration(cacheStaleDays) * 24 * time.Hour

	for _, repository := range rankedRepos {
		existing, exists := existingCache[repository.packagePath]
		plan := cachePlanItem{repository: repository, existing: existing, action: keepEntry}

		switch {
		case !exists && newCount < newLimit:
			plan.action = addEntry
			newCount++
		case !exists:
			plan.action = skipEntry
		case now.Sub(existing.CacheBuiltAt) >= staleAfter && refreshCount < refreshLimit:
			plan.action = refreshEntry
			refreshCount++
		}

		plans = append(plans, plan)
	}

	return plans
}

func analyzeRepository(ctx context.Context, ghClient *ghclient.Client, owner, repoName string, maxAge int, cacheBuiltAt time.Time) popular.Entry {
	// Get detailed repository info
	repoInfo, err := ghClient.GetRepositoryInfo(ctx, owner, repoName)
	if err != nil {
		// Return entry with not_found status
		return popular.Entry{
			Package:      fmt.Sprintf("github.com/%s/%s", owner, repoName),
			Owner:        owner,
			Repo:         repoName,
			Status:       popular.StatusNotFound,
			CacheBuiltAt: cacheBuiltAt,
		}
	}

	// Determine status
	var status popular.Status
	if !repoInfo.Exists {
		status = popular.StatusNotFound
	} else if repoInfo.IsArchived {
		status = popular.StatusArchived
	} else if !repoInfo.IsRepositoryActive(time.Duration(maxAge) * 24 * time.Hour) {
		status = popular.StatusInactive
	} else {
		status = popular.StatusActive
	}

	lastUpdated := repoInfo.UpdatedAt
	if repoInfo.LastCommitAt != nil && repoInfo.LastCommitAt.After(repoInfo.UpdatedAt) {
		lastUpdated = *repoInfo.LastCommitAt
	}

	return popular.Entry{
		Package:      fmt.Sprintf("github.com/%s/%s", owner, repoName),
		Owner:        owner,
		Repo:         repoName,
		Status:       status,
		LastUpdated:  lastUpdated,
		CacheBuiltAt: cacheBuiltAt,
	}
}
