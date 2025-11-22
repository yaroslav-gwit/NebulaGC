#!/usr/bin/env bash
#
# dev-setup.sh - Initialize development environment for NebulaGC
#
# This script sets up a complete development environment including:
# - Required development tools (golangci-lint, sqlc, goose)
# - Project dependencies
# - Development database
# - TLS certificates
# - Default configuration files
#
# Usage:
#   ./scripts/dev-setup.sh [--skip-tools] [--skip-db] [--skip-certs]
#
# Options:
#   --skip-tools   Skip installation of development tools
#   --skip-db      Skip database initialization
#   --skip-certs   Skip TLS certificate generation
#   --help         Show this help message

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default options
SKIP_TOOLS=false
SKIP_DB=false
SKIP_CERTS=false

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Tool versions
GOLANGCI_LINT_VERSION="v1.55.2"
SQLC_VERSION="v1.25.0"
GOOSE_VERSION="v3.18.0"

# Functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

show_help() {
    sed -n '/^# Usage:/,/^$/p' "$0" | sed 's/^# //;s/^#//'
}

check_command() {
    if command -v "$1" &> /dev/null; then
        return 0
    else
        return 1
    fi
}

install_tools() {
    log_info "Installing development tools..."
    
    # Check if Go is installed
    if ! check_command go; then
        log_error "Go is not installed. Please install Go 1.22 or later."
        exit 1
    fi
    
    local go_version
    go_version=$(go version | awk '{print $3}' | sed 's/go//')
    log_info "Found Go $go_version"
    
    # Install golangci-lint
    if check_command golangci-lint; then
        local current_version
        current_version=$(golangci-lint version --format short 2>/dev/null || echo "unknown")
        log_info "golangci-lint already installed: $current_version"
    else
        log_info "Installing golangci-lint ${GOLANGCI_LINT_VERSION}..."
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
            sh -s -- -b "$(go env GOPATH)/bin" "${GOLANGCI_LINT_VERSION}"
        log_success "Installed golangci-lint"
    fi
    
    # Install sqlc
    if check_command sqlc; then
        local current_version
        current_version=$(sqlc version 2>/dev/null || echo "unknown")
        log_info "sqlc already installed: $current_version"
    else
        log_info "Installing sqlc ${SQLC_VERSION}..."
        go install "github.com/sqlc-dev/sqlc/cmd/sqlc@${SQLC_VERSION}"
        log_success "Installed sqlc"
    fi
    
    # Install goose
    if check_command goose; then
        local current_version
        current_version=$(goose -version 2>/dev/null | head -n1 || echo "unknown")
        log_info "goose already installed: $current_version"
    else
        log_info "Installing goose ${GOOSE_VERSION}..."
        go install "github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}"
        log_success "Installed goose"
    fi
    
    log_success "Development tools installed"
}

setup_dependencies() {
    log_info "Setting up project dependencies..."
    
    cd "$PROJECT_ROOT"
    
    # Download all module dependencies
    log_info "Downloading Go modules..."
    go work sync
    go mod download
    
    log_success "Dependencies installed"
}

init_database() {
    log_info "Initializing development database..."
    
    cd "$PROJECT_ROOT"
    
    # Create tmp directory for database
    mkdir -p tmp
    
    # Check if database already exists
    if [ -f "tmp/nebulagc.db" ]; then
        log_warning "Database already exists at tmp/nebulagc.db"
        read -p "Do you want to recreate it? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -f tmp/nebulagc.db tmp/nebulagc.db-shm tmp/nebulagc.db-wal
            log_info "Removed existing database"
        else
            log_info "Keeping existing database"
            return
        fi
    fi
    
    # Run migrations
    log_info "Running database migrations..."
    cd server
    export DATABASE_URL="sqlite3://$(pwd)/../tmp/nebulagc.db"
    
    if ! goose -dir migrations sqlite3 "../tmp/nebulagc.db" up; then
        log_error "Failed to run migrations"
        exit 1
    fi
    
    cd "$PROJECT_ROOT"
    log_success "Database initialized"
}

generate_certs() {
    log_info "Generating development TLS certificates..."
    
    if [ ! -x "${SCRIPT_DIR}/generate-dev-certs.sh" ]; then
        log_error "Certificate generation script not found or not executable"
        exit 1
    fi
    
    "${SCRIPT_DIR}/generate-dev-certs.sh"
    log_success "TLS certificates generated"
}

create_dev_config() {
    log_info "Creating development configuration..."
    
    cd "$PROJECT_ROOT"
    
    local config_file="dev_config.yaml"
    if [ -f "$config_file" ]; then
        log_warning "Configuration file already exists: $config_file"
        return
    fi
    
    # Use Makefile target to generate config
    if make dev-config &> /dev/null; then
        log_success "Created development configuration: $config_file"
    else
        log_warning "Could not generate config with Make, creating manually..."
        cat > "$config_file" <<'EOF'
# Development Configuration for NebulaGC
server:
  address: "localhost:8080"
  tls:
    enabled: true
    cert_file: "tmp/certs/server-cert.pem"
    key_file: "tmp/certs/server-key.pem"
    ca_file: "tmp/certs/ca-cert.pem"
    client_auth: "VerifyClientCertIfGiven"

database:
  driver: "sqlite3"
  dsn: "./tmp/nebulagc.db"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: "5m"

lighthouse:
  binary_path: "/usr/local/bin/lighthouse"
  config_dir: "./tmp/lighthouse"
  log_level: "info"

ha:
  mode: "standalone"
  node_id: "dev-master"

log:
  level: "debug"
  format: "json"
  output: "stdout"

metrics:
  enabled: true
  address: "localhost:9090"
  path: "/metrics"

rate_limit:
  requests_per_second: 100
  burst: 200
  enabled: true
EOF
        log_success "Created development configuration: $config_file"
    fi
}

print_summary() {
    echo
    log_success "Development environment setup complete!"
    echo
    echo "Next steps:"
    echo "  1. Review configuration: dev_config.yaml"
    echo "  2. Start development server: make dev-server"
    echo "  3. Run tests: make test"
    echo "  4. Run linter: make lint"
    echo
    echo "Available Make targets:"
    echo "  make help          - Show all available targets"
    echo "  make dev-server    - Run server with development config"
    echo "  make dev-daemon    - Run daemon with development config"
    echo "  make test          - Run all tests"
    echo "  make lint          - Run linter"
    echo "  make build         - Build binaries"
    echo
    echo "Database:"
    echo "  Location: tmp/nebulagc.db"
    echo "  Seed data: ./scripts/seed-dev-data.sh"
    echo
    echo "TLS Certificates:"
    echo "  CA: tmp/certs/ca-cert.pem"
    echo "  Server: tmp/certs/server-cert.pem"
    echo "  Client: tmp/certs/client-cert.pem"
    echo
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-tools)
            SKIP_TOOLS=true
            shift
            ;;
        --skip-db)
            SKIP_DB=true
            shift
            ;;
        --skip-certs)
            SKIP_CERTS=true
            shift
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Main execution
main() {
    log_info "Setting up NebulaGC development environment..."
    echo
    
    if [ "$SKIP_TOOLS" = false ]; then
        install_tools
        echo
    fi
    
    setup_dependencies
    echo
    
    if [ "$SKIP_DB" = false ]; then
        init_database
        echo
    fi
    
    if [ "$SKIP_CERTS" = false ]; then
        generate_certs
        echo
    fi
    
    create_dev_config
    
    print_summary
}

main
