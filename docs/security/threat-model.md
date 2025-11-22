# NebulaGC Threat Model

**Document Version**: 1.0  
**Date**: 2025-11-22  
**Status**: Active

---

## 1. System Overview

NebulaGC is a centralized control plane for managing Nebula overlay networks at scale. It consists of three main components:

1. **Control Plane Server** (`nebulagc-server`)
   - HTTP API for node management and configuration
   - SQLite database for state persistence
   - HA support with master/replica architecture
   - Lighthouse process management

2. **Daemon** (`nebulagc`)
   - Runs on each node
   - Polls control plane for configuration updates
   - Manages local Nebula processes
   - Supports multiple clusters per node

3. **SDK** (Go client library)
   - Used by custom applications
   - Provides programmatic access to API
   - Handles authentication and failover

---

## 2. Security Boundaries

### Trust Zones

```
┌─────────────────────────────────────────────────────────────┐
│                     EXTERNAL UNTRUSTED                       │
│  • Internet                                                  │
│  • Potentially malicious actors                              │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        │ TLS (optional)
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                  CONTROL PLANE BOUNDARY                      │
│  • nebulagc-server API endpoints                             │
│  • Authentication required                                   │
│  • Rate limiting enforced                                    │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        │ Token authentication
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                 AUTHENTICATED INTERNAL                       │
│  • Authorized daemons                                        │
│  • Authorized SDK clients                                    │
│  • HA replicas                                               │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        │ Filesystem, DB access
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                   TRUSTED INTERNAL                           │
│  • SQLite database                                           │
│  • Configuration bundles                                     │
│  • Lighthouse processes                                      │
│  • System resources                                          │
└─────────────────────────────────────────────────────────────┘
```

### Security Assumptions

**Trusted**:
- Operating system and kernel
- Filesystem permissions enforced by OS
- SQLite database integrity
- Go runtime and standard library
- TLS certificate infrastructure (if used)

**Semi-Trusted**:
- Authenticated daemons (can be compromised nodes)
- Authenticated SDK clients
- HA replica instances

**Untrusted**:
- Network traffic (requires authentication)
- User inputs to API
- Configuration bundle contents (validated)
- External URLs in configuration

---

## 3. Asset Inventory

### Critical Assets

1. **Node Tokens** (High Value)
   - Purpose: Authenticate daemons to control plane
   - Storage: Bcrypt hashed in database, plaintext in daemon config
   - Impact if compromised: Node impersonation, unauthorized config access

2. **Cluster Tokens** (High Value)
   - Purpose: Initial node enrollment
   - Storage: Bcrypt hashed in database, shared out-of-band
   - Impact if compromised: Unauthorized node creation

3. **HMAC Secret** (Critical)
   - Purpose: Token generation and validation
   - Storage: Environment variable, not in database
   - Impact if compromised: Full authentication bypass

4. **Configuration Bundles** (Medium Value)
   - Purpose: Nebula network configuration and PKI
   - Storage: tar.gz in database BLOB
   - Impact if compromised: Network traffic interception, MitM

5. **Database** (High Value)
   - Purpose: All state persistence
   - Storage: SQLite file on disk
   - Impact if compromised: Full control plane compromise

### Data Flows

```
Daemon Registration:
  Daemon → [Cluster Token] → Control Plane
         ← [Node Token] ←

Configuration Update:
  Daemon → [Node Token] → Control Plane
         ← [Config Bundle] ←

Lighthouse Management:
  Control Plane → [Nebula Binary] → Lighthouse Process
                ← [Process Status] ←
```

---

## 4. Threat Analysis

### 4.1 Authentication Threats

#### T1: Token Theft
**Severity**: High  
**Attack Vector**: Attacker gains access to daemon configuration file containing node token  
**Impact**: Node impersonation, unauthorized configuration access  
**Mitigations**:
- File permissions (600) on daemon config
- Token rotation capability
- Rate limiting on authentication failures
- Logging of authentication events

#### T2: Token Brute Force
**Severity**: Medium  
**Attack Vector**: Attacker attempts to guess valid tokens  
**Impact**: Potential authentication bypass  
**Mitigations**:
- Tokens are 32+ bytes from crypto/rand (impossible to brute force)
- Rate limiting (10 failures/min per IP)
- Exponential backoff on failures
- Bcrypt hashing (cost 12)

