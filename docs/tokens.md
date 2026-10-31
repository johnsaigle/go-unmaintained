# GitHub Token Setup

`go-unmaintained` requires a GitHub Personal Access Token (PAT). The default Actions `GITHUB_TOKEN` does not have enough permissions for it.

Without a token, GitHub allows 60 API requests per hour. With one, the limit is 5,000.

## CLI

1. [Create a PAT](https://github.com/settings/tokens):
   - **Fine-grained (recommended):** Repository access set to "Public Repositories (read-only)". No other permissions are needed.
   - **Classic:** no scopes (or `public_repo`).
2. Pass it in with an environment variable or a flag:
   ```bash
   export PAT=your_token_here
   # or
   go-unmaintained --token your_token_here
   ```

## GitHub Actions (check action)

1. [Create a PAT](https://github.com/settings/tokens):
   - **Fine-grained:** Repository access set to "All repositories" or "Only select repositories", with **Metadata** permission (read-only, set automatically).
   - **Classic:** `public_repo` scope (or no scopes).
2. Add it as a repository secret named `PAT` under **Settings → Secrets and variables → Actions → New repository secret**.
3. Reference it in your workflow:
   ```yaml
   github-token: ${{ secrets.PAT }}
   ```

## Cache-building workflow (maintainers only)

1. [Create a PAT](https://github.com/settings/tokens):
   - **Fine-grained:** Repository access set to "Only select repositories" with this repo selected, and **Contents** set to **Read and write**.
   - **Classic:** `repo` scope.
2. Add it as a secret named `PAT`, using the same steps as above.
