# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=tornado-nginx-go-backend
BINARY_PATH=./bin/$(BINARY_NAME)

# Build flags
BUILD_FLAGS=-a -installsuffix cgo
LDFLAGS=-w -s

# Docker
DOCKER_IMAGE=tornado-nginx-go-backend
DOCKER_TAG=latest

.PHONY: all build clean test deps docker-build docker-run docker-stop help

# Default target
all: clean deps test build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_PATH) cmd/server/main.go

# Build for current OS
build-local:
	@echo "Building $(BINARY_NAME) for current OS..."
	@mkdir -p bin
	$(GOBUILD) -o $(BINARY_PATH) cmd/server/main.go

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Install development dependencies
deps-dev:
	@echo "Installing development dependencies..."
	$(GOGET) -u golang.org/x/tools/cmd/goimports
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Run the application locally
run:
	@echo "Running $(BINARY_NAME)..."
	$(GOCMD) run cmd/server/main.go

# Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	@echo "Starting development server with hot reload..."
	air

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Docker run
docker-run:
	@echo "Running Docker container..."
	docker-compose up --build

# Docker stop
docker-stop:
	@echo "Stopping Docker containers..."
	docker-compose down

# Docker clean
docker-clean:
	@echo "Cleaning Docker images and containers..."
	docker-compose down --rmi all --volumes --remove-orphans

# Database migration (placeholder for future use)
migrate-up:
	@echo "Running database migrations..."
	# Add migration commands here when needed

migrate-down:
	@echo "Rolling back database migrations..."
	# Add rollback commands here when needed

# Generate documentation
docs:
	@echo "Generating documentation..."
	$(GOCMD) doc -all ./... > docs/api.md

# Security scan
security:
	@echo "Running security scan..."
	gosec ./...

# Performance benchmark
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BINARY_PATH) /usr/local/bin/

# Uninstall the binary
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f /usr/local/bin/$(BINARY_NAME)

# Create release build
release: clean deps test
	@echo "Creating release build..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-amd64 cmd/server/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-darwin-amd64 cmd/server/main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-windows-amd64.exe cmd/server/main.go

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, download deps, test, and build"
	@echo "  build        - Build the application for Linux"
	@echo "  build-local  - Build the application for current OS"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  deps         - Download dependencies"
	@echo "  deps-dev     - Install development dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  run          - Run the application locally"
	@echo "  dev          - Run with hot reload (requires air)"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  docker-stop  - Stop Docker containers"
	@echo "  docker-clean - Clean Docker images and containers"
	@echo "  docs         - Generate documentation"
	@echo "  security     - Run security scan"
	@echo "  bench        - Run benchmarks"
	@echo "  install      - Install binary to /usr/local/bin"
	@echo "  uninstall    - Remove binary from /usr/local/bin"
	@echo "  release      - Create release builds for multiple platforms"
	@echo "  help         - Show this help message"