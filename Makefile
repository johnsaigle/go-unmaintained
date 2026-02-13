# go-unmaintained Makefile

# Variables
BINARY_NAME=go-unmaintained
MAIN_PACKAGE=.
BUILD_DIR=./bin
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*")
CACHE_FILE=./pkg/popular/data/popular-packages.json
CACHE_RELEASE_TAG=cache-data
REPO=johnsaigle/go-unmaintained

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
.PHONY: build
build: ensure-cache ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Binary built at $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-dev
build-dev: ensure-cache ## Build the binary for development (with debug info)
	@echo "Building $(BINARY_NAME) for development..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Development binary built at $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: install
install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@go install $(MAIN_PACKAGE)
	@echo "$(BINARY_NAME) installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

# Run targets
.PHONY: run
run: ## Run the application
	@go run $(MAIN_PACKAGE) $(ARGS)

.PHONY: run-help
run-help: ## Run the application with --help flag
	@go run $(MAIN_PACKAGE) --help

.PHONY: run-verbose
run-verbose: ## Run the application with verbose output
	@go run $(MAIN_PACKAGE) --verbose $(ARGS)

# Test targets
.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	@go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	@go test -race -v ./...

.PHONY: bench
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Lint and format targets
.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w $(GO_FILES) 2>/dev/null || echo "goimports not found, using go fmt only"

.PHONY: fmt-check
fmt-check: ## Check if code is formatted
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l $(GO_FILES))" ]; then \
		echo "The following files are not properly formatted:"; \
		gofmt -l $(GO_FILES); \
		exit 1; \
	fi
	@echo "All files are properly formatted"

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

.PHONY: check
check: fmt-check vet lint test ## Run all checks (format, vet, lint, test)

# Dependency management
.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

.PHONY: deps-update
deps-update: ## Update all dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

.PHONY: deps-vendor
deps-vendor: ## Create vendor directory
	@echo "Creating vendor directory..."
	@go mod vendor

# Clean targets
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@go clean -cache -testcache -modcache

.PHONY: clean-build
clean-build: ## Clean only build directory
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)

# Development targets
.PHONY: dev-setup
dev-setup: ## Set up development environment
	@echo "Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development tools installed"

.PHONY: example
example: build ## Run example analysis on current project
	@echo "Running example analysis on current project..."
	@$(BUILD_DIR)/$(BINARY_NAME) --verbose --target .

.PHONY: example-with-token
example-with-token: build ## Run example with GitHub token (requires PAT env var)
	@if [ -z "$$PAT" ]; then \
		echo "Please set PAT environment variable"; \
		exit 1; \
	fi
	@echo "Running example analysis with GitHub token..."
	@$(BUILD_DIR)/$(BINARY_NAME) --verbose --target . --token $$PAT

# Docker targets (optional)
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(BINARY_NAME):latest .

# Release targets
.PHONY: build-all
build-all: ## Build binaries for all platforms
	@echo "Building binaries for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	@GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	@GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	@echo "All platform binaries built in $(BUILD_DIR)/"

.PHONY: version
version: ## Show version information
	@echo "Go version: $$(go version)"
	@echo "Git commit: $$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Build date: $$(date -u +'%Y-%m-%d %H:%M:%S UTC')"

# Popular cache management
.PHONY: ensure-cache
ensure-cache: ## Ensure cache file exists (creates empty seed if missing, e.g. after git clean)
	@mkdir -p $(dir $(CACHE_FILE))
	@if [ ! -f $(CACHE_FILE) ]; then \
		echo "[]" > $(CACHE_FILE); \
		echo "Created empty cache seed at $(CACHE_FILE)"; \
	fi

.PHONY: download-cache
download-cache: ## Download latest popular packages cache from GitHub release
	@echo "Downloading popular packages cache from release $(CACHE_RELEASE_TAG)..."
	@mkdir -p $(dir $(CACHE_FILE))
	@if gh release download $(CACHE_RELEASE_TAG) --repo $(REPO) --pattern "popular-packages.json" --output $(CACHE_FILE) --clobber 2>/dev/null; then \
		echo "✓ Cache downloaded ($$(wc -c < $(CACHE_FILE)) bytes)"; \
	else \
		echo "⚠ Could not download cache (release may not exist yet). Using empty seed."; \
		echo "[]" > $(CACHE_FILE); \
	fi

.PHONY: upload-cache
upload-cache: ## Upload popular packages cache as GitHub release artifact (CI only)
	@if [ ! -f $(CACHE_FILE) ] || [ "$$(cat $(CACHE_FILE))" = "[]" ]; then \
		echo "Error: No cache data to upload"; \
		exit 1; \
	fi
	@echo "Uploading cache to release $(CACHE_RELEASE_TAG)..."
	@if ! gh release view $(CACHE_RELEASE_TAG) --repo $(REPO) >/dev/null 2>&1; then \
		echo "Creating release $(CACHE_RELEASE_TAG)..."; \
		gh release create $(CACHE_RELEASE_TAG) --repo $(REPO) \
			--title "Popular Packages Cache" \
			--notes "Auto-updated popular packages cache for go:embed. This release is updated nightly by CI." \
			--latest=false; \
	fi
	@gh release upload $(CACHE_RELEASE_TAG) $(CACHE_FILE)#popular-packages.json --repo $(REPO) --clobber
	@echo "✓ Cache uploaded to release $(CACHE_RELEASE_TAG)"

# Popular cache build targets
.PHONY: build-cache
build-cache: ## Build popular packages cache incrementally (requires PAT env var)
	@if [ -z "$$PAT" ]; then \
		echo "Error: PAT environment variable is required"; \
		exit 1; \
	fi
	@echo "Building popular packages cache (incremental: 10 new entries)..."
	@go run ./cmd/cache-builder --new-entries 10 --output ./pkg/popular/data/popular-packages.json --token $$PAT
	@echo "✓ Cache built successfully"

.PHONY: build-cache-bootstrap
build-cache-bootstrap: ## Bootstrap cache with many entries (requires PAT env var)
	@if [ -z "$$PAT" ]; then \
		echo "Error: PAT environment variable is required"; \
		exit 1; \
	fi
	@echo "Bootstrapping popular packages cache (100 new entries)..."
	@go run ./cmd/cache-builder --new-entries 100 --output ./pkg/popular/data/popular-packages.json --token $$PAT
	@echo "✓ Cache built successfully"

.PHONY: build-cache-small
build-cache-small: ## Build cache with a few entries for testing (requires PAT env var)
	@if [ -z "$$PAT" ]; then \
		echo "Error: PAT environment variable is required"; \
		exit 1; \
	fi
	@echo "Building small popular packages cache (5 new entries)..."
	@go run ./cmd/cache-builder --new-entries 5 --output ./pkg/popular/data/popular-packages.json --token $$PAT
	@echo "✓ Cache built successfully"

# Quick development workflow
.PHONY: quick
quick: fmt vet build ## Quick development cycle: format, vet, and build

.PHONY: ci
ci: deps check build ## Continuous Integration target

# Default target when just running 'make'
.DEFAULT_GOAL := build
