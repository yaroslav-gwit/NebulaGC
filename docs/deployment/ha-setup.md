# High Availability Setup Guide

This guide covers deploying NebulaGC in high availability configurations with automatic failover.

---

## Overview

High availability (HA) ensures NebulaGC Control Plane remains operational despite server failures. This guide covers:

- HA architecture and components
- Deployment topologies
- Failover behavior
- Monitoring and alerting
- Best practices

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│                        Load Balancer                          │
│                  (HAProxy / NGINX / AWS ALB)                  │
└────────────────┬───────────────┬─────────────────────────────┘
                 │               │
      ┌──────────▼──────┐       │       ┌─────────────────────┐
      │   Master Node   │       │       │  Replica Nodes      │
      │                 │       │       │                     │
      │  ┌───────────┐  │       └──────▶│  ┌───────────┐     │
      │  │  Server   │  │                │  │  Server   │     │
      │  │ (R/W Mode)│  │                │  │ (RO Mode) │     │
      │  └─────┬─────┘  │                │  └─────┬─────┘     │
      │        │        │                │        │           │
      │  ┌─────▼─────┐  │                │  ┌─────▼─────┐     │
      │  │  SQLite   │──┼───Replicate───▶│  │  SQLite   │     │
      │  │   (WAL)   │  │  (Litestream/  │  │   (RO)    │     │
      │  └───────────┘  │   LiteFS)      │  └───────────┘     │
      └─────────────────┘                └─────────────────────┘
                 │                                  │
                 └──────────────┬───────────────────┘
                                │
                      ┌─────────▼─────────┐
                      │  Daemon Nodes     │
                      │  (All connect to  │
                      │   all servers)    │
                      └───────────────────┘
```

---

## HA Components

### 1. Master Server

- **Role**: Handles all write operations
- **Database**: SQLite in WAL mode (read-write)
- **Replication**: Streams changes to replicas
- **Failure Impact**: Writes blocked until failover

### 2. Replica Servers

- **Role**: Handles read operations
- **Database**: SQLite (read-only)
- **Replication**: Receives changes from master
- **Failure Impact**: Reduced read capacity

### 3. Load Balancer

- **Role**: Distributes traffic to healthy servers
- **Health Checks**: Monitors `/health` endpoint
- **Sticky Sessions**: Routes writes to master

### 4. Replication Layer

- **Litestream**: Async replication to object storage
- **LiteFS**: Sync replication to replica nodes
- **Choice**: Depends on requirements

### 5. Service Discovery (Optional)

- **Consul**: Leader election, service registry
- **etcd**: Distributed configuration
- **Kubernetes**: Built-in service discovery

---

## Deployment Topologies

### Topology 1: Single Instance (No HA)

```
┌─────────────────┐
│  Single Server  │
│  (Master Mode)  │
└────────┬────────┘
         │
    ┌────▼────┐
    │ Daemons │
    └─────────┘
```

**Pros**:
- Simple setup
- No replication overhead
- Lower cost

**Cons**:
- Single point of failure
- No automatic failover
- Downtime during maintenance

**Use Case**: Development, testing, low-priority deployments

---

### Topology 2: Master + Litestream (Disaster Recovery)

```
┌─────────────────┐         ┌──────────────┐
│  Master Server  │────────▶│  S3 Bucket   │
│                 │ Stream  │  (Backups)   │
│  + Litestream   │         └──────────────┘
└────────┬────────┘
         │
    ┌────▼────┐
    │ Daemons │
    └─────────┘
```

**Pros**:
- Point-in-time recovery
- Geographic redundancy
- Low cost (storage only)

**Cons**:
- Manual failover required
- Recovery time: minutes
- No automatic failover

**Use Case**: Small deployments, cost-sensitive, infrequent writes

---

### Topology 3: Master + Replicas + Litestream (Full HA)

```
                  ┌──────────────┐
                  │ Load Balancer│
                  └──────┬───────┘
                         │
          ┌──────────────┼──────────────┐
          │              │              │
    ┌─────▼─────┐  ┌─────▼─────┐  ┌────▼──────┐
    │  Master   │  │ Replica 1 │  │ Replica 2 │
    │           │  │           │  │           │
    │+Litestream│  │+Litestream│  │+Litestream│
    └─────┬─────┘  └─────┬─────┘  └─────┬─────┘
          │              │              │
          └──────────────┼──────────────┘
                         │
                    ┌────▼────┐
                    │ Daemons │
                    └─────────┘
                         │
                    ┌────▼────┐
                    │   S3    │
                    └─────────┘
