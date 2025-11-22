# Docker Deployment Guide

This guide covers deploying NebulaGC Server and Daemon using Docker and Docker Compose.

---

## Overview

Docker provides portable, reproducible deployments with isolated environments. This guide includes:

- Multi-stage Dockerfiles for minimal images
- Docker Compose for local development and testing
- Production deployment considerations
- Volume management for persistence
- Health checks and monitoring

---

## Prerequisites

- Docker 20.10+ installed
- Docker Compose 2.0+ (or docker-compose 1.29+)
- Basic understanding of Docker concepts

---

## Server Dockerfile

Create `server/Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make sqlite

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.work ./
COPY server/go.mod server/
COPY models/go.mod models/
COPY pkg/go.mod pkg/
COPY sdk/go.mod sdk/

# Download dependencies
RUN go work sync
RUN go mod download

# Copy source code
COPY models/ models/
COPY pkg/ pkg/
COPY sdk/ sdk/
COPY server/ server/

# Build server binary
WORKDIR /build/server
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /build/nebulagc-server ./cmd/nebulagc-server

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Create non-root user
RUN addgroup -g 1000 nebulagc && \
    adduser -D -u 1000 -G nebulagc nebulagc

# Create directories
RUN mkdir -p /var/lib/nebulagc-server /etc/nebulagc && \
    chown -R nebulagc:nebulagc /var/lib/nebulagc-server /etc/nebulagc

# Copy binary from builder
COPY --from=builder /build/nebulagc-server /usr/local/bin/nebulagc-server
RUN chmod +x /usr/local/bin/nebulagc-server

# Switch to non-root user
USER nebulagc

# Set working directory
WORKDIR /var/lib/nebulagc-server

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default environment variables
ENV NEBULAGC_DB_PATH=/var/lib/nebulagc-server/nebulagc.db \
    NEBULAGC_LISTEN_ADDR=0.0.0.0:8080 \
    NEBULAGC_LOG_LEVEL=info \
    NEBULAGC_LOG_FORMAT=json

# Run server
ENTRYPOINT ["/usr/local/bin/nebulagc-server"]
```

### Build Server Image

```bash
cd /path/to/NebulaGC
docker build -f server/Dockerfile -t nebulagc-server:latest .
```

### Run Server Container

```bash
docker run -d \
  --name nebulagc-server \
  -p 8080:8080 \
  -v nebulagc-data:/var/lib/nebulagc-server \
  -e NEBULAGC_HMAC_SECRET=$(openssl rand -hex 32) \
  -e NEBULAGC_HA_MODE=master \
  nebulagc-server:latest
```

---

## Daemon Dockerfile

Create `cmd/nebulagc/Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.work ./
COPY cmd/nebulagc/go.mod cmd/nebulagc/
COPY models/go.mod models/
COPY sdk/go.mod sdk/

# Download dependencies
RUN go work sync
RUN go mod download

# Copy source code
COPY models/ models/
COPY sdk/ sdk/
COPY cmd/nebulagc/ cmd/nebulagc/

# Build daemon binary
WORKDIR /build/cmd/nebulagc
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/nebulagc .

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget

# Install Nebula
RUN wget -q https://github.com/slackhq/nebula/releases/download/v1.8.2/nebula-linux-amd64.tar.gz && \
    tar -xzf nebula-linux-amd64.tar.gz && \
    mv nebula /usr/local/bin/ && \
    chmod +x /usr/local/bin/nebula && \
    rm nebula-linux-amd64.tar.gz nebula-cert

# Create non-root user
RUN addgroup -g 1000 nebulagc && \
    adduser -D -u 1000 -G nebulagc nebulagc

# Create directories
RUN mkdir -p /etc/nebulagc/daemon /var/lib/nebulagc-daemon /etc/nebula && \
    chown -R nebulagc:nebulagc /etc/nebulagc /var/lib/nebulagc-daemon /etc/nebula

# Copy binary from builder
COPY --from=builder /build/nebulagc /usr/local/bin/nebulagc
RUN chmod +x /usr/local/bin/nebulagc

# Switch to non-root user
USER nebulagc

# Set working directory
WORKDIR /var/lib/nebulagc-daemon

# Default environment
ENV NEBULA_BINARY=/usr/local/bin/nebula \
    CONFIG_FILE=/etc/nebulagc/daemon/config.yaml

# Run daemon
ENTRYPOINT ["/usr/local/bin/nebulagc"]
CMD ["daemon", "--config", "/etc/nebulagc/daemon/config.yaml"]
```

