# GitHub Actions Integration

This document describes the GitHub Actions integration for `go-unmaintained`.

## Overview

The tool now supports running as a GitHub Action for automated dependency checking in CI/CD pipelines.

## Features

### ✅ Implemented

1. **JSON Output Format** - Structured output for programmatic processing
2. **GitHub Actions Wrapper** - Composite action for easy integration
3. **GitHub Annotations** - Inline PR annotations for violations
4. **Example Workflows** - Ready-to-use templates for common scenarios
5. **Comprehensive Documentation** - Action README with all options

### 🎯 Key Capabilities

- **Automated Checks**: Run on push, PR, or schedule
- **PR Comments**: Post detailed reports as PR comments
- **Issue Creation**: Auto-create issues for weekly audits
- **Artifacts**: Save results as downloadable artifacts
- **Outputs**: Expose counts and JSON for downstream steps
- **Flexible Failure**: Choose when to fail workflows

## Quick Start

### 1. Basic Check

Add to `.github/workflows/check-deps.yml`:

```yaml
name: Check Dependencies
on: [push, pull_request]

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: johnsaigle/go-unmaintained/.github/actions/check@v1
```

### 2. Enable in Your Repo

```bash
# Copy example workflows
cp .github/workflows/check-dependencies.yml.example .github/workflows/check-dependencies.yml

# Commit and push
git add .github/workflows/check-dependencies.yml
git commit -m "Add dependency checking"
git push
```

## Output Formats

### 1. Console Output (Default)

Human-readable format with colors and emojis:

```
📦 Project: github.com/example/project
🔍 Analyzing 53 dependencies (concurrent: 5 workers)...

🚨 UNMAINTAINED PACKAGES (2 found):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
❌ github.com/old/package (direct) - Repository is archived
   🔗 https://github.com/old/package
   Last commit: 500 days ago
```

### 2. JSON Output (`--json`)

Structured data for automation:

```json
{
  "summary": {
    "TotalDependencies": 53,
    "UnmaintainedCount": 2,
    "DirectUnmaintained": 1,
    "IndirectUnmaintained": 1
  },
  "results": [
    {
      "package": "github.com/old/package",
      "is_unmaintained": true,
      "is_direct": true,
      "reason": "repository_archived",
      "repo_info": {
        "url": "https://github.com/old/package",
        "is_archived": true
      }
    }
  ]
}
```

### 3. GitHub Annotations (`--github-actions`)

Inline annotations in PRs:

```
::error file=go.mod,title=Unmaintained Dependency::github.com/old/package (direct): Repository is archived - https://github.com/old/package
::notice file=go.mod,title=Dependency Path::main → parent → indirect-dep
::warning::Found 2 unmaintained packages (1 direct, 1 indirect)
```

## Action Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `github-token` | GitHub API token | `${{ github.token }}` |
| `max-age` | Days before repo considered inactive | `365` |
| `check-outdated` | Check for outdated versions | `false` |
| `fail-on-found` | Fail if unmaintained found | `true` |
| `concurrency` | Concurrent workers | `5` |
| `target-path` | Project directory | `.` |
| `verbose` | Detailed output | `false` |

## Action Outputs

| Output | Description |
|--------|-------------|
| `unmaintained-count` | Total unmaintained packages |
| `direct-count` | Direct dependencies |
| `indirect-count` | Indirect dependencies |
| `results-json` | Full JSON results |

## Example Workflows

### Weekly Audit with Issue Creation

```yaml
name: Weekly Audit
on:
  schedule:
    - cron: '0 9 * * 1'

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: johnsaigle/go-unmaintained/.github/actions/check@v1
        id: check
        with:
          fail-on-found: false
      
      - if: steps.check.outputs.unmaintained-count > 0
        uses: actions/github-script@v7
        with:
          script: |
            // Create GitHub issue with results
            // See scheduled-audit.yml.example for full implementation
```

### PR Comment with Details

```yaml
name: PR Check
on: pull_request

permissions:
  pull-requests: write

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: johnsaigle/go-unmaintained/.github/actions/check@v1
        id: check
        with:
          fail-on-found: false
      
      - if: steps.check.outputs.unmaintained-count > 0
        uses: actions/github-script@v7
        with:
          script: |
            // Post formatted PR comment
            // See pr-comment.yml.example for full implementation
```

## Files Structure

```
.github/
├── actions/
│   └── check/
│       ├── action.yml           # Composite action definition
│       └── README.md            # Action documentation
└── workflows/
    ├── check-dependencies.yml.example    # Basic CI check
    ├── pr-comment.yml.example           # PR comments
    └── scheduled-audit.yml.example      # Weekly audits
```

## Best Practices

### 1. Token Usage

Use default `${{ github.token }}` for most cases (1000 requests/hour):

```yaml
github-token: ${{ secrets.GITHUB_TOKEN }}
```

For large projects, use personal token (5000 requests/hour):

```yaml
github-token: ${{ secrets.PERSONAL_ACCESS_TOKEN }}
```

### 2. Performance Tuning

Adjust concurrency based on project size:

```yaml
# Small projects (<20 deps)
concurrency: 3

# Medium projects (20-100 deps)
concurrency: 5  # default

# Large projects (>100 deps)
concurrency: 10
```

### 3. Failure Strategy

Choose appropriate failure behavior:

```yaml
# Strict: Fail on any unmaintained dependency
fail-on-found: true

# Lenient: Report but don't fail
fail-on-found: false
```

### 4. Scheduling

Run audits at low-traffic times:

```yaml
# Weekly Monday morning
- cron: '0 9 * * 1'

# Daily at 2 AM
- cron: '0 2 * * *'
```

## Troubleshooting

### Rate Limiting

**Problem**: Hitting GitHub API rate limits

**Solutions**:
1. Reduce concurrency: `concurrency: 3`
2. Use personal access token
3. Enable caching (automatic)
4. Skip version checks: `check-outdated: false`

### Slow Performance

**Problem**: Action takes too long

**Solutions**:
1. Increase concurrency: `concurrency: 10`
2. Use async mode (default)
3. Cache is automatically enabled

### Missing Dependencies

**Problem**: Some dependencies not analyzed

**Solutions**:
1. Check `target-path` points to correct directory
2. Ensure `go.mod` exists
3. Use `verbose: true` for details

## Migration Guide

If upgrading from manual CLI usage:

**Before** (manual):
```bash
go-unmaintained --json > results.json
```

**After** (action):
```yaml
- uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  # Results automatically saved as artifact
```

## Future Enhancements

Planned features:

- [ ] SARIF output for GitHub Code Scanning
- [ ] Comparison mode (diff against main branch)
- [ ] Allow-list configuration file
- [ ] Severity levels (error/warning/info)
- [ ] Badge generation for README
- [ ] Slack/Discord notifications

## Support

- **Issues**: https://github.com/johnsaigle/go-unmaintained/issues
- **Documentation**: `.github/actions/check/README.md`
- **Examples**: `.github/workflows/*.example`

## License

Same as parent project. See LICENSE file.