```

**Pros**:
- High availability
- Read scaling
- Disaster recovery (S3)
- Fast failover (<30s)

**Cons**:
- More complex setup
- Higher cost (compute + storage)
- Manual master promotion

**Use Case**: Production deployments, critical applications

---

### Topology 4: LiteFS Cluster (Automatic Failover)

```
         ┌──────────────┐
         │Load Balancer │
         └──────┬───────┘
                │
   ┌────────────┼────────────┐
   │            │            │
┌──▼───┐   ┌───▼──┐    ┌────▼──┐
│Node 1│   │Node 2│    │Node 3 │
│      │   │      │    │       │
│LiteFS│◀─▶│LiteFS│◀──▶│LiteFS │
└──────┘   └──────┘    └───────┘
   △                        △
   └────────────┬───────────┘
                │
           ┌────▼────┐
           │ Consul  │
           │(Leader  │
           │Election)│
           └─────────┘
                │
           ┌────▼────┐
           │ Daemons │
           └─────────┘
```

**Pros**:
- Automatic failover
- Distributed reads
- Low replication lag (<10ms)
- No manual intervention

**Cons**:
- Most complex setup
- Requires Consul
- Higher resource usage

**Use Case**: Mission-critical deployments, high traffic, strict SLAs

---

## Failover Behavior

### Master Failure Scenarios

#### Scenario 1: Master Becomes Unresponsive

**With Litestream Only**:
1. Load balancer detects failure (health check fails)
2. All traffic blocked (no writes possible)
3. Admin manually promotes replica
4. Restore database from Litestream
5. Start new master
6. Update DNS/load balancer

**Downtime**: 5-15 minutes (manual)

**With LiteFS**:
1. Consul detects missed heartbeats (10-30s)
2. New leader automatically elected
3. Replicas connect to new primary
4. Writes resume automatically
5. Load balancer routes to new master

**Downtime**: <30 seconds (automatic)

#### Scenario 2: Network Partition

**With Litestream**:
- Master isolated from replicas
- Writes continue on master
- Replicas serve stale data
- Manual reconciliation needed

**With LiteFS + Consul**:
- Consul majority required for writes
- Split-brain prevented
- Minority partition becomes read-only

---

### Replica Failure Scenarios

#### Scenario: Replica Goes Down

1. Load balancer detects failure
2. Traffic routed to remaining replicas
3. Read capacity reduced
4. No impact on writes
5. Replica automatically rejoins on recovery

**Impact**: Minimal (reduced read capacity)

---

## Load Balancer Configuration

### HAProxy

```haproxy
# /etc/haproxy/haproxy.cfg

global
    log /dev/log local0
    maxconn 4096

defaults
    log     global
    mode    http
    option  httplog
    option  dontlognull
    timeout connect 5000
    timeout client  50000
    timeout server  50000

frontend nebulagc
    bind *:8080
    default_backend nebulagc_servers
    
    # Sticky sessions for HA mode
    cookie NEBULAGC_SERVER insert indirect nocache

backend nebulagc_servers
    balance roundrobin
    option httpchk GET /health
    http-check expect status 200
    
    # Master (writes)
    server master 10.0.1.10:8080 check cookie master weight 100
    
    # Replicas (reads)
    server replica1 10.0.1.11:8080 check cookie replica1 weight 50 backup
    server replica2 10.0.1.12:8080 check cookie replica2 weight 50 backup
```

### NGINX

```nginx
# /etc/nginx/nginx.conf