### Build Daemon Image

```bash
cd /path/to/NebulaGC
docker build -f cmd/nebulagc/Dockerfile -t nebulagc-daemon:latest .
```

### Run Daemon Container

```bash
docker run -d \
  --name nebulagc-daemon \
  --cap-add=NET_ADMIN \
  --device=/dev/net/tun \
  -v nebulagc-daemon-config:/etc/nebulagc/daemon \
  -v nebulagc-daemon-state:/var/lib/nebulagc-daemon \
  nebulagc-daemon:latest
```

---

## Docker Compose

Create `docker-compose.yml` for local development:

```yaml
version: '3.8'

services:
  # Control Plane Server
  server:
    build:
      context: .
      dockerfile: server/Dockerfile
    container_name: nebulagc-server
    ports:
      - "8080:8080"
    volumes:
      - server-data:/var/lib/nebulagc-server
    environment:
      NEBULAGC_DB_PATH: /var/lib/nebulagc-server/nebulagc.db
      NEBULAGC_LISTEN_ADDR: 0.0.0.0:8080
      NEBULAGC_HMAC_SECRET: dev-secret-change-in-production
      NEBULAGC_HA_MODE: master
      NEBULAGC_LOG_LEVEL: debug
      NEBULAGC_LOG_FORMAT: console
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 5s
    restart: unless-stopped
    networks:
      - nebulagc

  # Replica Server 1
  server-replica1:
    build:
      context: .
      dockerfile: server/Dockerfile
    container_name: nebulagc-server-replica1
    ports:
      - "8081:8080"
    volumes:
      - server-replica1-data:/var/lib/nebulagc-server
    environment:
      NEBULAGC_DB_PATH: /var/lib/nebulagc-server/nebulagc.db
      NEBULAGC_LISTEN_ADDR: 0.0.0.0:8080
      NEBULAGC_HMAC_SECRET: dev-secret-change-in-production
      NEBULAGC_HA_MODE: replica
      NEBULAGC_MASTER_URL: http://server:8080
      NEBULAGC_LOG_LEVEL: info
      NEBULAGC_LOG_FORMAT: json
    depends_on:
      - server
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 5s
    restart: unless-stopped
    networks:
      - nebulagc

  # Replica Server 2
  server-replica2:
    build:
      context: .
      dockerfile: server/Dockerfile
    container_name: nebulagc-server-replica2
    ports:
      - "8082:8080"
    volumes:
      - server-replica2-data:/var/lib/nebulagc-server
    environment:
      NEBULAGC_DB_PATH: /var/lib/nebulagc-server/nebulagc.db
      NEBULAGC_LISTEN_ADDR: 0.0.0.0:8080
      NEBULAGC_HMAC_SECRET: dev-secret-change-in-production
      NEBULAGC_HA_MODE: replica
      NEBULAGC_MASTER_URL: http://server:8080
      NEBULAGC_LOG_LEVEL: info
      NEBULAGC_LOG_FORMAT: json
    depends_on:
      - server
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 5s
    restart: unless-stopped
    networks:
      - nebulagc

  # Daemon (requires manual configuration)
  daemon:
    build:
      context: .
      dockerfile: cmd/nebulagc/Dockerfile
    container_name: nebulagc-daemon
    volumes:
      - daemon-config:/etc/nebulagc/daemon
      - daemon-state:/var/lib/nebulagc-daemon
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun
    depends_on:
      - server
    restart: unless-stopped
    networks:
      - nebulagc
    # Note: You must create config.yaml before starting daemon
    # See daemon configuration section below

volumes:
  server-data:
  server-replica1-data:
  server-replica2-data:
  daemon-config:
  daemon-state:

networks:
  nebulagc:
    driver: bridge
```

### Start Services

