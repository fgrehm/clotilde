.PHONY: help build test test-watch install clean lint fmt coverage vendor setup-hooks

# Build variables
BASE_VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.0")
GIT_TAG := $(shell git describe --exact-match --tags 2>/dev/null)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | awk '{print $$3}')

# If building from a git tag, use it. Otherwise append -dev+timestamp
ifeq ($(GIT_TAG),)
	VERSION := $(BASE_VERSION)-dev+$(shell date -u +"%Y%m%d%H%M%S")
else
	VERSION := $(GIT_TAG)
endif

LDFLAGS := -X 'github.com/fgrehm/clotilde/cmd.version=$(VERSION)' \
           -X 'github.com/fgrehm/clotilde/cmd.commit=$(COMMIT)' \
           -X 'github.com/fgrehm/clotilde/cmd.date=$(DATE)' \
           -X 'github.com/fgrehm/clotilde/cmd.goVersion=$(GO_VERSION)'

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

setup-hooks: ## Configure git hooks
	@git config core.hooksPath .githooks
	@chmod +x .githooks/*
	@echo "✓ Git hooks configured"

build: ## Build the clotilde binary
	@echo "Building clotilde..."
	@mkdir -p dist
	@go build -ldflags "$(LDFLAGS)" -o dist/clotilde .
	@echo "✓ Built to dist/clotilde"

test: ## Run tests with Ginkgo
	@go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --randomize-suites --fail-on-pending

test-watch: ## Run tests in watch mode
	@echo "Starting test watch mode..."
	@go run github.com/onsi/ginkgo/v2/ginkgo watch -r

install: build ## Install clotilde to ~/.local/bin
	@echo "Installing to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	@if [ -L ~/.local/bin/clotilde ]; then \
		echo "✓ Already installed as symlink (rebuilt binary at dist/clotilde)"; \
	elif [ -e ~/.local/bin/clotilde ]; then \
		rm -f ~/.local/bin/clotilde; \
		cp dist/clotilde ~/.local/bin/clotilde; \
		echo "✓ Replaced existing file and installed to ~/.local/bin/clotilde"; \
	else \
		cp dist/clotilde ~/.local/bin/clotilde; \
		echo "✓ Installed to ~/.local/bin/clotilde"; \
	fi

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@rm -rf dist/
	@rm -f *.test *.out coverage.txt coverage.html
	@find . -name "*.test" -delete
	@echo "✓ Cleaned"

lint: ## Run golangci-lint
	@echo "Running linter..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "⚠ golangci-lint not installed. Install with:"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
		exit 1; \
	fi
	@VERSION=$$(golangci-lint --version | grep -oP 'version \K[0-9]+\.[0-9]+' | head -1); \
	MAJOR=$$(echo $$VERSION | cut -d. -f1); \
	if [ "$$MAJOR" != "2" ]; then \
		echo "⚠ Warning: golangci-lint v$$VERSION detected, but CI uses v2.x"; \
		echo "  Your version may have different linting rules than CI."; \
		echo "  To upgrade: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
		echo ""; \
	fi
	@golangci-lint run ./... && echo "✓ Lint passed"

fmt: ## Format code with gofmt and goimports
	@echo "Formatting code..."
	@gofmt -s -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
		echo "✓ Formatted with gofmt and goimports"; \
	else \
		echo "✓ Formatted with gofmt (goimports not installed)"; \
		echo "  Install goimports with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	@go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --randomize-suites --cover --coverprofile=coverage.txt
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

vendor: ## Update vendored dependencies
	@echo "Vendoring dependencies..."
	@go mod tidy
	@go mod vendor
	@echo "✓ Dependencies vendored"
