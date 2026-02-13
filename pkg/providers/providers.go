package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/johnsaigle/go-unmaintained/pkg/types"
)

// Provider represents a hosting provider interface
type Provider interface {
	GetRepositoryInfo(ctx context.Context, owner, repo string) (*types.RepoInfo, error)
	GetName() string
	SupportsHost(host string) bool
}

// MultiProvider manages multiple hosting providers
type MultiProvider struct {
	providers []Provider
}

// NewMultiProvider creates a new multi-provider instance
func NewMultiProvider() *MultiProvider {
	return &MultiProvider{
		providers: []Provider{
			NewGitLabProvider(),
			NewBitbucketProvider(),
		},
	}
}

// GetRepositoryInfo attempts to get repository info from the appropriate provider
func (mp *MultiProvider) GetRepositoryInfo(ctx context.Context, host, owner, repo string) (*types.RepoInfo, error) {
	for _, provider := range mp.providers {
		if provider.SupportsHost(host) {
			return provider.GetRepositoryInfo(ctx, owner, repo)
		}
	}

	return nil, fmt.Errorf("no provider supports host: %s", host)
}

// GitLabProvider handles GitLab repositories
type GitLabProvider struct {
	httpClient *http.Client
}

// GitLabProject represents a GitLab project response
type GitLabProject struct {
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	WebURL         string    `json:"web_url"`
	DefaultBranch  string    `json:"default_branch"`
	ID             int       `json:"id"`
	Archived       bool      `json:"archived"`
}

// NewGitLabProvider creates a new GitLab provider
func NewGitLabProvider() *GitLabProvider {
	return &GitLabProvider{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetName returns the provider name
func (gp *GitLabProvider) GetName() string {
	return "GitLab"
}

// SupportsHost checks if this provider supports the given host
func (gp *GitLabProvider) SupportsHost(host string) bool {
	return host == "gitlab.com"
}

// GetRepositoryInfo fetches repository information from GitLab
func (gp *GitLabProvider) GetRepositoryInfo(ctx context.Context, owner, repo string) (*types.RepoInfo, error) {
	// GitLab API endpoint for projects
	projectPath := fmt.Sprintf("%s/%s", owner, repo)
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", strings.ReplaceAll(projectPath, "/", "%2F"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := gp.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return &types.RepoInfo{Exists: false}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var project GitLabProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, err
	}

	// Convert to common RepoInfo format
	repoInfo := &types.RepoInfo{
		Exists:        true,
		IsArchived:    project.Archived,
		Description:   project.Description,
		DefaultBranch: project.DefaultBranch,
		CreatedAt:     project.CreatedAt,
		UpdatedAt:     project.LastActivityAt,
		URL:           project.WebURL,
	}

	// Use LastActivityAt as commit time if available
	if !project.LastActivityAt.IsZero() {
		repoInfo.LastCommitAt = &project.LastActivityAt
	}

	return repoInfo, nil
}

// BitbucketProvider handles Bitbucket repositories
type BitbucketProvider struct {
	httpClient *http.Client
}

// BitbucketRepository represents a Bitbucket repository response
type BitbucketRepository struct {
	CreatedOn   time.Time `json:"created_on"`
	UpdatedOn   time.Time `json:"updated_on"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	Links       struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
	MainBranch struct {
		Name string `json:"name"`
	} `json:"mainbranch"`
	IsPrivate bool `json:"is_private"`
}

// NewBitbucketProvider creates a new Bitbucket provider
func NewBitbucketProvider() *BitbucketProvider {
	return &BitbucketProvider{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetName returns the provider name
func (bp *BitbucketProvider) GetName() string {
	return "Bitbucket"
}

// SupportsHost checks if this provider supports the given host
func (bp *BitbucketProvider) SupportsHost(host string) bool {
	return host == "bitbucket.org"
}

// GetRepositoryInfo fetches repository information from Bitbucket
func (bp *BitbucketProvider) GetRepositoryInfo(ctx context.Context, owner, repo string) (*types.RepoInfo, error) {
	// Bitbucket API endpoint for repositories
	url := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := bp.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return &types.RepoInfo{Exists: false}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bitbucket API returned status %d", resp.StatusCode)
	}

	var repository BitbucketRepository
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return nil, err
	}

	// Convert to common RepoInfo format
	repoInfo := &types.RepoInfo{
		Exists:        true,
		IsArchived:    false, // Bitbucket doesn't have archived status in this endpoint
		Description:   repository.Description,
		DefaultBranch: repository.MainBranch.Name,
		CreatedAt:     repository.CreatedOn,
		UpdatedAt:     repository.UpdatedOn,
		URL:           repository.Links.HTML.Href,
	}

	// Use UpdatedOn as commit time approximation
	if !repository.UpdatedOn.IsZero() {
		repoInfo.LastCommitAt = &repository.UpdatedOn
	}

	return repoInfo, nil
}

// GetProviderForHost returns the appropriate provider for a given host
func GetProviderForHost(host string) Provider {
	providers := []Provider{
		NewGitLabProvider(),
		NewBitbucketProvider(),
	}

	for _, provider := range providers {
		if provider.SupportsHost(host) {
			return provider
		}
	}

	return nil
}
