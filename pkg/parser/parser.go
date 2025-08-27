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

// ParseModulePath parses a module path into components
func ParseModulePath(path string) (host, owner, repo string, err error) {
	// Handle common Git hosting patterns
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid module path: %s", path)
	}

	host = parts[0]
	owner = parts[1]
	repo = parts[2]

	// Handle special cases like github.com/owner/repo/v2
	if len(parts) > 3 && strings.HasPrefix(parts[3], "v") {
		// This might be a version suffix, keep the base repo name
	}

	return host, owner, repo, nil
}
