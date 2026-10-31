# Configuration

Run `go-unmaintained --help` for the full list of flags.

## Detection

| Flag | Default | Effect |
|---|---|---|
| `--max-age <days>` | `365` | Days without commits before a repo counts as inactive |
| `--check-outdated` | off | Flag dependencies that are far behind the latest release |
| `--check-membership` | off | Flag modules that no longer match the `module` directive in their repo (renamed or removed) |
| `--resolve-unknown` | off | Try deeper analysis of dependencies hosted outside GitHub, GitLab, and Bitbucket |
| `--fail-fast` | off | Exit as soon as the first unmaintained package is found |
| `--no-exit-code` | off | Always exit `0`, even when unmaintained packages are found |

### Unknown status

Some dependencies are hosted where the tool can't fully analyze them. These are listed as **unknown** (`?`) and don't affect the exit code. Well-known packages such as `golang.org/x/*` are recognized automatically. To analyze the rest more deeply, use `--resolve-unknown`.

## Output

| Flag | Values |
|---|---|
| `--format` | `console` (default), `json`, `github-actions`, `golangci-lint` |
| `--color` | `auto` (default; respects `NO_COLOR`), `always`, `never` |
| `--verbose` | Also list maintained packages and extra details |
| `--tree` | Show the dependency path for each indirect dependency |

The `golangci-lint` format always exits `0`.

### Example console output

```
📦 Project: github.com/example/myapp
🔍 Analyzing 25 dependencies (concurrent: 5 workers)...

UNMAINTAINED — DIRECT DEPENDENCIES (3)
────────────────────────────────────────────
Listed in your go.mod; upgrade or replace these directly.

  ✗ github.com/abandoned/old-lib — Repository is archived
     Repo: abandoned/old-lib (https://github.com/abandoned/old-lib)
     No commits in 1024 days
     Archived: no new commits are possible

  ✗ github.com/missing/gone — Repository not found
     Repo: missing/gone (https://github.com/missing/gone)

  ✗ github.com/old/version — Using outdated version v1.2.0 (latest: v2.1.0)
     Repo: old/version (https://github.com/old/version)

UNMAINTAINED — INDIRECT DEPENDENCIES (1)
────────────────────────────────────────────
Pulled in by your dependencies; fix the parent package.

  ✗ github.com/stale/inactive — Repository inactive for 500 days
     Repo: stale/inactive (https://github.com/stale/inactive)
     No commits in 500 days
     Required by: myapp → dep-a → github.com/stale/inactive

UNKNOWN STATUS (1)
────────────────────────────────────────────

  ? golang.org/x/tools — Active non-GitHub dependency (golang.org): Official Go extended package

══════════════════════════════════════════════════
ANALYSIS SUMMARY
══════════════════════════════════════════════════
Total dependencies analyzed: 25

Unmaintained: 4 (3 direct, 1 indirect)
   Archived repositories: 1
   Not found/deleted: 1
   Stale/inactive: 1
   Outdated versions: 1

Unknown status: 1
Maintained: 20
```

## Performance

| Flag | Default | Effect |
|---|---|---|
| `--concurrency <n>` | `5` | Number of parallel API workers. Lower it if you're hitting rate limits. |
| `--sync` | off | Process dependencies one at a time (slowest, uses the least memory) |
| `--cache-duration <hours>` | `24` | How long API responses stay cached on disk |
| `--no-cache` | off | Turn off the disk cache |

API responses are cached in `~/.cache/go-unmaintained/` (or your system's cache directory). Repeat runs against the same project make very few API calls.

## Rate limits

The GitHub API allows 5,000 authenticated requests per hour, or 60 without a token. The tool retries with backoff and prints a clear error if you run out. Caching and the embedded popular-package cache keep the number of requests low. See [tokens.md](tokens.md) for token setup.
