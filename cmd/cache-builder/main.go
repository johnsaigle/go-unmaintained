package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/go-github/github"
	ghclient "github.com/johnsaigle/go-unmaintained/pkg/github"
	"github.com/johnsaigle/go-unmaintained/pkg/popular"
	"golang.org/x/oauth2"
)

var (
	newEntries     = flag.Int("new-entries", 10, "Number of new API requests to make (will fetch more repos from GitHub to find this many new/stale entries)")
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

	fmt.Printf("Building popular packages cache (incremental mode)...\n")
	fmt.Printf("  New entries to fetch: %d\n", *newEntries)
	fmt.Printf("  Output: %s\n", *output)
	fmt.Printf("  Inactive threshold: %d days\n", *maxAge)
	fmt.Printf("  Cache staleness: %d days\n", *cacheStaleDays)
	fmt.Println()

	entries, apiCallsMade, err := buildCacheIncremental(*token, *output, *newEntries, *maxAge, *cacheStaleDays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building cache: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total entries in cache: %d (made %d API calls)\n", len(entries), apiCallsMade)

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

	fmt.Printf("✓ Cache written to %s\n", *output)

	// Print statistics
	stats := calculateStats(entries)
	fmt.Println()
	fmt.Println("Statistics:")
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

type repoTask struct {
	owner string
	repo  string
	index int
}

func buildCacheIncremental(token, outputPath string, newEntries, maxAge, cacheStaleDays int) ([]popular.Entry, int, error) {
	ctx := context.Background()
	now := time.Now()

	// Load existing cache if it exists
	existingCache := make(map[string]*popular.Entry)
	if data, err := os.ReadFile(outputPath); err == nil && len(data) > 0 {
		var entries []popular.Entry
		if err := json.Unmarshal(data, &entries); err == nil {
			for i := range entries {
				existingCache[entries[i].Package] = &entries[i]
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

	// Track how many API calls we've made
	apiCallsMade := 0
	apiCallsNeeded := newEntries

	// We'll fetch repos in batches and stop when we've made enough API calls
	// Start with fetching more repos than we need to account for fresh entries
	maxReposToFetch := len(existingCache) + (newEntries * 3) // Fetch 3x to ensure we get enough new/stale ones
	if maxReposToFetch < 100 {
		maxReposToFetch = 100
	}

	fmt.Printf("Fetching up to %d repos from GitHub to find %d new/stale entries...\n", maxReposToFetch, newEntries)

	var allRepos []*github.Repository
	perPage := 100
	pages := (maxReposToFetch + perPage - 1) / perPage

	for page := 1; page <= pages && apiCallsMade < apiCallsNeeded; page++ {
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
			return nil, apiCallsMade, fmt.Errorf("failed to search repositories (page %d): %w", page, err)
		}

		for i := range result.Repositories {
			allRepos = append(allRepos, &result.Repositories[i])
		}
		fmt.Printf("  Fetched page %d/%d (%d repos so far)\n", page, pages, len(allRepos))

		// Check rate limit
		if resp.Remaining < 100 {
			fmt.Printf("  Warning: Only %d API calls remaining\n", resp.Remaining)
		}

		if len(allRepos) >= maxReposToFetch {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("\nProcessing %d repositories...\n", len(allRepos))

	// Process repos and build final cache
	finalCache := make(map[string]*popular.Entry)

	// First, add all existing entries to final cache
	for pkg, entry := range existingCache {
		finalCache[pkg] = entry
	}

	// Now process each repo from GitHub
	for _, repo := range allRepos {
		if apiCallsMade >= apiCallsNeeded {
			fmt.Printf("  Reached API call limit (%d), stopping\n", apiCallsNeeded)
			break
		}

		owner := repo.GetOwner().GetLogin()
		repoName := repo.GetName()
		pkg := fmt.Sprintf("github.com/%s/%s", owner, repoName)

		// Check if entry exists in cache
		existing, exists := existingCache[pkg]

		if exists {
			// Entry exists - check if it's stale
			cacheAge := now.Sub(existing.CacheBuiltAt)
			if cacheAge.Hours() < float64(cacheStaleDays*24) {
				// Fresh entry - skip API call
				fmt.Printf("  ✓ %s (cached %d days ago, fresh)\n", pkg, int(cacheAge.Hours()/24))
				continue
			} else {
				// Stale entry - refresh it
				fmt.Printf("  ↻ %s (cached %d days ago, refreshing)\n", pkg, int(cacheAge.Hours()/24))
				entry := analyzeRepository(ctx, ghClient, owner, repoName, maxAge, now)
				finalCache[pkg] = &entry
				apiCallsMade++
			}
		} else {
			// New entry - fetch it
			fmt.Printf("  + %s (new)\n", pkg)
			entry := analyzeRepository(ctx, ghClient, owner, repoName, maxAge, now)
			finalCache[pkg] = &entry
			apiCallsMade++
		}

		// Small delay between requests
		time.Sleep(200 * time.Millisecond)
	}

	// Convert map to slice
	var entries []popular.Entry
	for _, entry := range finalCache {
		entries = append(entries, *entry)
	}

	return entries, apiCallsMade, nil
}

func buildCache(token string, count, maxAge int) ([]popular.Entry, error) {
	ctx := context.Background()

	// Create GitHub client
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Create our wrapper client for getting repo info
	ghClient, err := ghclient.NewClient(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Search for top Go repositories
	fmt.Println("Searching for popular Go repositories...")

	var allRepos []*github.Repository
	perPage := 100
	pages := (count + perPage - 1) / perPage // Calculate number of pages needed

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
			return nil, fmt.Errorf("failed to search repositories (page %d): %w", page, err)
		}

		// Convert to pointers
		for i := range result.Repositories {
			allRepos = append(allRepos, &result.Repositories[i])
		}
		fmt.Printf("  Fetched page %d/%d (%d repos so far)\n", page, pages, len(allRepos))

		// Check rate limit
		if resp.Remaining < 100 {
			fmt.Printf("  Warning: Only %d API calls remaining\n", resp.Remaining)
			if resp.Remaining < 50 {
				waitTime := time.Until(resp.Reset.Time) + time.Second
				fmt.Printf("  Waiting %v for rate limit reset...\n", waitTime)
				time.Sleep(waitTime)
			}
		}

		// Stop if we have enough
		if len(allRepos) >= count {
			break
		}

		// Small delay to be nice to GitHub API
		time.Sleep(500 * time.Millisecond)
	}

	// Trim to exact count
	if len(allRepos) > count {
		allRepos = allRepos[:count]
	}

	fmt.Printf("\nAnalyzing %d repositories concurrently (3 workers)...\n", len(allRepos))

	// Process repositories concurrently with rate limiting
	// Use conservative concurrency (3) to avoid rate limits
	entries := processRepositoriesConcurrent(ctx, ghClient, allRepos, maxAge, 3)

	return entries, nil
}

func processRepositoriesConcurrent(ctx context.Context, ghClient *ghclient.Client, repos []*github.Repository, maxAge, concurrency int) []popular.Entry {
	cacheBuiltAt := time.Now()
	entries := make([]popular.Entry, len(repos))

	// Results channel
	type indexedEntry struct {
		entry popular.Entry
		index int
	}
	results := make(chan indexedEntry, len(repos))

	// Progress tracking
	completed := 0
	var mu sync.Mutex

	// Process in batches to better handle rate limiting
	batchSize := 50
	for batchStart := 0; batchStart < len(repos); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(repos) {
			batchEnd = len(repos)
		}

		batch := repos[batchStart:batchEnd]
		fmt.Printf("  Processing batch %d-%d...\n", batchStart+1, batchEnd)

		// Create work queue for this batch
		tasks := make(chan repoTask, len(batch))
		for i, repo := range batch {
			tasks <- repoTask{
				index: batchStart + i,
				owner: repo.GetOwner().GetLogin(),
				repo:  repo.GetName(),
			}
		}
		close(tasks)

		// Semaphore for rate limiting within batch
		semaphore := make(chan struct{}, concurrency)

		// Launch workers for this batch
		var wg sync.WaitGroup
		for task := range tasks {
			wg.Add(1)
			go func(t repoTask) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() {
					<-semaphore
					// Small delay between requests
					time.Sleep(200 * time.Millisecond)
				}()

				entry := analyzeRepository(ctx, ghClient, t.owner, t.repo, maxAge, cacheBuiltAt)
				results <- indexedEntry{index: t.index, entry: entry}

				// Update progress
				mu.Lock()
				completed++
				if completed%25 == 0 || completed == len(repos) {
					fmt.Printf("  Progress: %d/%d repositories (%.1f%%)\n",
						completed, len(repos), float64(completed)/float64(len(repos))*100)
				}
				mu.Unlock()
			}(task)
		}

		// Wait for batch to complete
		wg.Wait()

		// Delay between batches to avoid rate limiting
		if batchEnd < len(repos) {
			fmt.Printf("  Completed batch, waiting 5 seconds before next batch...\n")
			time.Sleep(5 * time.Second)
		}
	}

	close(results)

	// Collect results
	for res := range results {
		entries[res.index] = res.entry
	}

	return entries
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

	daysSinceUpdate := repoInfo.DaysSinceLastActivity()
	lastUpdated := repoInfo.UpdatedAt
	if repoInfo.LastCommitAt != nil && repoInfo.LastCommitAt.After(repoInfo.UpdatedAt) {
		lastUpdated = *repoInfo.LastCommitAt
	}

	return popular.Entry{
		Package:         fmt.Sprintf("github.com/%s/%s", owner, repoName),
		Owner:           owner,
		Repo:            repoName,
		Status:          status,
		DaysSinceUpdate: daysSinceUpdate,
		LastUpdated:     lastUpdated,
		CacheBuiltAt:    cacheBuiltAt,
	}
}
