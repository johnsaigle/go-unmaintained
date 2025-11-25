# Check Unmaintained Dependencies - GitHub Action

This action scans your Go project for unmaintained dependencies using heuristics.

## Features

- Detects archived, missing, and inactive repositories
- Identifies outdated package versions
- Fast concurrent analysis with smart caching

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

## Example Workflows

### Basic CI Check

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

### Fail Only on Direct Dependencies

```yaml
- name: Check dependencies
  id: check
  uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    fail-on-found: false

- name: Fail if direct dependencies unmaintained
  if: steps.check.outputs.direct-count > 0
  run: exit 1
```

## Detection Criteria

- **Archived**: Repository marked as archived
- **Not Found**: Repository doesn't exist or was deleted  
- **Inactive**: No commits in the last 365 days (configurable with `max-age`)
- **Outdated**: Using old version (requires `check-outdated: true`)

## Performance

Default 5 concurrent workers. Typical runtime: 30-60 seconds for 50 dependencies.

If you hit rate limits, reduce `concurrency` or use a personal access token instead of the default `${{ github.token }}`.

## License

See [LICENSE](../../../LICENSE) file in the repository root.

## Contributing

Issues and pull requests welcome at https://github.com/johnsaigle/go-unmaintained
