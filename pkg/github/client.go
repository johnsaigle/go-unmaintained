package github

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/mod/semver"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client with authentication
type Client struct {
	client *github.Client
	token  string
}

// RepoInfo holds repository information
type RepoInfo struct {
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastCommitAt  *time.Time
	Description   string
	DefaultBranch string
	URL           string
	IsArchived    bool
	Exists        bool
}

// NewClient creates a new authenticated GitHub client
func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, errors.New("GitHub token is required")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	client := &Client{
		client: github.NewClient(tc),
		token:  token,
	}

	// Validate the token before returning the client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.ValidateToken(ctx); err != nil {
		return nil, fmt.Errorf("GitHub token validation failed: %w", err)
	}

	return client, nil
}

// ValidateToken checks if the GitHub token is valid and has necessary permissions
func (c *Client) ValidateToken(ctx context.Context) error {
	if c.client == nil {
		return errors.New("GitHub client is nil")
	}

	// Test the token by trying to access a known public repository
	// This works for both PATs and GITHUB_TOKEN (Actions token)
	// Unlike Users.Get(), repository access doesn't require user-level permissions
	_, resp, err := c.client.Repositories.Get(ctx, "golang", "go")
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case 401:
				return errors.New("invalid or expired GitHub token")
			case 403:
				if resp.Header.Get("X-RateLimit-Remaining") == "0" {
					return errors.New("GitHub API rate limit exceeded")
				}
				return errors.New("GitHub token lacks necessary permissions to access public repositories")
			case 404:
				// Repository not found, but token is valid enough to make the request
				// This shouldn't happen with golang/go but handle gracefully
				return nil
			}
		}
		// For other errors, don't fail validation - the token might still work
		// Network issues, temporary GitHub outages, etc. shouldn't block usage
	}

	return nil
}

// GetRepositoryInfo fetches repository information for a given owner/repo
func (c *Client) GetRepositoryInfo(ctx context.Context, owner, repo string) (*RepoInfo, error) {
	if c.client == nil {
		return nil, errors.New("GitHub client is nil")
	}

	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo name must be provided")
	}

	// Fetch repository information
	repository, resp, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		// Check for specific API errors
		if resp != nil {
			switch resp.StatusCode {
			case 404:
				return &RepoInfo{Exists: false}, nil
			case 403:
				// Check if it's a rate limit error
				if resp.Header.Get("X-RateLimit-Remaining") == "0" {
					resetTime := resp.Header.Get("X-RateLimit-Reset")
					return nil, fmt.Errorf("GitHub API rate limit exceeded (resets at %s): %w", resetTime, err)
				}
				return nil, fmt.Errorf("GitHub API access forbidden: %w", err)
			case 401:
				return nil, fmt.Errorf("GitHub API authentication failed (check your token): %w", err)
			}
		}
		return nil, fmt.Errorf("failed to fetch repository: %w", err)
	}

	if repository == nil {
		return &RepoInfo{Exists: false}, nil
	}

	info := &RepoInfo{
		Exists:        true,
		IsArchived:    repository.GetArchived(),
		Description:   repository.GetDescription(),
		DefaultBranch: repository.GetDefaultBranch(),
		CreatedAt:     repository.GetCreatedAt().Time,
		UpdatedAt:     repository.GetUpdatedAt().Time,
		URL:           repository.GetHTMLURL(),
	}

	// Get the latest commit date
	commits, resp, err := c.client.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if err != nil {
		// Handle rate limiting for commits API call
		// Don't fail the entire request for commit info, just skip it
		_ = resp // Silence unused warning
	} else if len(commits) > 0 {
		commitDate := commits[0].GetCommit().GetCommitter().GetDate()
		info.LastCommitAt = &commitDate
	}

	return info, nil
}

// IsRepositoryActive checks if a repository has been active within the given duration
func (info *RepoInfo) IsRepositoryActive(maxAge time.Duration) bool {
	if !info.Exists {
		return false
	}

	// Use the latest of UpdatedAt or LastCommitAt
	latestActivity := info.UpdatedAt
	if info.LastCommitAt != nil && info.LastCommitAt.After(info.UpdatedAt) {
		latestActivity = *info.LastCommitAt
	}

	return time.Since(latestActivity) <= maxAge
}

// DaysSinceLastActivity returns the number of days since the last repository activity
func (info *RepoInfo) DaysSinceLastActivity() int {
	if !info.Exists {
		return -1
	}

	latestActivity := info.UpdatedAt
	if info.LastCommitAt != nil && info.LastCommitAt.After(info.UpdatedAt) {
		latestActivity = *info.LastCommitAt
	}

	return int(time.Since(latestActivity).Hours() / 24)
}

// GetLatestVersion gets the latest semantic version tag for a repository
func (c *Client) GetLatestVersion(ctx context.Context, owner, repo string) (string, error) {
	if c.client == nil {
		return "", errors.New("GitHub client is nil")
	}

	if owner == "" || repo == "" {
		return "", errors.New("owner and repo name must be provided")
	}

	// Get all tags for the repository
	tags, resp, err := c.client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{
		PerPage: 100, // Get up to 100 tags to find semantic versions
	})
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case 404:
				return "", errors.New("repository not found")
			case 403:
				if resp.Header.Get("X-RateLimit-Remaining") == "0" {
					resetTime := resp.Header.Get("X-RateLimit-Reset")
					return "", fmt.Errorf("GitHub API rate limit exceeded (resets at %s): %w", resetTime, err)
				}
				return "", fmt.Errorf("GitHub API access forbidden: %w", err)
			case 401:
				return "", fmt.Errorf("GitHub API authentication failed: %w", err)
			}
		}
		return "", fmt.Errorf("failed to fetch repository tags: %w", err)
	}

	if len(tags) == 0 {
		return "", errors.New("no tags found in repository")
	}

	// Extract semantic version tags
	var validVersions []string
	for _, tag := range tags {
		tagName := tag.GetName()
		// Normalize tag name to semantic version format
		tagName = strings.TrimPrefix(tagName, "v")
		if semver.IsValid("v" + tagName) {
			validVersions = append(validVersions, "v"+tagName)
		}
	}

	if len(validVersions) == 0 {
		// If no semantic versions found, return the latest tag name
		return tags[0].GetName(), nil
	}

	// Sort versions and return the latest
	sort.Slice(validVersions, func(i, j int) bool {
		return semver.Compare(validVersions[i], validVersions[j]) > 0
	})

	return validVersions[0], nil
}