```bash
# Start all services
docker-compose up -d

# Start only server
docker-compose up -d server

# View logs
docker-compose logs -f server

# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

---

## Production Configuration

### Environment Variables

For production, use environment variables or secrets management:

```bash
# Generate secure HMAC secret
export NEBULAGC_HMAC_SECRET=$(openssl rand -hex 32)

# Pass to container
docker run -d \
  --name nebulagc-server \
  -p 8080:8080 \
  -v nebulagc-data:/var/lib/nebulagc-server \
  -e NEBULAGC_HMAC_SECRET="${NEBULAGC_HMAC_SECRET}" \
  -e NEBULAGC_HA_MODE=master \
  -e NEBULAGC_LOG_LEVEL=info \
  -e NEBULAGC_LOG_FORMAT=json \
  nebulagc-server:latest
```

### Docker Secrets (Swarm Mode)

Create secret:

```bash
echo "your-secret-here" | docker secret create nebulagc_hmac_secret -
```

Use in service:

```yaml
version: '3.8'

services:
  server:
    image: nebulagc-server:latest
    secrets:
      - nebulagc_hmac_secret
    environment:
      NEBULAGC_HMAC_SECRET_FILE: /run/secrets/nebulagc_hmac_secret

secrets:
  nebulagc_hmac_secret:
    external: true
```

### Volume Management

```bash
# Create named volumes
docker volume create nebulagc-data
docker volume create nebulagc-daemon-config

# Inspect volume
docker volume inspect nebulagc-data

# Backup volume
docker run --rm \
  -v nebulagc-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/nebulagc-data-backup.tar.gz /data

# Restore volume
docker run --rm \
  -v nebulagc-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/nebulagc-data-backup.tar.gz -C /
```

### Resource Limits

```yaml
services:
  server:
    image: nebulagc-server:latest
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

Or with `docker run`:

```bash
docker run -d \
  --name nebulagc-server \
  --cpus=2 \
  --memory=512m \
  -p 8080:8080 \
  nebulagc-server:latest
```

---

## Multi-Stage Build Optimization

The Dockerfiles use multi-stage builds to minimize image size:

- **Build stage**: Uses `golang:1.23-alpine` with full build tools
- **Runtime stage**: Uses `alpine:3.19` with only runtime dependencies
- **Result**: Server image ~20-30MB, Daemon image ~15-25MB

### Further Optimization

Use `scratch` base for smallest images (daemon only, no shell):

```dockerfile
# Runtime stage (daemon only - no CGO dependencies)
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/nebulagc /nebulagc
COPY --from=builder /usr/local/bin/nebula /usr/local/bin/nebula

USER 1000:1000
ENTRYPOINT ["/nebulagc"]
```

---

## Daemon Configuration

Before starting the daemon container, create configuration file:

```bash
# Create config directory
docker volume create nebulagc-daemon-config

# Create temporary container to write config
docker run --rm -v nebulagc-daemon-config:/config alpine sh -c 'cat > /config/config.yaml << EOF
control_plane_urls:
  - "http://nebulagc-server:8080"

clusters:
  - cluster_id: "cluster-1"
    node_id: "node-1"
    node_token: "your-node-token-here"
    config_dir: "/etc/nebula/cluster-1"
    provide_lighthouse: false

poll_interval: 5s
nebula_binary: "/usr/local/bin/nebula"
state_dir: "/var/lib/nebulagc-daemon"
EOF'

# Verify config
docker run --rm -v nebulagc-daemon-config:/config alpine cat /config/config.yaml
```

---

## Networking

### Bridge Network (Default)

Services communicate via container names:

```yaml
networks:
  nebulagc:
    driver: bridge
```

### Host Network

Daemon needs host network if managing host Nebula interfaces:

```bash
docker run -d \
  --name nebulagc-daemon \
  --network host \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  nebulagc-daemon:latest
```

### Macvlan Network

For production with external access:

```bash
docker network create -d macvlan \
  --subnet=192.168.1.0/24 \
  --gateway=192.168.1.1 \
  -o parent=eth0 \
  nebulagc-macvlan

docker run -d \
  --name nebulagc-server \
  --network nebulagc-macvlan \
  --ip 192.168.1.100 \
  nebulagc-server:latest
```

