# TODO

## Research a way to mitigate rate-limiting

- Caching analyses of packages/versions
- Batching requests to GH by using the API more cleverly

## Add detection and error handling for GH token rate-limiting:

Example error when running in verbose mode
```
✅ github.com/prometheus/common - Analysis error: failed to get repository info: failed to fetch repository: GET https://api.github.com/repos/prometheus/common: 403 API rate limit of 60 still exceeded until 2025-08-27 12:15:49 -0400 EDT, not making remote request. [rate reset in 51m59s]
```

## Parse a go.mod file and check each of its dependencies to see if they are outdated.

## Add README

- Specify that a classic github token needs to be created (no permissions) in order to see the archived status of a target repo
