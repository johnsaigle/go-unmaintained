# Maintaining the Popular Packages Cache

The binary includes a cache of the most-starred Go repositories, so common dependencies don't need API calls. The cache is a bounded rolling snapshot. Each update can add newly ranked repositories and refresh stale entries, and it drops any repository that falls out of the top 500.

Updating it needs a PAT with write access to this repo (see [tokens.md](tokens.md#cache-building-workflow-maintainers-only)).

```bash
export PAT=your_token_here
make download-cache
make build-cache
make upload-cache
```

To change the limits for a local run:

```bash
make build-cache \
  CACHE_NEW_ENTRIES=20 \
  CACHE_REFRESH_ENTRIES=20 \
  CACHE_MAX_ENTRIES=500 \
  CACHE_STALE_DAYS=90
```
