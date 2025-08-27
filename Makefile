# go-unmaintained Makefile

# Variables
BINARY_NAME=go-unmaintained
MAIN_PACKAGE=.
BUILD_DIR=./bin
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*" -not -path "./.git/*")

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
.PHONY: build
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Binary built at $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-dev
build-dev: ## Build the binary for development (with debug info)
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
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Please install it: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

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
example-with-token: build ## Run example with GitHub token (requires GITHUB_TOKEN env var)
	@if [ -z "$$GITHUB_TOKEN" ]; then \
		echo "Please set GITHUB_TOKEN environment variable"; \
		exit 1; \
	fi
	@echo "Running example analysis with GitHub token..."
	@$(BUILD_DIR)/$(BINARY_NAME) --verbose --target . --token $$GITHUB_TOKEN

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

# Quick development workflow
.PHONY: quick
quick: fmt vet build ## Quick development cycle: format, vet, and build

.PHONY: ci
ci: deps check build ## Continuous Integration target

# Default target when just running 'make'
.DEFAULT_GOAL := build