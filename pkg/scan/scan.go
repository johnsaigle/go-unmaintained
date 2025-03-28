package scan

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Config holds the application configuration
type Config struct {
	Target    string
	Token    string
	Owner    string
	RepoName string
	Timeout  time.Duration
}


var defaultTimeout = time.Minute 

// Validate ensures all required fields are present and correctly formatted
func (c *Config) Validate() error {
	if c.Token == "" {
		return errors.New("GitHub token is required")
	}

	if c.Owner == "" {
		return errors.New("repository owner is required")
	}

	if c.RepoName == "" {
		return errors.New("repository name is required")
	}

	if c.Timeout <= 0 {
		return errors.New("timeout must be a positive duration")
	}

	return nil
}

func ScanRepo(config Config) {

	f, err := os.Open(fmt.Sprintf("%s/go.mod", config.Target))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var line string
	for scanner.Scan() {
		line = scanner.Text()
		break

	}

	slog.Debug("line: %s\n", line)
	parts := strings.Split(line, " ")
	url := parts[1]
	parts = strings.Split(url, "/")

	config.Owner = parts[1]
	config.RepoName = parts[2]
	config.Timeout = defaultTimeout

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// Create a GitHub client
	client, err := createGitHubClient(config.Token)
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Get repository status
	status, err := getRepositoryArchiveStatus(ctx, client, config.Owner, config.RepoName)
	if err != nil {
		log.Fatalf("Error fetching repository status: %v", err)
	}

	// Print the result
	if status.IsArchived {
		fmt.Printf("Repository %s/%s is archived.\n", config.Owner, config.RepoName)
	} else {
		fmt.Printf("Repository %s/%s is not archived.\n", config.Owner, config.RepoName)
	}

	// Print additional repository information
	// fmt.Printf("Description: %s\n", status.Description)
	// fmt.Printf("Default Branch: %s\n", status.DefaultBranch)
	// fmt.Printf("Created: %s\n", status.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Last Updated: %s\n", status.UpdatedAt.Format(time.RFC3339))
}

// RepoStatus holds repository status information
type RepoStatus struct {
	IsArchived    bool
	Description   string
	DefaultBranch string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// createGitHubClient creates a new authenticated GitHub client
func createGitHubClient(token string) (*github.Client, error) {
	if token == "" {
		return nil, errors.New("GitHub token is required")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return github.NewClient(tc), nil
}

// getRepositoryArchiveStatus fetches the archive status of a GitHub repository
func getRepositoryArchiveStatus(ctx context.Context, client *github.Client, owner, repo string) (*RepoStatus, error) {
	if client == nil {
		return nil, errors.New("GitHub client is nil")
	}

	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo name must be provided")
	}

	// Fetch repository information
	repository, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		// Check for specific API errors
		if resp != nil && resp.StatusCode == 404 {
			return nil, fmt.Errorf("repository %s/%s not found", owner, repo)
		}
		return nil, fmt.Errorf("failed to fetch repository: %w", err)
	}

	// Ensure repository is not nil
	if repository == nil {
		return nil, errors.New("received nil repository from GitHub API")
	}

	// Extract relevant information, using safe dereference for pointers
	status := &RepoStatus{
		IsArchived:    repository.GetArchived(),
		Description:   repository.GetDescription(),
		DefaultBranch: repository.GetDefaultBranch(),
		CreatedAt:     repository.GetCreatedAt().Time,
		UpdatedAt:     repository.GetUpdatedAt().Time,
	}

	return status, nil
}