upstream nebulagc_backend {
    # IP hash for sticky sessions
    ip_hash;
    
    # Master
    server 10.0.1.10:8080 max_fails=3 fail_timeout=30s weight=2;
    
    # Replicas
    server 10.0.1.11:8080 max_fails=3 fail_timeout=30s weight=1 backup;
    server 10.0.1.12:8080 max_fails=3 fail_timeout=30s weight=1 backup;
}

server {
    listen 8080;
    
    location /health {
        proxy_pass http://nebulagc_backend;
        proxy_connect_timeout 2s;
        proxy_read_timeout 2s;
    }
    
    location / {
        proxy_pass http://nebulagc_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        # Health check
        proxy_next_upstream error timeout http_500 http_502 http_503;
    }
}
```

### AWS Application Load Balancer

```yaml
# Terraform configuration
resource "aws_lb" "nebulagc" {
  name               = "nebulagc-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id
}

resource "aws_lb_target_group" "nebulagc" {
  name     = "nebulagc-tg"
  port     = 8080
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id
  
  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    path                = "/health"
    matcher             = "200"
  }
  
  stickiness {
    type            = "lb_cookie"
    cookie_duration = 86400
  }
}

resource "aws_lb_listener" "nebulagc" {
  load_balancer_arn = aws_lb.nebulagc.arn
  port              = "8080"
  protocol          = "HTTP"
  
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.nebulagc.arn
  }
}

resource "aws_lb_target_group_attachment" "master" {
  target_group_arn = aws_lb_target_group.nebulagc.arn
  target_id        = aws_instance.master.id
  port             = 8080
}

resource "aws_lb_target_group_attachment" "replica" {
  count            = 2
  target_group_arn = aws_lb_target_group.nebulagc.arn
  target_id        = aws_instance.replica[count.index].id
  port             = 8080
}
```

---

## Monitoring

### Health Checks

```bash
# Simple health check
curl -f http://localhost:8080/health || exit 1

# With timeout
timeout 5 curl -f http://localhost:8080/health || exit 1

# Check HA mode
curl -s http://localhost:8080/health | jq -r '.ha_mode'
```

### Prometheus Metrics

Key metrics to monitor:

```promql
# HA status
nebulagc_ha_is_master == 1

# Heartbeat health
rate(nebulagc_ha_heartbeat_errors_total[5m]) > 0

# Replica lag
nebulagc_ha_heartbeat_duration_seconds > 1

# State transitions (failovers)
increase(nebulagc_ha_state_transitions_total[1h]) > 0

# Request distribution
sum by (instance) (rate(nebulagc_http_requests_total[5m]))
```

### Grafana Dashboard

Example panels:

**HA Status Panel**:
```promql
nebulagc_ha_is_master
```
Display: Stat panel (0 = replica, 1 = master)

**Replica Count**:
```promql
nebulagc_ha_replicas_total
```
Display: Gauge (expected: 2+)

**Failover Events**:
```promql
increase(nebulagc_ha_state_transitions_total[24h])
```
Display: Stat panel (alert if > 0)

**Request Distribution**:
```promql
sum by (instance) (rate(nebulagc_http_requests_total[5m]))
```
Display: Graph (verify load balancing)

---

## Alerting Rules

### Prometheus Alerts

```yaml
# /etc/prometheus/rules/nebulagc.yml
groups:
  - name: nebulagc_ha
    interval: 30s
    rules:
    
    # No master available
    - alert: NebulaGCNoMaster
      expr: sum(nebulagc_ha_is_master) == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "No NebulaGC master available"
        description: "No server is currently in master mode. Writes are blocked."
    
    # Multiple masters (split-brain)
    - alert: NebulaGCSplitBrain
      expr: sum(nebulagc_ha_is_master) > 1
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "Multiple NebulaGC masters detected"
        description: "{{ $value }} servers claim to be master. Potential split-brain scenario."
    
    # Heartbeat failures
    - alert: NebulaGCHeartbeatFailures
      expr: rate(nebulagc_ha_heartbeat_errors_total[5m]) > 0
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "NebulaGC heartbeat failures"
        description: "{{ $labels.instance }} is experiencing heartbeat failures."
    
    # High replication lag
    - alert: NebulaGCHighReplicationLag
      expr: nebulagc_ha_heartbeat_duration_seconds > 5
      for: 2m
      labels:
        severity: warning
      annotations:
        summary: "High replication lag"
        description: "Replication lag on {{ $labels.instance }} is {{ $value }}s."
    
    # Replica offline
    - alert: NebulaGCReplicaOffline
      expr: nebulagc_ha_replicas_total < 2
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "NebulaGC replica offline"
        description: "Only {{ $value }} replicas are online (expected: 2+)."
    
    # Failover event
    - alert: NebulaGCFailover
      expr: increase(nebulagc_ha_state_transitions_total[5m]) > 0
      labels:
        severity: warning
      annotations:
        summary: "NebulaGC failover detected"
        description: "{{ $labels.instance }} transitioned from {{ $labels.from_state }} to {{ $labels.to_state }}."
