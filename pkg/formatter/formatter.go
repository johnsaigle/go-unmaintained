package formatter

import (
	"fmt"
	"io"

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
