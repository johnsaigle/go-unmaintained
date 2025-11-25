package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/johnsaigle/go-unmaintained/pkg/analyzer"
)

// Formatter defines the interface for output formatters
type Formatter interface {
	// Format writes results to output in the specific format
	Format(w io.Writer, results []analyzer.Result, summary analyzer.SummaryStats) error

	// ShouldExit returns the exit code based on results
	// 0 = success, 1 = issues found, 2 = error
	ShouldExit(results []analyzer.Result) int
}

// Options holds configuration options for formatters
type Options struct {
	Verbose    bool
	ShowPaths  bool
	FailFast   bool
	NoExitCode bool
}

// New creates a formatter based on the format string
func New(format string, opts Options) (Formatter, error) {
	switch format {
	case "console", "":
		return &ConsoleFormatter{opts: opts}, nil
	case "json":
		return &JSONFormatter{opts: opts}, nil
	case "github-actions":
		return &GitHubActionsFormatter{opts: opts}, nil
	case "golangci-lint":
		return &GolangciLintFormatter{opts: opts}, nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// DefaultShouldExit returns the standard exit code based on results
// Used by formatters that don't need custom exit code logic
func DefaultShouldExit(results []analyzer.Result, noExitCode bool) int {
	if noExitCode {
		return 0
	}

	for _, result := range results {
		if result.IsUnmaintained {
			return 1
		}
	}

	return 0
}

// GetRepositoryURL extracts or constructs a repository URL from the result
func GetRepositoryURL(result analyzer.Result) string {
	// Try to use the URL from RepoInfo first
	if result.RepoInfo != nil && result.RepoInfo.URL != "" {
		return result.RepoInfo.URL
	}

	// Try to construct URL from package path for known hosts
	hosts := []struct {
		prefix string
		urlFmt string
	}{
		{"github.com/", "https://github.com/%s/%s"},
		{"gitlab.com/", "https://gitlab.com/%s/%s"},
		{"bitbucket.org/", "https://bitbucket.org/%s/%s"},
	}

	for _, host := range hosts {
		if strings.HasPrefix(result.Package, host.prefix) {
			parts := strings.Split(result.Package, "/")
			if len(parts) >= 3 {
				return fmt.Sprintf(host.urlFmt, parts[1], parts[2])
			}
		}
	}

	return ""
}