---

## Monitoring and Logging

### View Logs

```bash
# Follow logs
docker logs -f nebulagc-server

# Last 100 lines
docker logs --tail 100 nebulagc-server

# Logs with timestamps
docker logs -t nebulagc-server

# Logs since specific time
docker logs --since 2024-11-20T10:00:00 nebulagc-server
```

### Log Drivers

Configure log driver for centralized logging:

```yaml
services:
  server:
    image: nebulagc-server:latest
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

Or use syslog:

```yaml
logging:
  driver: "syslog"
  options:
    syslog-address: "tcp://syslog-server:514"
    tag: "nebulagc-server"
```

### Health Checks

Check container health:

```bash
# Inspect health status
docker inspect --format='{{.State.Health.Status}}' nebulagc-server

# View health check logs
docker inspect --format='{{json .State.Health}}' nebulagc-server | jq
```

### Prometheus Metrics

Expose metrics endpoint:

```bash
# Server metrics
curl http://localhost:8080/metrics

# With docker-compose
curl http://$(docker-compose port server 8080)/metrics
```

---

## Troubleshooting

### Container Won't Start

```bash
# Check container logs
docker logs nebulagc-server

# Inspect container
docker inspect nebulagc-server

# Check exit code
docker inspect --format='{{.State.ExitCode}}' nebulagc-server
```

### Permission Issues

```bash
# Check volume permissions
docker run --rm -v nebulagc-data:/data alpine ls -la /data

# Fix ownership (if needed)
docker run --rm -v nebulagc-data:/data alpine chown -R 1000:1000 /data
```

### Database Lock Errors

```bash
# Check if multiple containers using same volume
docker ps -a --filter volume=nebulagc-data

# Stop all containers using volume
docker stop $(docker ps -aq --filter volume=nebulagc-data)

# Verify database WAL mode
docker run --rm -v nebulagc-data:/data alpine \
  sqlite3 /data/nebulagc.db "PRAGMA journal_mode;"
```

### Network Connectivity

```bash
# Test connectivity between containers
docker exec nebulagc-daemon ping nebulagc-server

# Check DNS resolution
docker exec nebulagc-daemon nslookup nebulagc-server

# Inspect network
docker network inspect nebulagc
```

### Build Failures

```bash
# Build with no cache
docker build --no-cache -f server/Dockerfile -t nebulagc-server:latest .

# Build with verbose output
docker build --progress=plain -f server/Dockerfile .

# Check build args
docker build --build-arg GO_VERSION=1.23 -f server/Dockerfile .
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build Docker Images

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2
      
      - name: Login to Docker Hub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}
      
      - name: Build and push server
        uses: docker/build-push-action@v4
        with:
          context: .
          file: server/Dockerfile
          push: true
          tags: |
            nebulagc/server:latest
            nebulagc/server:${{ github.sha }}
      
      - name: Build and push daemon
        uses: docker/build-push-action@v4
        with:
          context: .
          file: cmd/nebulagc/Dockerfile
          push: true
          tags: |
            nebulagc/daemon:latest
            nebulagc/daemon:${{ github.sha }}
```

---

## Best Practices

1. **Use Multi-Stage Builds**: Minimize image size
2. **Run as Non-Root**: Never use `USER root` in production
3. **Health Checks**: Always include `HEALTHCHECK` in Dockerfile
4. **Resource Limits**: Set memory and CPU limits
5. **Volume Management**: Use named volumes for persistence
6. **Secrets Management**: Never hardcode secrets in images
7. **Image Tagging**: Use semantic versioning for tags
8. **Regular Updates**: Keep base images updated for security
9. **Log Management**: Configure log rotation to prevent disk issues
10. **Security Scanning**: Scan images with tools like Trivy

### Security Scanning

```bash
# Scan with Trivy
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy image nebulagc-server:latest

# Scan with Grype
grype nebulagc-server:latest
```

---

## Additional Resources

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Reference](https://docs.docker.com/compose/compose-file/)
- [Multi-Stage Builds](https://docs.docker.com/build/building/multi-stage/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Dockerfile Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
