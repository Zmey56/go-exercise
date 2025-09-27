# Go Exercise Makefile
.PHONY: help build test coverage clean docker-build docker-up docker-down lint fmt vet benchmark

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the server binary
	@echo "🔨 Building server..."
	@go build -o server ./cmd/server
	@echo "✅ Build complete"

build-docker: ## Build optimized binary for Docker
	@echo "🔨 Building optimized server for Docker..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags='-w -s -extldflags "-static"' \
		-a -installsuffix cgo \
		-o server ./cmd/server
	@echo "✅ Docker build complete"

# Test targets
test: ## Run all tests
	@echo "🧪 Running tests..."
	@go test -v ./...
	@echo "✅ All tests passed"

test-short: ## Run tests with short flag
	@echo "🧪 Running short tests..."
	@go test -short -v ./...
	@echo "✅ Short tests passed"

# Coverage targets
coverage: ## Run tests with coverage report
	@echo "📊 Running tests with coverage..."
	@mkdir -p coverage
	@go test -cover -coverprofile=coverage/coverage.out -covermode=atomic ./... -v
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@go tool cover -func=coverage/coverage.out > coverage/coverage.txt
	@echo "📈 Generating coverage summary..."
	@go tool cover -func=coverage/coverage.out | tail -n 1 | awk '{print "Overall coverage: " $$3}' | tee coverage/summary.txt
	@echo "📊 Coverage reports generated:"
	@echo "  - HTML: coverage/coverage.html"
	@echo "  - Text: coverage/coverage.txt"
	@echo "  - Raw:  coverage/coverage.out"
	@echo "✅ Coverage analysis complete"

coverage-html: coverage ## Generate and open HTML coverage report
	@echo "🌐 Opening coverage report in browser..."
	@open coverage/coverage.html 2>/dev/null || xdg-open coverage/coverage.html 2>/dev/null || echo "Please open coverage/coverage.html manually"

# Quality targets
lint: ## Run linters
	@echo "🔍 Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, running go vet instead"; \
		$(MAKE) vet; \
	fi
	@echo "✅ Linting complete"

fmt: ## Format Go code
	@echo "🎨 Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

vet: ## Run go vet
	@echo "🔍 Running go vet..."
	@go vet ./...
	@echo "✅ Vet analysis complete"

# Performance targets
benchmark: ## Run benchmarks
	@echo "⚡ Running benchmarks..."
	@go test -bench=. -benchmem ./... | tee coverage/benchmarks.txt
	@echo "✅ Benchmarks complete"

# Docker targets
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	@docker-compose build --no-cache
	@echo "✅ Docker image built"

docker-up: ## Start Docker containers
	@echo "🚀 Starting Docker containers..."
	@docker-compose up -d
	@echo "✅ Containers started"
	@echo "🌐 API available at: http://localhost:8080"
	@echo "📚 Swagger UI at: http://localhost:8080/docs"

docker-down: ## Stop Docker containers
	@echo "🛑 Stopping Docker containers..."
	@docker-compose down -v
	@echo "✅ Containers stopped"

docker-logs: ## Show Docker logs
	@docker-compose logs -f

docker-restart: docker-down docker-up ## Restart Docker containers

# Development targets
dev: ## Start development server
	@echo "🚀 Starting development server..."
	@go run ./cmd/server

dev-watch: ## Start development server with hot reload (requires air)
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not found, install with: go install github.com/cosmtrek/air@latest"; \
		$(MAKE) dev; \
	fi

# Maintenance targets
clean: ## Clean build artifacts and coverage reports
	@echo "🧹 Cleaning up..."
	@rm -f server
	@rm -rf coverage/
	@docker-compose down -v 2>/dev/null || true
	@docker system prune -f 2>/dev/null || true
	@echo "✅ Cleanup complete"

deps: ## Download and tidy dependencies
	@echo "📦 Managing dependencies..."
	@go mod download
	@go mod tidy
	@go mod verify
	@echo "✅ Dependencies updated"

# CI/CD targets
ci: fmt vet test coverage ## Run CI pipeline locally
	@echo "🎯 CI pipeline complete"

# Health check
health: ## Check if API is running
	@echo "🏥 Checking API health..."
	@curl -s http://localhost:8080/health | jq . || echo "API not responding"
	@curl -s http://localhost:8080/ready | jq . || echo "Readiness check failed"