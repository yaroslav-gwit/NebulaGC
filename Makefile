# NebulaGC Makefile
#
# This Makefile provides common development tasks for the NebulaGC project.

.PHONY: help build test test-coverage lint format generate migrate clean dev-server dev-daemon install-tools

# Default target
help:
	@echo "NebulaGC Development Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build           - Build all binaries"
	@echo "  test            - Run all tests"
	@echo "  test-coverage   - Generate coverage report"
	@echo "  lint            - Run linters (requires golangci-lint)"
	@echo "  format          - Format all Go code"
	@echo "  generate        - Run code generators (sqlc)"
	@echo "  migrate         - Apply database migrations"
	@echo "  clean           - Clean build artifacts"
	@echo "  dev-server      - Start development server"
	@echo "  dev-daemon      - Start development daemon"
	@echo "  install-tools   - Install required development tools"

# Install required development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "Tools installed successfully"

# Build all binaries
build:
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o bin/nebulagc-server ./server/cmd/nebulagc-server
	go build -o bin/nebulagc ./cmd/nebulagc
	@echo "Build complete: bin/nebulagc-server, bin/nebulagc"

# Run all tests
test:
	@echo "Running tests..."
	go test -race -v ./...

# Generate coverage report
test-coverage:
	@echo "Generating coverage report..."
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin"; \
	fi

# Format all Go code
format:
	@echo "Formatting code..."
	gofmt -w -s .
	@echo "Code formatted"

# Run code generators
generate:
	@echo "Running code generators..."
	@if [ -f server/sqlc.yaml ]; then \
		cd server && sqlc generate; \
		echo "SQLc generation complete"; \
	else \
		echo "SQLc config not found, skipping..."; \
	fi

# Apply database migrations
migrate:
	@echo "Applying database migrations..."
	@if [ ! -f tmp/nebula.db ]; then \
		mkdir -p tmp; \
		echo "Creating new database..."; \
	fi
	goose -dir server/migrations sqlite3 tmp/nebula.db up
	@echo "Migrations applied"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf tmp/
	rm -f coverage.txt coverage.html
	rm -f *.db *.db-shm *.db-wal
	@echo "Clean complete"

# Start development server (master mode)
dev-server:
	@echo "Starting development server..."
	@mkdir -p tmp
	@if [ ! -f tmp/nebula.db ]; then \
		$(MAKE) migrate; \
	fi
	NEBULAGC_HMAC_SECRET="dev-secret-key-minimum-32-characters-long" \
	NEBULAGC_LOG_LEVEL=debug \
	NEBULAGC_LOG_FORMAT=console \
	NEBULAGC_INSTANCE_ID=dev-master \
	./bin/nebulagc-server serve --master --http :8080 --db ./tmp/nebula.db

# Start development daemon
dev-daemon:
	@echo "Starting development daemon..."
	@if [ ! -f dev_config.json ]; then \
		echo "dev_config.json not found. Create it first."; \
		exit 1; \
	fi
	NEBULAGC_LOG_LEVEL=debug \
	NEBULAGC_LOG_FORMAT=console \
	./bin/nebulagc daemon --config dev_config.json

# Quick dev workflow: clean, build, test
dev: clean build test
	@echo "Development build and test complete"

# CI workflow: format, lint, test with coverage
ci: format lint test-coverage
	@echo "CI checks complete"
