# Variables
BINARY_NAME=gojira
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -w -s"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Build directories
DIST_DIR=dist
BIN_DIR=bin

# Platforms
PLATFORMS=darwin linux windows
ARCHITECTURES=amd64 arm64

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[0;33m
NC=\033[0m # No Color

.PHONY: all build clean test coverage fmt vet lint run help

# Default target
all: clean fmt vet test build

# Help target
help:
	@echo "$(GREEN)GoJira Makefile Commands:$(NC)"
	@echo "  $(YELLOW)make build$(NC)         - Build for current platform"
	@echo "  $(YELLOW)make build-all$(NC)     - Build for all platforms"
	@echo "  $(YELLOW)make build-windows$(NC) - Build Windows executables"
	@echo "  $(YELLOW)make build-linux$(NC)   - Build Linux binaries"
	@echo "  $(YELLOW)make build-darwin$(NC)  - Build macOS binaries"
	@echo "  $(YELLOW)make run$(NC)           - Run the application"
	@echo "  $(YELLOW)make test$(NC)          - Run tests"
	@echo "  $(YELLOW)make test-coverage$(NC) - Run tests with coverage"
	@echo "  $(YELLOW)make fmt$(NC)           - Format code"
	@echo "  $(YELLOW)make vet$(NC)           - Run go vet"
	@echo "  $(YELLOW)make lint$(NC)          - Run linter"
	@echo "  $(YELLOW)make clean$(NC)         - Clean build artifacts"
	@echo "  $(YELLOW)make docker-build$(NC)  - Build Docker image"
	@echo "  $(YELLOW)make docker-run$(NC)    - Run with Docker"
	@echo "  $(YELLOW)make deps$(NC)          - Download dependencies"

# Build for current platform
build:
	@echo "$(GREEN)Building for current platform...$(NC)"
	@mkdir -p $(DIST_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME) cmd/gojira/main.go
	@echo "$(GREEN)Build complete: $(DIST_DIR)/$(BINARY_NAME)$(NC)"

# Build for all platforms
build-all: clean
	@echo "$(GREEN)Building for all platforms...$(NC)"
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		for arch in $(ARCHITECTURES); do \
			echo "$(YELLOW)Building for $$platform/$$arch...$(NC)"; \
			output_name=$(DIST_DIR)/$(BINARY_NAME)-$$platform-$$arch; \
			if [ "$$platform" = "windows" ]; then \
				output_name="$$output_name.exe"; \
			fi; \
			GOOS=$$platform GOARCH=$$arch $(GOBUILD) $(LDFLAGS) -o $$output_name cmd/gojira/main.go || true; \
		done; \
	done
	@echo "$(GREEN)All builds complete!$(NC)"

# Platform-specific builds
build-windows:
	@echo "$(GREEN)Building for Windows...$(NC)"
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe cmd/gojira/main.go
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-arm64.exe cmd/gojira/main.go
	@echo "$(GREEN)Windows builds complete!$(NC)"

build-linux:
	@echo "$(GREEN)Building for Linux...$(NC)"
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 cmd/gojira/main.go
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 cmd/gojira/main.go
	@echo "$(GREEN)Linux builds complete!$(NC)"

build-darwin:
	@echo "$(GREEN)Building for macOS...$(NC)"
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 cmd/gojira/main.go
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 cmd/gojira/main.go
	@echo "$(GREEN)macOS builds complete!$(NC)"

# Run the application
run: build
	@echo "$(GREEN)Running GoJira...$(NC)"
	./$(DIST_DIR)/$(BINARY_NAME)

# Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	$(GOCLEAN)
	rm -rf $(DIST_DIR) $(BIN_DIR)
	@echo "$(GREEN)Clean complete!$(NC)"

# Run tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

# Run integration tests
test-integration:
	@echo "$(GREEN)Running integration tests...$(NC)"
	$(GOTEST) -v ./tests/integration/...

# Run benchmarks
test-benchmark:
	@echo "$(GREEN)Running benchmarks...$(NC)"
	$(GOTEST) -bench=. -benchmem ./...

# Format code
fmt:
	@echo "$(YELLOW)Formatting code...$(NC)"
	$(GOFMT) -w .
	@echo "$(GREEN)Format complete!$(NC)"

# Run go vet
vet:
	@echo "$(YELLOW)Running go vet...$(NC)"
	$(GOVET) ./...
	@echo "$(GREEN)Vet complete!$(NC)"

# Run linter (requires golangci-lint)
lint:
	@echo "$(YELLOW)Running linter...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not installed$(NC)" && exit 1)
	golangci-lint run
	@echo "$(GREEN)Lint complete!$(NC)"

# Download dependencies
deps:
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "$(GREEN)Dependencies updated!$(NC)"

# Docker targets
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(BINARY_NAME):$(VERSION) -f docker/Dockerfile .
	docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest
	@echo "$(GREEN)Docker build complete!$(NC)"

docker-run: docker-build
	@echo "$(GREEN)Running with Docker...$(NC)"
	docker run --rm -p 8080:8080 $(BINARY_NAME):latest

docker-push:
	@echo "$(GREEN)Pushing Docker image...$(NC)"
	docker push $(BINARY_NAME):$(VERSION)
	docker push $(BINARY_NAME):latest

# Development setup
dev-setup:
	@echo "$(GREEN)Setting up development environment...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	$(GOMOD) download
	@echo "$(GREEN)Development setup complete!$(NC)"

# Generate checksums for releases
checksums:
	@echo "$(GREEN)Generating checksums...$(NC)"
	@cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "$(GREEN)Checksums generated: $(DIST_DIR)/checksums.txt$(NC)"

# Create release
release: clean build-all checksums
	@echo "$(GREEN)Release artifacts ready in $(DIST_DIR)/$(NC)"
	@ls -la $(DIST_DIR)/

# Install locally
install: build
	@echo "$(GREEN)Installing GoJira...$(NC)"
	@mkdir -p $(HOME)/.local/bin
	@cp $(DIST_DIR)/$(BINARY_NAME) $(HOME)/.local/bin/
	@echo "$(GREEN)GoJira installed to $(HOME)/.local/bin/$(BINARY_NAME)$(NC)"
	@echo "$(YELLOW)Make sure $(HOME)/.local/bin is in your PATH$(NC)"

# Uninstall
uninstall:
	@echo "$(YELLOW)Uninstalling GoJira...$(NC)"
	@rm -f $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "$(GREEN)GoJira uninstalled$(NC)"

.DEFAULT_GOAL := help