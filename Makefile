.PHONY: help build test lint fmt vet clean run install deps tidy

# Variables
BINARY_NAME=k8s-doctor
BUILD_DIR=bin
GO=go
GOFLAGS=-v
LDFLAGS=-w -s
COVERAGE_FILE=coverage.out

# Colors for output
CYAN=\033[0;36m
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m # No Color

help: ## Show this help message
	@echo "$(CYAN)SRE Toolkit - Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'

deps: ## Download dependencies
	@echo "$(CYAN)Downloading dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod verify

tidy: ## Tidy up go.mod and go.sum
	@echo "$(CYAN)Tidying modules...$(NC)"
	$(GO) mod tidy

build: ## Build the k8s-doctor binary
	@echo "$(CYAN)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/k8s-doctor
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

build-all: ## Build all tools
	@echo "$(CYAN)Building all tools...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/k8s-doctor ./cmd/k8s-doctor
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/alert-analyzer ./cmd/alert-analyzer
	@echo "$(GREEN)All builds complete$(NC)"

install: build ## Install the binary to GOPATH/bin
	@echo "$(CYAN)Installing $(BINARY_NAME)...$(NC)"
	$(GO) install ./cmd/k8s-doctor
	@echo "$(GREEN)Installation complete$(NC)"

test: ## Run tests
	@echo "$(CYAN)Running tests...$(NC)"
	$(GO) test -v -race -coverprofile=$(COVERAGE_FILE) ./...
	@echo "$(GREEN)Tests complete$(NC)"

test-coverage: test ## Run tests with coverage report
	@echo "$(CYAN)Generating coverage report...$(NC)"
	$(GO) tool cover -html=$(COVERAGE_FILE)

lint: ## Run golangci-lint
	@echo "$(CYAN)Running linter...$(NC)"
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run --timeout=5m; \
		echo "$(GREEN)Linting complete$(NC)"; \
	else \
		echo "$(RED)golangci-lint not installed. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin$(NC)"; \
		exit 1; \
	fi

fmt: ## Format code
	@echo "$(CYAN)Formatting code...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)Formatting complete$(NC)"

vet: ## Run go vet
	@echo "$(CYAN)Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)Vet complete$(NC)"

clean: ## Clean build artifacts
	@echo "$(CYAN)Cleaning...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_FILE)
	@echo "$(GREEN)Clean complete$(NC)"

run: build ## Build and run k8s-doctor
	@echo "$(CYAN)Running $(BINARY_NAME)...$(NC)"
	@$(BUILD_DIR)/$(BINARY_NAME) --help

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "$(GREEN)All checks passed!$(NC)"

.DEFAULT_GOAL := help
