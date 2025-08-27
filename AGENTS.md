# AGENTS.md

## Project: go-unmaintained
A CLI tool that identifies unmaintained Go packages using heuristics, similar to cargo-unmaintained.

## Critical: The tools that you have access to

- `fabric-ai` -- an AI-powered multipurpose tool. Details are in `FABRIC.md`
    - Use like a bash command: send relevant input from STDIN to the command
        - e.g. `curl example.com/article | fabric-ai --pattern analyze_prose_json`
- **OpenCode Agents** -- specialized AI agents configured for specific tasks (`.opencode/agent/`)

### Workflow

Use a fabric command if you want to perform a specific task that it can do.

Use specialized agents for domain-specific tasks:

### Available Specialized Agents
- **security-auditor** (`.opencode/agent/security-auditor.md`): Performs security audits, focuses on logic bugs, DoS vulnerabilities, input validation, and blockchain security. Use when code needs security review.
- **code-quality** (`.opencode/agent/code-quality.md`): Enforces ultra-secure coding practices with bounded resource management. Use when implementing or reviewing code for quality and security standards.

## Essential Commands

### Development Workflow
```bash
make build          # Build the binary
make test           # Run all tests  
make fmt            # Format code
make vet            # Run static analysis
make quick          # Format, vet, and build
make example        # Run on current project (needs GITHUB_TOKEN)
```

### Project Structure
- `cmd/root.go` - CLI command structure
- `pkg/parser/` - Go module parsing  
- `pkg/github/` - GitHub API client
- `pkg/analyzer/` - Core heuristics engine
- `PRD.md` - Product requirements and roadmap
- `Makefile` - Build automation