#### T3: HMAC Secret Compromise
**Severity**: Critical  
**Attack Vector**: Attacker accesses server environment variables or memory  
**Impact**: Complete authentication bypass, token forgery  
**Mitigations**:
- Stored only in environment variables (not database)
- Server process isolation
- Memory protection (OS-level)
- Restart required to change secret

#### T4: Replay Attacks
**Severity**: Low  
**Attack Vector**: Attacker captures and replays valid requests  
**Impact**: Limited, tokens are long-lived  
**Mitigations**:
- TLS prevents MITM (if enabled)
- Tokens are per-node (limited scope)
- Token rotation available
- Future: Add timestamp/nonce to requests

### 4.2 Injection Threats

#### T5: SQL Injection
**Severity**: Critical (if present)  
**Attack Vector**: Malicious input in API parameters  
**Impact**: Database compromise, data exfiltration  
**Mitigations**:
- SQLc generates parameterized queries only
- No string concatenation in SQL
- Input validation on all parameters
- Regular security audits

#### T6: Path Traversal
**Severity**: High  
**Attack Vector**: Malicious paths in config directories  
**Impact**: Unauthorized file access, code execution  
**Mitigations**:
- Absolute path validation
- No .. sequences allowed
- Restricted to specific directories
- Filesystem permissions

#### T7: Command Injection
**Severity**: Critical (if present)  
**Attack Vector**: Malicious input passed to shell commands  
**Impact**: Remote code execution  
**Mitigations**:
- No shell commands executed with user input
- exec.Command used directly (not via shell)
- Limited use of external processes
- Input validation

### 4.3 Network Threats

#### T8: SSRF (Server-Side Request Forgery)
**Severity**: High  
**Attack Vector**: Malicious URLs in control plane configuration  
**Impact**: Internal network scanning, cloud metadata access  
**Mitigations**:
- URL validation for replica discovery
- No access to private IP ranges
- Timeout on external requests
- Protocol restrictions (http/https only)

#### T9: Man-in-the-Middle
**Severity**: High (without TLS)  
**Attack Vector**: Network eavesdropping  
**Impact**: Token theft, configuration tampering  
**Mitigations**:
- TLS support available
- Certificate validation
- Recommend TLS for production
- Warn if TLS disabled

#### T10: DDoS (Denial of Service)
**Severity**: Medium  
**Attack Vector**: Overwhelming control plane with requests  
**Impact**: Service unavailability  
**Mitigations**:
- Rate limiting (per-IP, per-node)
- Connection limits
- Request timeouts
- Resource limits (memory, CPU)

### 4.4 Data Exposure Threats

#### T11: Sensitive Data in Logs
**Severity**: Medium  
**Attack Vector**: Tokens/secrets logged and exposed  
**Impact**: Authentication compromise  
**Mitigations**:
- Tokens never logged
- Passwords never logged
- Sensitive fields redacted
- Regular log audits

#### T12: Error Message Information Disclosure
**Severity**: Low  
**Attack Vector**: Detailed error messages reveal system internals  
**Impact**: Assists further attacks  
**Mitigations**:
- Generic error messages for clients
- Detailed errors only in server logs
- No stack traces exposed
- Version information controlled

#### T13: Database File Exposure
**Severity**: Critical  
**Attack Vector**: Unauthorized access to SQLite file  
**Impact**: Full control plane compromise  
**Mitigations**:
- Filesystem permissions (600)
- Database in restricted directory
- No remote database access
- Regular backups (encrypted)

### 4.5 Concurrency Threats

#### T14: Race Conditions
**Severity**: Medium  
**Attack Vector**: Concurrent access to shared state  
**Impact**: Data corruption, crashes, undefined behavior  
**Mitigations**:
- Proper mutex usage
- Atomic operations
- Race detector in tests
- Code review for concurrency

#### T15: Deadlocks
**Severity**: Low  
**Attack Vector**: Improper lock ordering  
**Impact**: Service hang  
**Mitigations**:
- Consistent lock ordering
- Timeout on lock acquisition
- Deadlock detection in tests

---

## 5. Attack Scenarios

### Scenario 1: Compromised Node
**Attacker Goal**: Use compromised daemon to attack control plane

