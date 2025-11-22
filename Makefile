# NebulaGC Makefile
#
# This Makefile provides common development tasks for the NebulaGC project.

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOVET := $(GOCMD) vet

# Build parameters
SERVER_BINARY := bin/nebulagc-server
DAEMON_BINARY := bin/nebulagc
BUILD_FLAGS := -trimpath
LDFLAGS := -ldflags "\
	-s -w \
	-X 'github.com/yaroslav/nebulagc/cmd/nebulagc/cmd.Version=$(VERSION)' \
	-X 'github.com/yaroslav/nebulagc/cmd/nebulagc/cmd.Commit=$(COMMIT)' \
	-X 'github.com/yaroslav/nebulagc/cmd/nebulagc/cmd.BuildDate=$(BUILD_DATE)'"

# Test parameters
TEST_FLAGS := -race -v -timeout 5m
COVERAGE_FILE := coverage.txt
COVERAGE_HTML := coverage.html

# Module directories
MODULES := models pkg sdk server cmd/nebulagc
E2E_TEST_DIR := tests/e2e

# Database
DEV_DB := tmp/dev.db

.PHONY: help build build-server build-daemon test test-coverage test-unit test-integration test-e2e test-all \
	lint lint-fix format check vet \
	generate migrate migrate-status migrate-up migrate-down \
	clean clean-db clean-all \
	dev-server dev-daemon dev-replica \
	install-tools deps mod-tidy \
	docker-build docker-push \
	ci pre-commit \
	all

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "NebulaGC Development Makefile"
	@echo ""
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make build          # Build all binaries"
	@echo "  make test           # Run all tests"
	@echo "  make test-e2e       # Run E2E tests"
	@echo "  make test-all       # Run unit + E2E tests"
	@echo "  make dev-server     # Start dev server"
	@echo "  make ci             # Run CI checks"

all: clean build test ## Clean, build, and test everything

install-tools: ## Install required development tools
	@echo "Installing development tools..."
	@which sqlc >/dev/null || $(GOGET) github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@which goose >/dev/null || $(GOGET) github.com/pressly/goose/v3/cmd/goose@latest
	@which golangci-lint >/dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.55.2
	@echo "✓ Development tools installed"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "✓ Dependencies downloaded"

mod-tidy: ## Tidy go.mod files
	@echo "Tidying go.mod files..."
	$(GOMOD) tidy
	cd server && $(GOMOD) tidy
	cd models && $(GOMOD) tidy
	cd pkg && $(GOMOD) tidy
	cd sdk && $(GOMOD) tidy
	cd cmd/nebulagc && $(GOMOD) tidy
	@echo "✓ Modules tidied"

build: build-server build-daemon ## Build all binaries

build-server: ## Build server binary
	@echo "Building server..."
	@mkdir -p bin
	cd server && $(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o ../$(SERVER_BINARY) ./cmd/nebulagc-server
	@echo "✓ Server built: $(SERVER_BINARY)"

build-daemon: ## Build daemon binary
	@echo "Building daemon..."
	@mkdir -p bin
	cd cmd/nebulagc && $(GOBUILD) $(BUILD_FLAGS) $(LDFLAGS) -o ../../$(DAEMON_BINARY) .
	@echo "✓ Daemon built: $(DAEMON_BINARY)"

test: ## Run all tests
	@echo "Running tests..."
	@for dir in $(MODULES); do \
		echo "Testing $$dir..."; \
		(cd $$dir && $(GOTEST) $(TEST_FLAGS) ./...) || exit 1; \
	done
	@echo "✓ Tests passed"

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	@for dir in $(MODULES); do \
		echo "Testing $$dir..."; \
		(cd $$dir && $(GOTEST) $(TEST_FLAGS) -short ./...) || exit 1; \
	done
	@echo "✓ Unit tests passed"

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	@for dir in $(MODULES); do \
		echo "Testing $$dir..."; \
		(cd $$dir && $(GOTEST) $(TEST_FLAGS) -run Integration ./...) || exit 1; \
	done
	@echo "✓ Integration tests passed"

test-coverage: ## Generate coverage report
	@echo "Generating coverage report..."
	@echo "mode: atomic" > $(COVERAGE_FILE)
	@for dir in $(MODULES); do \
		echo "Coverage for $$dir..."; \
		(cd $$dir && $(GOTEST) -race -coverprofile=coverage.tmp -covermode=atomic ./... && \
		 tail -n +2 coverage.tmp >> ../$(COVERAGE_FILE) && rm coverage.tmp) || true; \
	done
	@$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✓ Coverage report: $(COVERAGE_HTML)"
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print "  Total coverage: " $$3}'

