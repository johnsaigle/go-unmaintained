package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Dependency represents a single module dependency
type Dependency struct {
	Path    string
	Version string
	Replace *Replace
}

// Replace represents a replace directive
type Replace struct {
	OldPath string
	NewPath string
	Version string
}

// Module represents a parsed go.mod file
type Module struct {
	Path         string
	GoVersion    string
	Dependencies []Dependency
	Replaces     []Replace
}

// ParseGoMod parses a go.mod file and returns module information
func ParseGoMod(projectPath string) (*Module, error) {
	goModPath := filepath.Join(projectPath, "go.mod")

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod file: %w", err)
	}

	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod file: %w", err)
	}

	mod := &Module{
		Path:      modFile.Module.Mod.Path,
		GoVersion: modFile.Go.Version,
	}

	// Parse dependencies
	for _, req := range modFile.Require {
		if req.Indirect {
			continue // Skip indirect dependencies for now
		}

		dep := Dependency{
			Path:    req.Mod.Path,
			Version: req.Mod.Version,
		}

		mod.Dependencies = append(mod.Dependencies, dep)
	}

	// Parse replace directives
	for _, replace := range modFile.Replace {
		repl := Replace{
			OldPath: replace.Old.Path,
			NewPath: replace.New.Path,
			Version: replace.New.Version,
		}
		mod.Replaces = append(mod.Replaces, repl)

		// Update dependency if it has a replace directive
		for i, dep := range mod.Dependencies {
			if dep.Path == replace.Old.Path {
				mod.Dependencies[i].Replace = &repl
				break
			}
		}
	}

	return mod, nil
}

// IsValidModulePath checks if a module path is valid
func IsValidModulePath(path string) bool {
	return module.CheckPath(path) == nil
}

// ModuleInfo contains parsed information about a module
type ModuleInfo struct {
	Host        string
	Owner       string
	Repo        string
	IsGitHub    bool
	IsKnownHost bool
	IsValid     bool
}

// ParseModulePath parses a module path into components with enhanced information
func ParseModulePath(path string) *ModuleInfo {
	info := &ModuleInfo{}

	if !IsValidModulePath(path) {
		return info // IsValid remains false
	}

	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		return info
	}

	info.Host = parts[0]
	info.IsValid = true

	// Check for known hosting providers
	switch info.Host {
	case "github.com":
		info.IsGitHub = true
		info.IsKnownHost = true
		if len(parts) >= 3 {
			info.Owner = parts[1]
			info.Repo = parts[2]
		}
	case "gitlab.com", "bitbucket.org":
		info.IsKnownHost = true
		if len(parts) >= 3 {
			info.Owner = parts[1]
			info.Repo = parts[2]
		}
	default:
		// Handle special cases for well-known Go modules
		if isWellKnownGoModule(path) {
			info.IsKnownHost = true
		}

		// Try to extract owner/repo for generic hosts
		if len(parts) >= 3 {
			info.Owner = parts[1]
			info.Repo = parts[2]
		}
	}

	return info
}

// isWellKnownGoModule checks if the module is a well-known Go module
func isWellKnownGoModule(path string) bool {
	wellKnownPrefixes := []string{
		"golang.org/x/",
		"google.golang.org/",
		"cloud.google.com/",
		"go.uber.org/",
		"go.opentelemetry.io/",
		"gopkg.in/",
		"k8s.io/",
		"sigs.k8s.io/",
	}

	for _, prefix := range wellKnownPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// IsTrustedGoModule checks if the module is from a trusted, actively maintained source
func IsTrustedGoModule(path string) bool {
	trustedPrefixes := []string{
		"golang.org/x/",      // Official Go extended packages
		"google.golang.org/", // Google-maintained packages
		"cloud.google.com/",  // Google Cloud packages
		"k8s.io/",            // Kubernetes packages
		"sigs.k8s.io/",       // Kubernetes SIG packages
		"go.uber.org/",       // Uber Go packages (well-maintained)
	}

	for _, prefix := range trustedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// GetGitHubMapping returns the GitHub repository for golang.org/x modules
func GetGitHubMapping(path string) (owner, repo string, ok bool) {
	if strings.HasPrefix(path, "golang.org/x/") {
		repoName := strings.TrimPrefix(path, "golang.org/x/")
		// Handle sub-packages like golang.org/x/crypto/ssh
		if idx := strings.Index(repoName, "/"); idx != -1 {
			repoName = repoName[:idx]
		}
		return "golang", repoName, true
	}
	return "", "", false
}

// Legacy function for backward compatibility
func ParseModulePathLegacy(path string) (host, owner, repo string, err error) {
	info := ParseModulePath(path)
	if !info.IsValid {
		return "", "", "", fmt.Errorf("invalid module path: %s", path)
	}
	return info.Host, info.Owner, info.Repo, nil
}
