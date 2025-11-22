#!/usr/bin/env bash
#
# seed-dev-data.sh - Seed development database with test data
#
# This script populates the development database with sample data:
# - Test tenant (test-tenant)
# - Sample clusters (dev-cluster, staging-cluster)
# - Lighthouse nodes
# - Configuration bundles
# - HA replica entries
#
# Usage:
#   ./scripts/seed-dev-data.sh [--db PATH] [--api URL] [--token TOKEN]
#
# Options:
#   --db PATH       Path to SQLite database (default: tmp/nebulagc.db)
#   --api URL       API endpoint for seeding via REST (alternative to direct DB)
#   --token TOKEN   API token for authentication (required with --api)
#   --clean         Remove existing data before seeding

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# Default settings
DB_PATH="tmp/nebulagc.db"
API_URL=""
API_TOKEN=""
CLEAN=false

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

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

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --db)
            DB_PATH="$2"
            shift 2
            ;;
        --api)
            API_URL="$2"
            shift 2
            ;;
        --token)
            API_TOKEN="$2"
            shift 2
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--db PATH] [--api URL] [--token TOKEN] [--clean]"
            exit 1
            ;;
    esac
done

seed_via_database() {
    local db_path="$1"
    
    if [ ! -f "$db_path" ]; then
        log_error "Database not found: $db_path"
        log_error "Run 'make migrate-up' or './scripts/dev-setup.sh' first"
        exit 1
    fi
    
    log_info "Seeding database: $db_path"
    
    # Check if sqlite3 is available
    if ! command -v sqlite3 &> /dev/null; then
        log_error "sqlite3 command not found"
        exit 1
    fi
    
    # Clean existing data if requested
    if [ "$CLEAN" = true ]; then
        log_info "Cleaning existing data..."
        sqlite3 "$db_path" <<EOF
DELETE FROM nodes;
DELETE FROM config_bundles;
DELETE FROM replicas;
DELETE FROM cluster_state;
DELETE FROM clusters;
DELETE FROM tenants;
EOF
        log_success "Existing data removed"
    fi
    
    log_info "Creating test tenant..."
    sqlite3 "$db_path" <<EOF
INSERT OR IGNORE INTO tenants (id, name, created_at, updated_at)
VALUES (
    'test-tenant',
    'Test Tenant',
    datetime('now'),
    datetime('now')
);
EOF
    log_success "Created tenant: test-tenant"
    
    log_info "Creating development cluster..."
    sqlite3 "$db_path" <<EOF
INSERT OR IGNORE INTO clusters (id, tenant_id, name, created_at, updated_at)
VALUES (
    'dev-cluster',
    'test-tenant',
    'Development Cluster',
    datetime('now'),
    datetime('now')
);

INSERT OR IGNORE INTO cluster_state (cluster_id, version, lighthouse_config, node_count, last_updated)
VALUES (
    'dev-cluster',
    1,
    '{"pki": {"ca": "-----BEGIN CERTIFICATE-----\nMIID..."}}',
    0,
    datetime('now')
);
EOF
    log_success "Created cluster: dev-cluster"
    
    log_info "Creating staging cluster..."
    sqlite3 "$db_path" <<EOF
INSERT OR IGNORE INTO clusters (id, tenant_id, name, created_at, updated_at)
VALUES (
    'staging-cluster',
    'test-tenant',
    'Staging Cluster',
    datetime('now'),
    datetime('now')
);

INSERT OR IGNORE INTO cluster_state (cluster_id, version, lighthouse_config, node_count, last_updated)
VALUES (
    'staging-cluster',
    1,
    '{"pki": {"ca": "-----BEGIN CERTIFICATE-----\nMIID..."}}',
    0,
    datetime('now')
);
EOF
    log_success "Created cluster: staging-cluster"
    
    log_info "Creating lighthouse nodes..."
    sqlite3 "$db_path" <<EOF
INSERT OR IGNORE INTO nodes (id, cluster_id, name, ip_address, is_lighthouse, status, created_at, updated_at)
VALUES
    ('node-lighthouse-1', 'dev-cluster', 'lighthouse-1', '10.99.0.1', 1, 'active', datetime('now'), datetime('now')),
    ('node-lighthouse-2', 'dev-cluster', 'lighthouse-2', '10.99.0.2', 1, 'active', datetime('now'), datetime('now')),
    ('node-edge-1', 'dev-cluster', 'edge-1', '10.99.1.1', 0, 'active', datetime('now'), datetime('now')),
    ('node-edge-2', 'dev-cluster', 'edge-2', '10.99.1.2', 0, 'active', datetime('now'), datetime('now')),
    ('node-lighthouse-staging', 'staging-cluster', 'lighthouse-staging', '10.100.0.1', 1, 'active', datetime('now'), datetime('now'));

UPDATE cluster_state SET node_count = 4 WHERE cluster_id = 'dev-cluster';
UPDATE cluster_state SET node_count = 1 WHERE cluster_id = 'staging-cluster';
EOF
    log_success "Created 5 nodes (3 lighthouses, 2 edge)"
    
    log_info "Creating configuration bundles..."
    sqlite3 "$db_path" <<EOF
INSERT OR IGNORE INTO config_bundles (id, cluster_id, version, config_data, created_at, activated_at)
VALUES
    (
        'bundle-dev-v1',
        'dev-cluster',
        1,
        '{"pki":{"ca":"-----BEGIN CERTIFICATE-----\nMIID..."},"static_host_map":{"10.99.0.1":["192.168.1.10:4242"],"10.99.0.2":["192.168.1.11:4242"]},"lighthouse":{"am_lighthouse":false,"interval":60,"hosts":["10.99.0.1","10.99.0.2"]},"relay":{"am_relay":false,"use_relays":true,"relays":["10.99.0.1"]},"listen":{"host":"0.0.0.0","port":4242},"punchy":{"punch":true,"respond":true},"tun":{"disabled":false,"dev":"nebula1","drop_local_broadcast":false,"drop_multicast":false,"tx_queue":500,"mtu":1300}}',
        datetime('now'),
        datetime('now')
    ),
    (
        'bundle-dev-v2',
        'dev-cluster',
        2,
        '{"pki":{"ca":"-----BEGIN CERTIFICATE-----\nMIID..."},"static_host_map":{"10.99.0.1":["192.168.1.10:4242"],"10.99.0.2":["192.168.1.11:4242"]},"lighthouse":{"am_lighthouse":false,"interval":60,"hosts":["10.99.0.1","10.99.0.2"]},"relay":{"am_relay":false,"use_relays":true,"relays":["10.99.0.1"]},"listen":{"host":"0.0.0.0","port":4242},"punchy":{"punch":true,"respond":true,"delay":"1s"},"tun":{"disabled":false,"dev":"nebula1","drop_local_broadcast":false,"drop_multicast":false,"tx_queue":500,"mtu":1300,"routes":[{"mtu":1300,"route":"10.99.0.0/16"}]}}',
        datetime('now'),
        NULL
    ),
    (
        'bundle-staging-v1',
        'staging-cluster',
        1,
        '{"pki":{"ca":"-----BEGIN CERTIFICATE-----\nMIIE..."},"static_host_map":{"10.100.0.1":["192.168.2.10:4242"]},"lighthouse":{"am_lighthouse":false,"interval":60,"hosts":["10.100.0.1"]},"listen":{"host":"0.0.0.0","port":4242},"punchy":{"punch":true,"respond":true},"tun":{"disabled":false,"dev":"nebula1","mtu":1300}}',
        datetime('now'),
        datetime('now')
    );
EOF
    log_success "Created 3 configuration bundles"
    
    log_info "Creating HA replica entries..."
    sqlite3 "$db_path" <<EOF
INSERT OR IGNORE INTO replicas (id, node_id, api_url, status, last_heartbeat, created_at, updated_at)
VALUES
    ('replica-master', 'dev-master', 'https://localhost:8080', 'active', datetime('now'), datetime('now'), datetime('now')),
    ('replica-backup-1', 'dev-backup-1', 'https://localhost:8081', 'active', datetime('now', '-5 seconds'), datetime('now'), datetime('now')),
    ('replica-backup-2', 'dev-backup-2', 'https://localhost:8082', 'inactive', datetime('now', '-2 minutes'), datetime('now'), datetime('now'));
EOF
    log_success "Created 3 HA replica entries"
    
    log_info "Verifying data..."
    local tenant_count cluster_count node_count bundle_count replica_count
    tenant_count=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM tenants;")
    cluster_count=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM clusters;")
    node_count=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM nodes;")
    bundle_count=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM config_bundles;")
    replica_count=$(sqlite3 "$db_path" "SELECT COUNT(*) FROM replicas;")
    
    echo
    log_success "Database seeded successfully!"
    echo
    echo "Summary:"
    echo "  Tenants:           $tenant_count"
    echo "  Clusters:          $cluster_count"
    echo "  Nodes:             $node_count"
    echo "  Config Bundles:    $bundle_count"
    echo "  HA Replicas:       $replica_count"
    echo
    echo "Sample data:"
    echo "  Tenant ID:         test-tenant"
    echo "  Cluster IDs:       dev-cluster, staging-cluster"
    echo "  Lighthouse Nodes:  10.99.0.1, 10.99.0.2, 10.100.0.1"
    echo "  Edge Nodes:        10.99.1.1, 10.99.1.2"
    echo "  Active Bundle:     bundle-dev-v1 (dev-cluster)"
    echo
}

