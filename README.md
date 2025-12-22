# go-unmaintained

[![Dependency Health](https://github.com/johnsaigle/go-unmaintained/actions/workflows/self-check.yml/badge.svg)](https://github.com/johnsaigle/go-unmaintained/actions/workflows/self-check.yml)

> [!WARNING]
> Largely vibe-coded, trust at your own peril.

A CLI tool that identifies unmaintained Go packages using heuristics.

## Features

- Scans `go.mod` files to identify potentially unmaintained dependencies
- Detects archived repositories, missing packages, inactive projects, and outdated versions
- Multi-platform support: GitHub, GitLab, Bitbucket
- Concurrent analysis with configurable workers (default: 5)
- Smart caching for performance (24-hour default)
- Multiple output formats: console, JSON, GitHub Actions annotations, golangci-lint
- Dependency tree visualization for indirect dependencies
- Intelligent rate limiting and retry logic

## Installation

### From Source

```bash
git clone https://github.com/johnsaigle/go-unmaintained.git
cd go-unmaintained
make build
```

The binary will be available at `./bin/go-unmaintained`.

### Install to GOPATH

```bash
make install
```

### As GitHub Action

Add to your workflow:

```yaml
- name: Check for unmaintained dependencies
  uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    github-token: ${{ secrets.PAT }}  # Required: see setup below
```

See [GitHub Action documentation](./.github/actions/check/README.md) for more details and examples.

## GitHub Token Setup

A GitHub Personal Access Token (PAT) is required for API access:

**For CLI usage:**
1. [Create a GitHub PAT](https://github.com/settings/tokens):
   - **Fine-grained token (recommended)**: 
     - Repository access: "Public Repositories (read-only)"
     - No additional permissions needed (Metadata is auto-set to read-only)
   - **Classic token**: No scopes needed (or `public_repo` for consistency)
2. Set it as an environment variable:
   ```bash
   export PAT=your_token_here
   ```
   Or use the `--token` flag when running the tool

**For GitHub Actions (check action):**
1. [Create a GitHub PAT](https://github.com/settings/tokens):
   - **Fine-grained token**: 
     - Repository access: "All repositories" or "Only select repositories"
     - Repository permissions: **Metadata** (auto-set to read-only)
   - **Classic token**: `public_repo` scope (or no scopes)
2. Add it as a repository secret named `PAT`:
   - Go to Settings → Secrets and variables → Actions
   - Click "New repository secret"  
   - Name: `PAT`, Value: your token
3. Use it in workflows: `github-token: ${{ secrets.PAT }}`

**For cache building workflow (maintainers only):**
1. [Create a GitHub PAT](https://github.com/settings/tokens):
   - **Fine-grained token**: 
     - Repository access: "Only select repositories" → select this repo
     - Repository permissions: **Contents** → **Read and write**
   - **Classic token**: `repo` scope (full control)
2. Add as secret named `PAT` (same process as above)

> **Note:** `GITHUB_TOKEN` (the default Actions token) has insufficient permissions for this tool. A PAT is required.

## Usage

### Basic Usage

Analyze the current project (uses concurrent analysis by default):

```bash
go-unmaintained
```

Analyze a specific project:

```bash
go-unmaintained --target /path/to/project
```

### Common Options

```bash
# Check for outdated versions
go-unmaintained --check-outdated

# JSON output
go-unmaintained --format json

# GitHub Actions annotations
go-unmaintained --format github-actions

# Verbose details with maintained packages
go-unmaintained --verbose

# Show dependency tree for indirect dependencies
go-unmaintained --tree

# Control concurrent workers (default: 5)
go-unmaintained --concurrency 10

# Disable caching
go-unmaintained --no-cache

# Sequential processing (slower, less memory)
go-unmaintained --sync

# Resolve non-GitHub dependencies
go-unmaintained --resolve-unknown

# Exit immediately on first unmaintained package
go-unmaintained --fail-fast
```

See `go-unmaintained --help` for all options.

### Example Output

```
📦 Project: github.com/example/myapp
🔍 Analyzing 25 dependencies (concurrent: 5 workers)...

Dependency Analysis Results:
============================

🚨 UNMAINTAINED PACKAGES (4 found):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
❌ github.com/abandoned/old-lib (direct) - Repository is archived
   🔗 https://github.com/abandoned/old-lib
   Last commit: 1024 days ago
   ⚠️  Repository archived (no new commits possible)
❌ github.com/missing/gone (direct) - Repository not found
   🔗 https://github.com/missing/gone
❌ github.com/stale/inactive (indirect) - Repository inactive for 500 days
   🔗 https://github.com/stale/inactive
   Last commit: 500 days ago
   📍 Dependency path: myapp → dep-a → stale/inactive
❌ github.com/old/version (direct) - Using outdated version v1.2.0 (latest: v2.1.0)
   🔗 https://github.com/old/version

❓ UNKNOWN STATUS PACKAGES (1 found):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
❓ golang.org/x/tools - Active non-GitHub dependency (golang.org): Official Go extended package

══════════════════════════════════════════════════
📊 ANALYSIS SUMMARY
══════════════════════════════════════════════════
Total dependencies analyzed: 25

🚨 UNMAINTAINED PACKAGES: 4 (3 direct, 1 indirect)
   📦 Archived repositories: 1
   🚫 Not found/deleted: 1
   💤 Stale/Inactive: 1
   📅 Outdated versions: 1

❓ UNKNOWN STATUS: 1
   (Non-GitHub dependencies that couldn't be fully analyzed)
✅ MAINTAINED PACKAGES: 20
   (Active repositories with recent updates)
```

## Detection Heuristics

The tool uses several heuristics to identify unmaintained packages:

1. **Repository Archived**: Repository is marked as archived (GitHub, GitLab, Bitbucket)
2. **Package Not Found**: Repository doesn't exist or is inaccessible (404 errors)
3. **Inactive Repository**: No commits or updates within the specified time frame (default: 365 days, configurable with `--max-age`)
4. **Outdated Versions**: (requires `--check-outdated`) Current version is significantly behind the latest released version
5. **Unknown Status**: Non-GitHub dependencies that couldn't be resolved (shown with ❓)
   - Use `--resolve-unknown` to attempt deeper analysis of these packages
   - Popular/official packages (e.g., `golang.org/x/*`) are automatically recognized

### Multi-Platform Support

The tool supports multiple Git hosting platforms:
- **GitHub**: Full support with API integration
- **GitLab**: Full support with API integration
- **Bitbucket**: Full support with API integration
- **Others**: Basic support via `--resolve-unknown` flag

## Output Formats

The tool supports multiple output formats via the `--format` flag:

### Console (default)
Human-readable output with emojis and color support:
```bash
go-unmaintained --format console
```

### JSON
Machine-readable JSON output for automation:
```bash
go-unmaintained --format json
```

### GitHub Actions
Annotations format for GitHub Actions workflows:
```bash
go-unmaintained --format github-actions
```

### golangci-lint
Compatible with golangci-lint output format:
```bash
go-unmaintained --format golangci-lint
```

### Color Control
Control color output for console format:
```bash
go-unmaintained --color always   # Always use colors
go-unmaintained --color never    # Never use colors
go-unmaintained --color auto     # Auto-detect (default)
```

## Performance & Configuration

### Concurrent Processing

By default, the tool uses 5 concurrent workers for parallel analysis. You can adjust this based on your needs:

```bash
# More workers for faster analysis (if you have good network/API limits)
go-unmaintained --concurrency 10

# Fewer workers to be more conservative with rate limits
go-unmaintained --concurrency 2

# Sequential processing (slowest, but minimal resource usage)
go-unmaintained --sync
```

### Caching

The tool caches API responses to disk for 24 hours by default:

```bash
# Adjust cache duration
go-unmaintained --cache-duration 48  # 48 hours

# Disable caching entirely
go-unmaintained --no-cache
```

Cache location: `~/.cache/go-unmaintained/` (or system-appropriate cache directory)

### Rate Limiting

- **Authenticated requests**: 5,000 GitHub API requests/hour
- **Unauthenticated requests**: 60 GitHub API requests/hour
- The tool includes intelligent rate limiting and retry logic
- Caching significantly reduces API calls for repeated analyses
- Clear error messages when rate limits are exceeded

## Acknowledgments

Inspired by [cargo-unmaintained](https://github.com/trailofbits/cargo-unmaintained) for the Rust ecosystem.