```

---

## Testing HA

### Test 1: Master Failure

```bash
# 1. Identify master
curl -s http://localhost:8080/health | jq -r '.ha_mode'

# 2. Kill master process
sudo systemctl stop nebulagc-server  # On master node

# 3. Verify writes fail (Litestream) or continue (LiteFS)
curl -X POST http://localhost:8080/api/v1/nodes -d '{...}'

# 4. Promote replica (Litestream only)
# SSH to replica
sudo systemctl stop nebulagc-server
# Edit /etc/nebulagc/server.env
# NEBULAGC_HA_MODE=master
sudo systemctl start nebulagc-server

# 5. Verify writes succeed
curl -X POST http://localhost:8080/api/v1/nodes -d '{...}'
```

### Test 2: Replica Failure

```bash
# 1. Stop replica
sudo systemctl stop nebulagc-server  # On replica node

# 2. Verify reads still work
curl http://localhost:8080/health

# 3. Verify load balancer adjusts
# Check HAProxy stats or ALB target health

# 4. Restart replica
sudo systemctl start nebulagc-server

# 5. Verify replica rejoins
curl -s http://replica:8080/health | jq -r '.ha_mode'
```

### Test 3: Network Partition

```bash
# 1. Simulate partition with iptables
sudo iptables -A INPUT -s <master-ip> -j DROP
sudo iptables -A OUTPUT -d <master-ip> -j DROP

# 2. Verify replica behavior
# With LiteFS: Should maintain reads, block writes
# With Litestream: Serve stale data

# 3. Remove partition
sudo iptables -D INPUT -s <master-ip> -j DROP
sudo iptables -D OUTPUT -d <master-ip> -j DROP

# 4. Verify reconciliation
# Check logs for reconnection
```

---

## Maintenance Operations

### Rolling Updates

```bash
# 1. Update replicas first (one at a time)
# On replica-1
sudo systemctl stop nebulagc-server
sudo cp nebulagc-server-new /usr/local/bin/nebulagc-server
sudo systemctl start nebulagc-server

# Wait for health check to pass
watch curl -s http://replica-1:8080/health

# Repeat for replica-2

# 2. Update master (last)
# On master
sudo systemctl stop nebulagc-server
sudo cp nebulagc-server-new /usr/local/bin/nebulagc-server
sudo systemctl start nebulagc-server

# 3. Verify all nodes healthy
curl http://localhost:8080/health
```

### Master Promotion (Manual)

```bash
# Scenario: Master failed, need to promote replica

# 1. On replica to promote:
sudo systemctl stop nebulagc-server

# 2. Update configuration
sudo sed -i 's/NEBULAGC_HA_MODE=replica/NEBULAGC_HA_MODE=master/' /etc/nebulagc/server.env
sudo sed -i '/NEBULAGC_MASTER_URL/d' /etc/nebulagc/server.env

# 3. Start as master
sudo systemctl start nebulagc-server

# 4. Verify master mode
curl -s http://localhost:8080/health | jq -r '.ha_mode'

