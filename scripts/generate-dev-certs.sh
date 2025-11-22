#!/usr/bin/env bash
#
# generate-dev-certs.sh - Generate TLS certificates for development
#
# This script generates a self-signed CA and TLS certificates for:
# - Server certificate (localhost, 127.0.0.1)
# - Client certificate (for mTLS testing)
#
# Generated files:
#   tmp/certs/ca-key.pem       - CA private key
#   tmp/certs/ca-cert.pem      - CA certificate
#   tmp/certs/server-key.pem   - Server private key
#   tmp/certs/server-cert.pem  - Server certificate
#   tmp/certs/client-key.pem   - Client private key
#   tmp/certs/client-cert.pem  - Client certificate
#
# Usage:
#   ./scripts/generate-dev-certs.sh [--force]
#
# Options:
#   --force    Overwrite existing certificates

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CERT_DIR="${PROJECT_ROOT}/tmp/certs"

# Certificate settings
DAYS_VALID=365
CA_SUBJECT="/C=US/ST=Development/L=Local/O=NebulaGC/CN=NebulaGC Development CA"
SERVER_SUBJECT="/C=US/ST=Development/L=Local/O=NebulaGC/CN=localhost"
CLIENT_SUBJECT="/C=US/ST=Development/L=Local/O=NebulaGC/CN=nebulagc-client"

# Options
FORCE=false

log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --force)
            FORCE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--force]"
            exit 1
            ;;
    esac
done

# Create certificate directory
mkdir -p "$CERT_DIR"

# Check if certificates already exist
if [ "$FORCE" = false ] && [ -f "${CERT_DIR}/server-cert.pem" ]; then
    log_warning "Certificates already exist in ${CERT_DIR}"
    log_warning "Use --force to overwrite"
    exit 0
fi

log_info "Generating TLS certificates for development..."
echo

# Step 1: Generate CA private key
log_info "Generating CA private key..."
openssl genrsa -out "${CERT_DIR}/ca-key.pem" 4096
log_success "Created ${CERT_DIR}/ca-key.pem"

# Step 2: Generate CA certificate
log_info "Generating CA certificate..."
openssl req -new -x509 -days ${DAYS_VALID} \
    -key "${CERT_DIR}/ca-key.pem" \
    -out "${CERT_DIR}/ca-cert.pem" \
    -subj "${CA_SUBJECT}"
log_success "Created ${CERT_DIR}/ca-cert.pem"

# Step 3: Generate server private key
log_info "Generating server private key..."
openssl genrsa -out "${CERT_DIR}/server-key.pem" 2048
log_success "Created ${CERT_DIR}/server-key.pem"

# Step 4: Generate server CSR
log_info "Generating server certificate signing request..."
openssl req -new \
    -key "${CERT_DIR}/server-key.pem" \
    -out "${CERT_DIR}/server.csr" \
    -subj "${SERVER_SUBJECT}"

# Step 5: Create server certificate extensions file
cat > "${CERT_DIR}/server-ext.cnf" <<EOF
subjectAltName = @alt_names
extendedKeyUsage = serverAuth

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# Step 6: Sign server certificate with CA
log_info "Signing server certificate..."
openssl x509 -req -days ${DAYS_VALID} \
    -in "${CERT_DIR}/server.csr" \
    -CA "${CERT_DIR}/ca-cert.pem" \
    -CAkey "${CERT_DIR}/ca-key.pem" \
    -CAcreateserial \
    -out "${CERT_DIR}/server-cert.pem" \
    -extfile "${CERT_DIR}/server-ext.cnf"
log_success "Created ${CERT_DIR}/server-cert.pem"

# Step 7: Generate client private key
log_info "Generating client private key..."
openssl genrsa -out "${CERT_DIR}/client-key.pem" 2048
log_success "Created ${CERT_DIR}/client-key.pem"

# Step 8: Generate client CSR
log_info "Generating client certificate signing request..."
openssl req -new \
    -key "${CERT_DIR}/client-key.pem" \
    -out "${CERT_DIR}/client.csr" \
    -subj "${CLIENT_SUBJECT}"

# Step 9: Create client certificate extensions file
cat > "${CERT_DIR}/client-ext.cnf" <<EOF
extendedKeyUsage = clientAuth
EOF

# Step 10: Sign client certificate with CA
log_info "Signing client certificate..."
openssl x509 -req -days ${DAYS_VALID} \
    -in "${CERT_DIR}/client.csr" \
    -CA "${CERT_DIR}/ca-cert.pem" \
    -CAkey "${CERT_DIR}/ca-key.pem" \
    -CAcreateserial \
    -out "${CERT_DIR}/client-cert.pem" \
    -extfile "${CERT_DIR}/client-ext.cnf"
log_success "Created ${CERT_DIR}/client-cert.pem"

# Cleanup temporary files
rm -f "${CERT_DIR}/server.csr" \
      "${CERT_DIR}/server-ext.cnf" \
      "${CERT_DIR}/client.csr" \
      "${CERT_DIR}/client-ext.cnf" \
      "${CERT_DIR}/ca-cert.srl"

# Set appropriate permissions
chmod 600 "${CERT_DIR}"/*-key.pem
chmod 644 "${CERT_DIR}"/*-cert.pem

echo
log_success "TLS certificates generated successfully!"
echo
echo "Certificate details:"
echo "  CA Certificate:     ${CERT_DIR}/ca-cert.pem"
echo "  CA Private Key:     ${CERT_DIR}/ca-key.pem"
echo
echo "  Server Certificate: ${CERT_DIR}/server-cert.pem"
echo "  Server Private Key: ${CERT_DIR}/server-key.pem"
echo
echo "  Client Certificate: ${CERT_DIR}/client-cert.pem"
echo "  Client Private Key: ${CERT_DIR}/client-key.pem"
echo
echo "Valid for ${DAYS_VALID} days"
echo
echo "To verify certificates:"
echo "  openssl x509 -in ${CERT_DIR}/server-cert.pem -text -noout"
echo "  openssl verify -CAfile ${CERT_DIR}/ca-cert.pem ${CERT_DIR}/server-cert.pem"
echo