**Attack Steps**:
1. Attacker compromises one daemon node
2. Extracts node token from config file
3. Attempts to access other nodes' configurations
4. Attempts to create/modify nodes
5. Attempts to upload malicious bundles

**Defenses**:
- Node tokens scoped to single node (can't access other nodes)
- Write operations require elevated privileges
- Bundle validation (tar.gz format, required files)
- Rate limiting prevents mass operations
- Audit logging tracks all modifications

**Residual Risk**: Medium (limited to single node's config)

### Scenario 2: Control Plane Server Compromise
**Attacker Goal**: Full control of Nebula infrastructure

**Attack Steps**:
1. Attacker gains shell access to server
2. Reads SQLite database file
3. Extracts all node tokens (hashed)
4. Accesses HMAC secret from environment
5. Forges tokens for all nodes

**Defenses**:
- Server hardening (minimal services, firewall)
- Filesystem permissions
- Process isolation
- Intrusion detection
- Database encryption at rest (future)

**Residual Risk**: High (game over if server compromised)

### Scenario 3: Network Eavesdropping
**Attacker Goal**: Steal tokens via network sniffing

**Attack Steps**:
1. Attacker positions on network path
2. Captures daemon-to-control plane traffic
3. Extracts authentication tokens
4. Replays tokens to control plane

**Defenses**:
- TLS encryption (optional but recommended)
- Certificate pinning (future)
- Token rotation
- Short-lived tokens (future)

**Residual Risk**: High without TLS, Low with TLS

---

## 6. Security Controls Summary

### Implemented Controls

| Control | Type | Effectiveness |
|---------|------|---------------|
| Token authentication | Preventive | High |
| Bcrypt password hashing | Preventive | High |
| Rate limiting | Preventive | Medium |
| SQLc parameterized queries | Preventive | High |
| Input validation | Preventive | Medium |
| Logging | Detective | Medium |
| HA failover | Corrective | Medium |

### Recommended Additional Controls

| Control | Type | Priority |
|---------|------|----------|
| TLS encryption | Preventive | High |
| Database encryption | Preventive | Medium |
| Certificate pinning | Preventive | Medium |
| Time-based tokens | Preventive | Low |
| Multi-factor auth | Preventive | Low |
| SIEM integration | Detective | Medium |
| Automated backups | Corrective | High |

---

## 7. Compliance Considerations

### Data Protection
- No personally identifiable information (PII) stored
- Node IDs and cluster IDs are UUIDs (not sensitive)
- Tokens treated as secrets (hashed at rest)

### Audit Requirements
- All administrative actions logged
- Authentication events logged
- Structured logging for SIEM integration

### Access Control
- Principle of least privilege
- Node tokens scoped to single node
- No default/shared credentials

---

## 8. Security Testing Recommendations

### Static Analysis
- `go vet` for common issues
- `golangci-lint` with security linters (gosec)
- `staticcheck` for bugs
- Regular dependency vulnerability scanning

### Dynamic Testing
- Integration tests with race detector
- Fuzzing for input validation
- Penetration testing (annual)
- Load testing for DoS resilience

### Security Audits
- Code review for security issues
- Third-party security assessment (before v1.0)
- Regular threat model updates
- Incident response plan testing

---

## 9. Incident Response

### Detection
- Monitor authentication failure rates
- Alert on repeated failed authentications
- Track unusual API usage patterns
- Monitor system resource usage

### Response
- Isolate affected components
- Rotate compromised tokens
- Review audit logs
- Patch vulnerabilities
- Post-incident analysis

---

## 10. Future Enhancements

1. **Token Rotation**: Automated periodic rotation
2. **Certificate Pinning**: Prevent MITM with TLS
3. **Database Encryption**: Encrypt SQLite file at rest
4. **Audit Trail**: Immutable audit log
5. **Security Monitoring**: Real-time threat detection
6. **Backup Encryption**: Encrypted backups to S3/GCS
7. **HSM Integration**: Hardware security module for HMAC secret

---

## Document Maintenance

This threat model should be reviewed and updated:
- When new features are added
- After security incidents
- At least quarterly
- Before major releases

**Last Reviewed**: 2025-11-22  
**Next Review**: 2026-02-22
