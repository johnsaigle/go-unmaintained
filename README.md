# go-unmaintained

[![Dependency Health](https://github.com/johnsaigle/go-unmaintained/actions/workflows/self-check.yml/badge.svg)](https://github.com/johnsaigle/go-unmaintained/actions/workflows/self-check.yml)

**Supply-chain risk detection for Go dependencies.**

`govulncheck` reports the vulnerabilities that are already known. It can't warn you about a dependency that will never be patched. An archived repo, a deleted project, or a maintainer who left years ago means the next CVE in that code stays open. A dependency nobody owns can also be taken over by someone else. `go-unmaintained` flags these dependencies before they become an incident.

```
$ go-unmaintained

UNMAINTAINED — DIRECT DEPENDENCIES (2)
  ✗ github.com/abandoned/old-lib — Repository is archived
  ✗ github.com/missing/gone      — Repository not found

UNMAINTAINED — INDIRECT DEPENDENCIES (1)
  ✗ github.com/stale/inactive    — Stale: no repository updates in 500 days
     Required by: myapp → dep-a → github.com/stale/inactive
```

## What it catches

| Signal | Risk |
|---|---|
| **Archived** | The repo is read-only, so future vulnerabilities will never be patched. |
| **Not found** | The source was deleted or made private. You can no longer audit it, and its namespace may be open for someone else to claim. |
| **Inactive** | No commits within `--max-age` days (default 365). Security fixes are unlikely to arrive. |
| **Outdated** | You're far behind the latest release and may be missing fixes (`--check-outdated`). |
| **Not in repo** | The module was renamed or removed upstream, so your code points at an orphaned path (`--check-membership`). |

Works with GitHub, GitLab, and Bitbucket. It reports direct and indirect dependencies separately. That covers the whole dependency tree and shows which risks you can fix yourself and which ones to raise with a parent package.

## Install

```bash
go install github.com/johnsaigle/go-unmaintained@51c887651b0770b132e8cc63947f186a5dd6ba93
```

The command pins an exact commit, so you know what you're installing and the build can be reproduced. Check the [commit history](https://github.com/johnsaigle/go-unmaintained/commits/main) for newer revisions.

Or build from source with `make build` (the binary goes to `./bin/`).

## Quick start

```bash
export PAT=<github-token>   # read-only, no scopes needed (see docs/tokens.md)
go-unmaintained             # scan the current module
```

The exit code is `1` when an unmaintained dependency is found. Run it in CI to stop risky dependencies from being merged.

### Useful flags

```bash
go-unmaintained --target ./path/to/module   # scan another project
go-unmaintained --check-outdated            # also flag stale versions
go-unmaintained --check-membership          # detect renamed/removed modules
go-unmaintained --tree                      # show why indirect deps are pulled in
go-unmaintained --format json               # or: github-actions, golangci-lint
go-unmaintained --max-age 730               # change the inactivity threshold
```

Run `go-unmaintained --help` for the full list.

## GitHub Action

```yaml
- uses: johnsaigle/go-unmaintained/.github/actions/check@v1
  with:
    github-token: ${{ secrets.PAT }}
```

The default `GITHUB_TOKEN` won't work here. You need a PAT, as described in [docs/tokens.md](docs/tokens.md). More examples are in the [action README](.github/actions/check/README.md).

## Docs

- [Token setup](docs/tokens.md): CLI, Actions, and maintainer tokens
- [Configuration](docs/configuration.md): output formats, concurrency, caching, rate limits, and unknown/non-GitHub packages
- [Maintaining the package cache](docs/cache.md): for project maintainers

## Acknowledgments

Inspired by [cargo-unmaintained](https://github.com/trailofbits/cargo-unmaintained) for Rust.
