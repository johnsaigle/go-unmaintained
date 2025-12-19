# go-unmaintained

[![Dependency Health](https://github.com/johnsaigle/go-unmaintained/actions/workflows/self-check.yml/badge.svg)](https://github.com/johnsaigle/go-unmaintained/actions/workflows/self-check.yml)

> [!WARNING]
> Largely vibe-coded, trust at your own peril.

A CLI tool that identifies unmaintained Go packages using heuristics.

## Features

- Scans `go.mod` files to identify potentially unmaintained dependencies
- Detects archived repositories, missing packages, inactive projects, and outdated versions
- Concurrent analysis with smart rate limiting
- Multiple output formats: console, JSON, GitHub Actions annotations

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
1. [Create a GitHub PAT](https://github.com/settings/tokens) (no scopes required - public repo access only)
2. Set it as an environment variable:
   ```bash
   export PAT=your_token_here
   ```
   Or use the `--token` flag when running the tool

**For GitHub Actions:**
1. [Create a GitHub PAT](https://github.com/settings/tokens) (no scopes required)
2. Add it as a repository secret named `PAT`:
   - Go to repository Settings → Secrets and variables → Actions
   - Click "New repository secret"
   - Name: `PAT`, Value: your token
3. Use it in workflows: `github-token: ${{ secrets.PAT }}`

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
go-unmaintained --json

# Verbose details
go-unmaintained --verbose
```

See `go-unmaintained --help` for all options.

### Example Output

```
Analyzing project: github.com/example/myapp
Go version: 1.21
Dependencies: 25

Dependency Analysis Results:
============================
❌ github.com/abandoned/old-lib - Repository is archived
   Last updated: 1024 days ago
❌ github.com/missing/gone - Repository not found
✅ github.com/active/current - Active repository, last updated 5 days ago
❌ github.com/stale/inactive - Repository inactive for 500 days
❌ github.com/old/version - Using outdated version v1.2.0 (latest: v2.1.0)
❓ golang.org/x/tools - Active non-GitHub dependency (golang.org): Official Go extended package
✅ gitlab.com/example/project - Active GitLab repository, last updated 10 days ago

Summary:
Total dependencies: 25
Unmaintained: 4
  - Archived: 1
  - Not found: 1
  - Stale/Inactive: 1
  - Outdated: 1
Unknown status: 1
```

## Detection Heuristics

The tool uses several heuristics to identify unmaintained packages:

1. **Repository Archived**: Repository is marked as archived (GitHub, GitLab, Bitbucket)
2. **Package Not Found**: Repository doesn't exist or is inaccessible
3. **Inactive Repository**: No commits or updates within the specified time frame (default: 365 days)
4. **Outdated Versions**: (with `--check-outdated`) Current version is significantly behind the latest released version
5. **Unknown Status**: Non-GitHub dependencies that couldn't be resolved (shown with ❓)

## Rate Limiting

Authenticated requests get 5,000 GitHub API requests/hour vs 60 for unauthenticated. The tool uses caching to minimize API calls and provides clear error messages when rate limits are exceeded.

## Acknowledgments

Inspired by [cargo-unmaintained](https://github.com/trailofbits/cargo-unmaintained) for the Rust ecosystem.