seed_via_api() {
    local api_url="$1"
    local token="$2"
    
    if [ -z "$token" ]; then
        log_error "--token is required when using --api"
        exit 1
    fi
    
    log_info "Seeding via API: $api_url"
    
    # Check if curl is available
    if ! command -v curl &> /dev/null; then
        log_error "curl command not found"
        exit 1
    fi
    
    # Create tenant
    log_info "Creating tenant..."
    local response
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d '{"id":"test-tenant","name":"Test Tenant"}' \
        "${api_url}/api/v1/tenants")
    
    local http_code
    http_code=$(echo "$response" | tail -n1)
    if [ "$http_code" = "200" ] || [ "$http_code" = "201" ]; then
        log_success "Created tenant: test-tenant"
    elif [ "$http_code" = "409" ]; then
        log_warning "Tenant already exists"
    else
        log_error "Failed to create tenant (HTTP $http_code)"
        exit 1
    fi
    
    # Additional API calls would go here...
    log_warning "Full API seeding not yet implemented"
    log_info "Use direct database seeding: $0 --db $DB_PATH"
}

# Main execution
main() {
    cd "$PROJECT_ROOT"
    
    if [ -n "$API_URL" ]; then
        seed_via_api "$API_URL" "$API_TOKEN"
    else
        seed_via_database "$DB_PATH"
    fi
}

main
