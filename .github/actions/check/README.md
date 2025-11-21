# Check Unmaintained Dependencies - GitHub Action

This action scans your Go project for unmaintained dependencies using heuristics.

## Features

- 🔍 Detects archived, missing, and inactive repositories
- 📊 Identifies outdated package versions
- 🔗 Provides repository URLs for easy verification
- 📍 Shows dependency paths for indirect dependencies
- ⚡ Fast concurrent analysis
- 💾 Smart caching to minimize API calls

## Usage

### Basic Example

```yaml
- name: Check for unmaintained dependencies
  uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

### With All Options

```yaml
- name: Check for unmaintained dependencies
  uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    github-token: ${{ secrets.GITHUB_TOKEN }}
    max-age: 365
    check-outdated: true
    fail-on-found: true
    concurrency: 10
    target-path: '.'
    verbose: true
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `github-token` | GitHub token for API access | No | `${{ github.token }}` |
| `max-age` | Max age in days for inactive repos | No | `365` |
| `check-outdated` | Check for outdated versions | No | `false` |
| `fail-on-found` | Fail if unmaintained packages found | No | `true` |
| `concurrency` | Number of concurrent workers | No | `5` |
| `target-path` | Path to Go project | No | `.` |
| `verbose` | Enable verbose output | No | `false` |

## Outputs

| Output | Description | Example |
|--------|-------------|---------|
| `unmaintained-count` | Total unmaintained packages | `5` |
| `direct-count` | Direct unmaintained dependencies | `3` |
| `indirect-count` | Indirect unmaintained dependencies | `2` |
| `results-json` | Full results in JSON format | `{...}` |

## Examples

### Example 1: Basic CI Check

```yaml
name: Check Dependencies
on: [push, pull_request]

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Check unmaintained dependencies
        uses: johnsaigle/go-unmaintained/.github/actions/check@v1
```

### Example 2: PR Comments

```yaml
name: PR Dependency Check
on: pull_request

permissions:
  pull-requests: write

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Check dependencies
        id: check
        uses: johnsaigle/go-unmaintained/.github/actions/check@v1
        with:
          fail-on-found: false
      
      - name: Comment on PR
        if: steps.check.outputs.unmaintained-count > 0
        uses: actions/github-script@v7
        with:
          script: |
            const count = ${{ steps.check.outputs.unmaintained-count }};
            github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body: `⚠️ Found ${count} unmaintained dependencies`
            });
```

### Example 3: Conditional Failure

```yaml
- name: Check dependencies
  id: check
  uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    fail-on-found: false

- name: Fail if direct dependencies are unmaintained
  if: steps.check.outputs.direct-count > 0
  run: |
    echo "Found ${{ steps.check.outputs.direct-count }} direct unmaintained dependencies"
    exit 1
```

### Example 4: Upload Report Artifact

```yaml
- name: Check dependencies
  uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    fail-on-found: false

# Artifact is automatically uploaded to:
# - Name: unmaintained-dependencies-report
# - Path: results.json
# - Retention: 30 days
```

### Example 5: Multiple Projects

```yaml
jobs:
  check-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: johnsaigle/go-unmaintained/.github/actions/check@v1
        with:
          target-path: './backend'
  
  check-cli:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: johnsaigle/go-unmaintained/.github/actions/check@v1
        with:
          target-path: './cli'
```

## Understanding Results

### Output Format

The action produces both human-readable console output and structured JSON:

**Console Output:**
```
🚨 UNMAINTAINED PACKAGES (2 found):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
❌ github.com/old/package (direct) - Repository is archived
   🔗 https://github.com/old/package
   Last commit: 500 days ago
   ⚠️  Repository archived (no new commits possible)
```

**JSON Output** (via `results-json` output):
```json
{
  "summary": {
    "TotalDependencies": 50,
    "UnmaintainedCount": 2,
    "DirectUnmaintained": 1,
    "IndirectUnmaintained": 1
  },
  "results": [...]
}
```

### Detection Criteria

| Criteria | Description |
|----------|-------------|
| **Archived** | Repository marked as archived on hosting platform |
| **Not Found** | Repository doesn't exist or was deleted |
| **Inactive** | No commits in the last 365 days (configurable) |
| **Outdated** | Using old version (with `check-outdated: true`) |

### Dependency Types

- **Direct (🔴)**: Listed in your `go.mod` - you control the version
- **Indirect (🟡)**: Transitive dependency - update parent package

## Performance

- **Concurrent Analysis**: Default 5 workers (configurable)
- **Smart Caching**: Reduces API calls for repeated scans
- **Rate Limiting**: Respects GitHub API limits automatically
- **Typical Runtime**: 30-60 seconds for 50 dependencies

## Troubleshooting

### Rate Limiting

If you hit GitHub API rate limits:

```yaml
- uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    concurrency: 3  # Reduce concurrent requests
```

### Large Projects

For projects with many dependencies:

```yaml
- uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    concurrency: 10  # Increase for faster analysis
    check-outdated: false  # Skip version checks
```

### Authentication

The action uses `${{ github.token }}` by default, which provides 1000 API requests per hour. For larger projects, consider using a personal access token:

```yaml
- uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    github-token: ${{ secrets.PERSONAL_ACCESS_TOKEN }}
```

## License

See [LICENSE](../../../LICENSE) file in the repository root.

## Contributing

Issues and pull requests welcome at https://github.com/johnsaigle/go-unmaintained
