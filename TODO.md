# TODO

## Output error? ✅

**Status**: COMPLETED

### Problem Solved
Previously, output was confusing for archived repositories:
1. "Last updated" could show recent dates even for archived repos (because GitHub's `UpdatedAt` changes for metadata)
2. No indication that repos were archived (preventing new commits)
3. No dependency path shown for indirect dependencies

### Solution Implemented

#### ✅ **Improved Time Information**
- **Before**: `Last updated: 3 days ago` (misleading for archived repos)
- **After**: `Last commit: 500 days ago` (accurate code activity)
- Uses `LastCommitAt` for accurate code activity
- Falls back to `Last activity` for repos without commit info

#### ✅ **Archive Indication**
Since GitHub API doesn't provide archive date, we now show:
```bash
⚠️  Repository archived (no new commits possible)
```

#### ✅ **Dependency Path for Indirect Dependencies**
Uses `go mod why` to show the dependency chain:
```bash
❌ github.com/golang/snappy (indirect) - Repository is archived
   🔗 https://github.com/golang/snappy
   Last commit: 500 days ago
   ⚠️  Repository archived (no new commits possible)
   📍 Dependency path: github.com/your-project → github.com/parent-dep → github.com/golang/snappy
```

### Example Output (Improved)

```bash
🚨 UNMAINTAINED PACKAGES (3 found):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
❌ github.com/grpc-ecosystem/go-grpc-prometheus (direct) - Repository is archived
   🔗 https://github.com/grpc-ecosystem/go-grpc-prometheus
   Last commit: 800 days ago
   ⚠️  Repository archived (no new commits possible)

❌ github.com/aws/aws-sdk-go (direct) - Repository is archived
   🔗 https://github.com/aws/aws-sdk-go
   Last commit: 100 days ago
   ⚠️  Repository archived (no new commits possible)

❌ github.com/golang/snappy (indirect) - Repository is archived
   🔗 https://github.com/golang/snappy
   Last commit: 500 days ago
   ⚠️  Repository archived (no new commits possible)
   📍 Dependency path: main → github.com/klauspost/compress → github.com/golang/snappy
```

### Technical Implementation

#### **Parser Enhancement** (`pkg/parser/parser.go`)
- Added `GetDependencyPath()` function using `go mod why`
- Parses dependency chain from go toolchain
- Added `ProjectPath` to Module struct for running go commands

#### **Analyzer Updates** (`pkg/analyzer/analyzer.go`)
- Added `DependencyPath` field to Result
- Removed `ShowDepPath` config (always enabled for indirect)

#### **Output Improvements** (`cmd/root.go`)
- Post-processing step to fetch dependency paths for indirect unmaintained packages
- Shows "Last commit" vs "Last activity" appropriately
- Archive warning for archived repositories
- Dependency path with arrow notation (→) for clarity

### Benefits for Operators

1. **Accurate Activity**: See actual code activity, not metadata changes
2. **Archive Awareness**: Clear indication repos are archived (can't be fixed with PRs)
3. **Remediation Path**: For indirect deps, know which direct dependency to upgrade
4. **Better Decision Making**: Understand if issue is in direct control or transitive

### Remediation Examples

**Direct Dependency (Easy to Fix)**:
```bash
❌ github.com/old/package (direct) - Repository is archived
   → Action: Update go.mod directly
   → Command: go get new/replacement@latest
```

**Indirect Dependency (Harder to Fix)**:
```bash
❌ github.com/old/transitive (indirect) - Repository is archived
   📍 Dependency path: main → github.com/middleware → github.com/old/transitive
   → Action 1: Contact github.com/middleware maintainers
   → Action 2: Upgrade github.com/middleware (may have fixed it)
   → Action 3: Use replace directive as temporary workaround
```