# 5. Update load balancer to point to new master

# 6. Update remaining replicas
# On other replicas:
sudo sed -i 's/NEBULAGC_MASTER_URL=.*/NEBULAGC_MASTER_URL=http:\/\/new-master:8080/' /etc/nebulagc/server.env
sudo systemctl restart nebulagc-server
```

### Scaling Replicas

```bash
# Add new replica

# 1. Install nebulagc-server on new node

# 2. Configure as replica
cat > /etc/nebulagc/server.env <<EOF
NEBULAGC_HA_MODE=replica
NEBULAGC_MASTER_URL=http://master:8080
NEBULAGC_DB_PATH=/var/lib/nebulagc-server/nebulagc.db
NEBULAGC_HMAC_SECRET=<same-as-master>
EOF

# 3. Start service
sudo systemctl start nebulagc-server

# 4. Add to load balancer
# Update HAProxy/NGINX/ALB config

# 5. Verify replication
curl -s http://new-replica:8080/health
```

---

## Best Practices

1. **Always Use Load Balancer**: Even with 1 server
2. **Monitor HA Metrics**: Set up alerts for master failures
3. **Test Failover Regularly**: Monthly chaos testing
4. **Document Runbooks**: Clear procedures for common scenarios
5. **Automate Failover**: Use LiteFS for automatic failover
6. **Backup Regularly**: Litestream to S3 for disaster recovery
7. **Use Odd Number of Nodes**: 3, 5, 7 for Consul quorum
8. **Set Resource Limits**: Prevent resource exhaustion
9. **Secure Communication**: TLS between master and replicas
10. **Plan Capacity**: Ensure replicas can handle full load

---

## Troubleshooting

### Split-Brain Scenario

**Symptoms**: Multiple servers claim to be master

**Cause**: Network partition or Consul failure

**Resolution**:
```bash
# 1. Identify the true master (most recent writes)
curl -s http://server-1:8080/api/v1/nodes | jq '.[0].updated_at'
curl -s http://server-2:8080/api/v1/nodes | jq '.[0].updated_at'

# 2. Force others to replica mode
# On incorrect master(s):
sudo systemctl stop nebulagc-server
sudo sed -i 's/NEBULAGC_HA_MODE=master/NEBULAGC_HA_MODE=replica/' /etc/nebulagc/server.env
sudo systemctl start nebulagc-server

# 3. Verify single master
curl http://localhost:8080/metrics | grep nebulagc_ha_is_master
```

### Stale Reads on Replicas

**Symptoms**: Replicas serve old data

**Cause**: Replication lag or failure

**Check**:
```bash
# Check last heartbeat
curl http://replica:8080/metrics | grep nebulagc_ha_last_heartbeat

# Compare with current time
date +%s
```

**Resolution**:
```bash
# Restart replica
sudo systemctl restart nebulagc-server

# Force resync (if needed)
sudo systemctl stop nebulagc-server
sudo rm /var/lib/nebulagc-server/nebulagc.db*
sudo systemctl start nebulagc-server
```

### Load Balancer Not Routing Correctly

**Check health endpoint**:
```bash
# On each server
curl -v http://server-1:8080/health
curl -v http://server-2:8080/health
curl -v http://server-3:8080/health
```

**Check load balancer logs**:
```bash
# HAProxy
sudo tail -f /var/log/haproxy.log

# NGINX
sudo tail -f /var/log/nginx/access.log

# AWS ALB
aws elbv2 describe-target-health --target-group-arn <arn>
```

---

## Additional Resources

- [HAProxy Configuration Manual](http://cbonte.github.io/haproxy-dconv/)
- [NGINX Load Balancing](https://docs.nginx.com/nginx/admin-guide/load-balancer/)
- [AWS ALB Documentation](https://docs.aws.amazon.com/elasticloadbalancing/)
- [Consul Documentation](https://www.consul.io/docs)
- [SQLite WAL Mode](https://www.sqlite.org/wal.html)
- [High Availability Patterns](https://en.wikipedia.org/wiki/High_availability)
