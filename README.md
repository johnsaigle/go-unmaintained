# go-unmaintained

> [!WARNING]
> Largely vibe-coded, trust at your own peril.

A CLI tool that identifies unmaintained Go packages using heuristics.

## Features

- 🔍 **Dependency Analysis**: Scans `go.mod` files to identify potentially unmaintained dependencies
- 🏗️ **Multi-Platform Support**: Supports GitHub, GitLab, Bitbucket, and well-known Go modules
- 📊 **Multiple Heuristics**: Detects archived repositories, missing packages, inactive projects, and outdated versions  
- 🔗 **Advanced Resolution**: Resolves vanity URLs, Go module proxy, and custom domains
- 💾 **Smart Rate Limiting**: Handles GitHub API rate limits gracefully with proper error messages
- ⚡ **Fast Processing**: Concurrent analysis enabled by default for optimal performance
- 📋 **Flexible Output**: Supports both console and JSON output formats

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

## GitHub Token Setup

**Important**: To check repository archival status and access detailed GitHub information, you need a GitHub personal access token.

### Creating a Classic GitHub Token

1. Go to [GitHub Settings > Developer settings > Personal access tokens > Tokens (classic)](https://github.com/settings/tokens)
2. Click "Generate new token (classic)"
3. Give it a descriptive name like "go-unmaintained"
4. **No scopes/permissions are required** - leave all checkboxes unchecked
5. Click "Generate token"
6. Copy the token and store it securely

### Using the Token

Set the token as an environment variable:

```bash
export GITHUB_TOKEN=your_token_here
```

Or pass it directly via the `--token` flag:

```bash
go-unmaintained --token your_token_here
```

**Note**: The token is validated when the tool starts. If it's invalid, expired, or lacks necessary permissions, you'll get a clear error message.

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

### Advanced Options

```bash
# Verbose output with detailed information
go-unmaintained --verbose

# Check for outdated versions in addition to unmaintained repos
go-unmaintained --check-outdated

# Enable resolution of non-GitHub dependencies (GitLab, Bitbucket, etc.)
go-unmaintained --resolve-unknown

# Custom concurrency level (default: 5 concurrent workers)
go-unmaintained --concurrency 10

# Disable concurrent processing (use sequential mode)
go-unmaintained --sync

# Custom age threshold (default: 365 days)
go-unmaintained --max-age 180

# JSON output for programmatic use
go-unmaintained --json

# Fail fast - exit as soon as first unmaintained package is found
go-unmaintained --fail-fast

# Analyze a single package
go-unmaintained --package github.com/example/package
```

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

## Supported Hosting Providers

- ✅ **GitHub**: Full analysis with API integration
- ✅ **GitLab**: Repository analysis via GitLab API
- ✅ **Bitbucket**: Repository analysis via Bitbucket API
- ✅ **Well-known Go modules**: golang.org/x/, google.golang.org/, k8s.io/, etc.
- ✅ **Go Module Proxy**: Verification via proxy.golang.org
- ✅ **Vanity URLs**: Custom domain resolution

## Rate Limiting

The tool handles GitHub API rate limiting intelligently:

- **Authentication**: Authenticated requests get 5,000 requests/hour vs 60 for unauthenticated
- **Error Handling**: Clear error messages when rate limits are exceeded, including reset time
- **Graceful Degradation**: Some features may be skipped if rate limits are hit during analysis

### Future Rate Limiting Improvements

Planned improvements for better rate limit handling:

- **Caching**: Store repository analysis results to avoid repeated API calls
- **Request Batching**: Use GitHub's GraphQL API for more efficient bulk operations
- **Smart Scheduling**: Distribute API calls over time to stay within limits

## Development

### Prerequisites

- Go 1.21 or later
- `golangci-lint` for linting
- `goimports` for import formatting

### Setup Development Environment

```bash
make dev-setup
```

### Common Commands

```bash
# Quick development cycle (format, vet, build)
make quick

# Run all checks (format, vet, lint, test)
make check

# Run with verbose output
make run-verbose

# Build for all platforms
make build-all

# Run example analysis (requires GITHUB_TOKEN)
make example
```

### Testing

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Run tests with race detection
make test-race
```

## Exit Codes

- `0`: Success (no unmaintained packages found)
- `1`: Unmaintained packages detected
- `2`: Tool error (invalid arguments, missing token, etc.)

Use `--no-exit-code` to always exit with code 0, useful for CI environments where you want to collect results without failing the build.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make check` to ensure code quality
5. Submit a pull request

## Acknowledgments

Inspired by [cargo-unmaintained](https://github.com/trailofbits/cargo-unmaintained) for the Rust ecosystem.
