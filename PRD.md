# go-unmaintained: Go Package Maintenance Analysis Tool

## Overview
A CLI tool that automatically identifies unmaintained Go packages using heuristics, mirroring `cargo-unmaintained` functionality for the Go ecosystem.

**Goal:** Analyze `go.mod` files and dependencies to detect unmaintained packages before they become security/reliability risks.

## Problem
- No automated way to identify unmaintained Go packages
- Manual dependency auditing is time-consuming and error-prone  
- Go ecosystem lacks centralized unmaintained package tracking

## Detection Heuristics

### Core Detection Rules (from cargo-unmaintained)
1. **Repository Archived** - Package's GitHub repository is archived
2. **Package Not Found** - Package name doesn't exist in claimed repository  
3. **Stale Dependencies + Inactive Repo** - Package has incompatible dependency versions (>1 year old) AND repository hasn't been updated in >1 year

### Go-Specific Detection Rules
4. **Module Proxy Missing** - Package not available in Go module proxy (proxy.golang.org)
5. **Import Path Broken** - Import paths don't resolve or are redirected
6. **Go Version Incompatible** - Package doesn't support recent Go versions

## Features

### Input Methods
- Analyze `go.mod` file (current directory or specified path)
- Single package analysis mode (`--package flag`)
- Workspace support (`go.work` files)

### Analysis Capabilities  
- Parse `go.mod` and `go.sum` files
- Resolve transitive dependencies
- Handle replace directives and local modules
- Repository metadata via GitHub API
- Version comparison and compatibility checking

### Output Options
- Colorized console output
- Tree view showing dependency paths (`--tree`)
- JSON format for CI/CD (`--json`)
- Days since last repository update
- Exit codes: 0=clean, 1=unmaintained found, 2=error

## Technical Architecture

### Core Dependencies
- `github.com/spf13/cobra` - CLI framework  
- `golang.org/x/oauth2` - GitHub API authentication
- `github.com/google/go-github/v45` - GitHub API client
- `golang.org/x/mod` - Go module parsing

### Caching & Storage
- Cache location: `$HOME/.cache/go-unmaintained/` 
- Store repository metadata and analysis results
- Time-based expiry (24 hours default)
- `--no-cache` and `--purge` options

### Authentication
- `GITHUB_TOKEN` environment variable
- `GITHUB_TOKEN_PATH` for file-based tokens  
- Default storage: `~/.config/go-unmaintained/token.txt`
- `--save-token` command for setup

## CLI Interface

### Basic Usage
```bash
go-unmaintained                              # Analyze current directory
go-unmaintained --target /path/to/project    # Analyze specific project  
go-unmaintained --package github.com/foo/bar # Single package analysis
go-unmaintained --json                       # JSON output for CI/CD
```

### Configuration Options
```bash
--max-age 180      # Custom staleness threshold (days)
--no-cache         # Skip cache usage
--purge            # Clear all cached data
--fail-fast        # Exit on first unmaintained package  
--tree             # Show dependency paths
--verbose          # Detailed logging
--color always     # Force colored output
--no-warnings      # Suppress warnings
--no-exit-code     # Don't set exit code
```

### Configuration File
```toml
# .go-unmaintained.toml or in go.mod metadata
[tool.go-unmaintained]
ignore = [
    "github.com/legacy/pkg",    # Known maintained legacy package
    "example.com/internal/*"    # Internal packages
]
max_age = 365                   # Days until considered stale
```

## Implementation Roadmap

### Phase 1: Core Functionality
- [ ] Parse `go.mod` files and resolve dependencies
- [ ] GitHub API integration for repository metadata  
- [ ] Implement core heuristics (archived, not found, stale)
- [ ] Basic console output with coloring
- [ ] Caching system

### Phase 2: Enhanced Detection  
- [ ] Go module proxy integration
- [ ] Import path validation
- [ ] Version compatibility checking
- [ ] Tree view for dependency paths
- [ ] JSON output format

### Phase 3: Advanced Features
- [ ] Configuration file support
- [ ] Ignore patterns and rules
- [ ] CI/CD integration examples
- [ ] Performance optimization
- [ ] Extended repository provider support (GitLab, Bitbucket)

### Phase 4: Ecosystem Integration
- [ ] Go workspace (`go.work`) support  
- [ ] Private module support
- [ ] Advanced heuristics and ML-based detection
- [ ] Community-driven unmaintained package database

## Current Status
- ✅ Basic GitHub repository analysis
- ✅ CLI framework with Cobra
- ✅ GitHub API client integration
- 🔄 `go.mod` parsing (basic implementation exists)
- ❌ Core heuristic engine
- ❌ Dependency tree analysis
- ❌ Caching system