test-bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@for dir in $(MODULES); do \
		echo "Benchmarking $$dir..."; \
		(cd $$dir && $(GOTEST) -bench=. -benchmem ./...) || true; \
	done
	@echo "✓ Benchmarks complete"

test-e2e: ## Run end-to-end tests
	@echo "Running E2E tests..."
	cd $(E2E_TEST_DIR) && $(GOTEST) -v -count=1 ./scenarios/...
	@echo "✓ E2E tests passed"

test-e2e-verbose: ## Run E2E tests with verbose output
	@echo "Running E2E tests (verbose)..."
	cd $(E2E_TEST_DIR) && $(GOTEST) -v -count=1 -timeout=10m ./scenarios/...
	@echo "✓ E2E tests passed"

test-e2e-coverage: ## Run E2E tests with coverage
	@echo "Running E2E tests with coverage..."
	cd $(E2E_TEST_DIR) && $(GOTEST) -v -count=1 -coverprofile=coverage.txt -covermode=atomic ./scenarios/...
	cd $(E2E_TEST_DIR) && $(GOCMD) tool cover -html=coverage.txt -o coverage.html
	@echo "✓ E2E coverage report: $(E2E_TEST_DIR)/coverage.html"

test-all: test test-e2e ## Run all tests (unit, integration, and E2E)
	@echo "✓ All tests passed"

lint: ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		for dir in $(MODULES); do \
			echo "Linting $$dir..."; \
			(cd $$dir && golangci-lint run --timeout=5m ./...) || exit 1; \
		done; \
		echo "✓ Linting passed"; \
	else \
		echo "✗ golangci-lint not installed"; \
		echo "  Install with: make install-tools"; \
		exit 1; \
	fi

lint-fix: ## Run linters with auto-fix
	@echo "Running linters with auto-fix..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		for dir in $(MODULES); do \
			echo "Linting $$dir..."; \
			(cd $$dir && golangci-lint run --fix --timeout=5m ./...) || exit 1; \
		done; \
		echo "✓ Linting and fixes applied"; \
	else \
		echo "✗ golangci-lint not installed"; \
		exit 1; \
	fi

format: ## Format all Go code
	@echo "Formatting code..."
	$(GOFMT) -w -s .
	@echo "✓ Code formatted"

check: vet lint ## Run checks (vet + lint)

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "✓ Vet passed"

generate: ## Run code generators (sqlc)
	@echo "Running code generators..."
	@if [ -f server/sqlc.yaml ]; then \
		cd server && sqlc generate; \
		echo "✓ SQLc generation complete"; \
	else \
		echo "✗ SQLc config not found"; \
		exit 1; \
	fi

migrate: migrate-up ## Apply all pending database migrations

migrate-up: ## Apply all pending migrations
	@echo "Applying database migrations..."
	@mkdir -p tmp
	@if command -v goose >/dev/null 2>&1; then \
		goose -dir server/migrations sqlite3 $(DEV_DB) up; \
		echo "✓ Migrations applied"; \
	else \
		echo "✗ goose not installed"; \
		echo "  Install with: make install-tools"; \
		exit 1; \
	fi

migrate-down: ## Rollback last migration
	@echo "Rolling back last migration..."
	@if command -v goose >/dev/null 2>&1; then \
		goose -dir server/migrations sqlite3 $(DEV_DB) down; \
		echo "✓ Migration rolled back"; \
	else \
		echo "✗ goose not installed"; \
		exit 1; \
	fi

migrate-status: ## Show migration status
	@echo "Migration status:"
	@if command -v goose >/dev/null 2>&1; then \
		goose -dir server/migrations sqlite3 $(DEV_DB) status; \
	else \
		echo "✗ goose not installed"; \
		exit 1; \
	fi

migrate-create: ## Create new migration (usage: make migrate-create NAME=migration_name)
	@if [ -z "$(NAME)" ]; then \
		echo "✗ NAME is required"; \
		echo "  Usage: make migrate-create NAME=create_users_table"; \
		exit 1; \
	fi
	@echo "Creating migration: $(NAME)"
	@if command -v goose >/dev/null 2>&1; then \
		goose -dir server/migrations create $(NAME) sql; \
		echo "✓ Migration created"; \
	else \
		echo "✗ goose not installed"; \
		exit 1; \
	fi

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@echo "✓ Build artifacts cleaned"

