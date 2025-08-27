package github

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client with authentication
type Client struct {
	client *github.Client
	token  string
}

// RepoInfo holds repository information
type RepoInfo struct {
	IsArchived    bool
	Description   string
	DefaultBranch string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastCommitAt  *time.Time
	URL           string
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

	return &Client{
		client: github.NewClient(tc),
		token:  token,
	}, nil
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
		if resp != nil && resp.StatusCode == 404 {
			return &RepoInfo{Exists: false}, nil
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
	commits, _, err := c.client.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if err == nil && len(commits) > 0 {
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