clean-db: ## Clean development database
	@echo "Cleaning development database..."
	rm -rf tmp/
	rm -f *.db *.db-shm *.db-wal
	@echo "✓ Database cleaned"

clean-all: clean clean-db ## Clean everything (build artifacts + database)
	@echo "✓ Everything cleaned"

dev-server: build-server migrate ## Start development server (master mode)
	@echo "Starting development server..."
	@echo "Server will run at http://localhost:8080"
	@echo "Press Ctrl+C to stop"
	@echo ""
	NEBULAGC_DB_PATH="$(DEV_DB)" \
	NEBULAGC_LISTEN_ADDR="0.0.0.0:8080" \
	NEBULAGC_HMAC_SECRET="dev-secret-key-minimum-32-characters-long-for-development-only" \
	NEBULAGC_LOG_LEVEL=debug \
	NEBULAGC_LOG_FORMAT=console \
	NEBULAGC_HA_MODE=master \
	./$(SERVER_BINARY)

dev-replica: build-server migrate ## Start development replica server
	@echo "Starting development replica server..."
	@echo "Replica will run at http://localhost:8081"
	@echo "Press Ctrl+C to stop"
	@echo ""
	NEBULAGC_DB_PATH="$(DEV_DB)" \
	NEBULAGC_LISTEN_ADDR="0.0.0.0:8081" \
	NEBULAGC_HMAC_SECRET="dev-secret-key-minimum-32-characters-long-for-development-only" \
	NEBULAGC_LOG_LEVEL=debug \
	NEBULAGC_LOG_FORMAT=console \
	NEBULAGC_HA_MODE=replica \
	NEBULAGC_MASTER_URL="http://localhost:8080" \
	./$(SERVER_BINARY)

dev-daemon: build-daemon ## Start development daemon
	@echo "Starting development daemon..."
	@if [ ! -f dev_config.yaml ]; then \
		echo "✗ dev_config.yaml not found"; \
		echo "  Create it with: make dev-config"; \
		exit 1; \
	fi
	@echo "Press Ctrl+C to stop"
	@echo ""
	./$(DAEMON_BINARY) daemon --config dev_config.yaml

dev-config: ## Create development config file
	@echo "Creating development config file..."
	@echo "control_plane_urls:" > dev_config.yaml
	@echo "  - \"http://localhost:8080\"" >> dev_config.yaml
	@echo "" >> dev_config.yaml
	@echo "clusters:" >> dev_config.yaml
	@echo "  - cluster_id: \"dev-cluster\"" >> dev_config.yaml
	@echo "    node_id: \"dev-node\"" >> dev_config.yaml
	@echo "    node_token: \"dev-token-change-me\"" >> dev_config.yaml
	@echo "    config_dir: \"/tmp/nebula/dev-cluster\"" >> dev_config.yaml
	@echo "    provide_lighthouse: false" >> dev_config.yaml
	@echo "" >> dev_config.yaml
	@echo "poll_interval: 5s" >> dev_config.yaml
	@echo "nebula_binary: \"nebula\"" >> dev_config.yaml
	@echo "state_dir: \"/tmp/nebulagc-daemon\"" >> dev_config.yaml
	@echo "✓ Created dev_config.yaml"

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -f server/Dockerfile -t nebulagc-server:$(VERSION) -t nebulagc-server:latest .
	docker build -f cmd/nebulagc/Dockerfile -t nebulagc-daemon:$(VERSION) -t nebulagc-daemon:latest .
	@echo "✓ Docker images built"

docker-push: ## Push Docker images
	@echo "Pushing Docker images..."
	docker push nebulagc-server:$(VERSION)
	docker push nebulagc-server:latest
	docker push nebulagc-daemon:$(VERSION)
	docker push nebulagc-daemon:latest
	@echo "✓ Docker images pushed"

ci: format vet lint test test-e2e ## Run CI checks (format, vet, lint, unit tests, E2E tests)
	@echo "✓ CI checks complete"

pre-commit: format vet lint test ## Run pre-commit checks
	@echo "✓ Pre-commit checks passed"
