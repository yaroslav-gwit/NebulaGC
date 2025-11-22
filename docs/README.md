# NebulaGC Documentation

Welcome to the NebulaGC documentation! This guide will help you navigate the available documentation.

## Quick Links

- **[Getting Started Guide](getting-started.md)** - Start here if you're new to NebulaGC
- **[Architecture Overview](architecture.md)** - Understand how NebulaGC works
- **[API Reference](api-reference.md)** - Complete API documentation
- **[Operations Manual](operations.md)** - Deployment and maintenance

## Documentation Overview

### For Users

**Getting Started**
- [Getting Started Guide](getting-started.md) - Installation and initial setup
- [Quick Start](../README.md#quick-start) - Get running in 5 minutes

**Using NebulaGC**
- [API Reference](api-reference.md) - Complete API endpoint documentation
- [API Examples](../README.md#api-examples) - Common API usage patterns

### For Operators

**Deployment**
- [Operations Manual](operations.md) - Complete operational guide
- [Deployment Guides](operations.md#deployment) - systemd, Docker, Kubernetes
- [High Availability Setup](operations.md#high-availability-deployment) - HA configuration
- [Monitoring](operations.md#monitoring) - Prometheus metrics and Grafana dashboards
- [Backup and Recovery](operations.md#backup-and-recovery) - Database backup procedures

**Maintenance**
- [Troubleshooting](operations.md#troubleshooting) - Common issues and solutions
- [Maintenance Tasks](operations.md#maintenance) - Routine maintenance procedures
- [Security Operations](operations.md#security-operations) - Token rotation, certificate renewal
- [Disaster Recovery](operations.md#disaster-recovery) - Recovery procedures

### For Developers

**Architecture**
- [Architecture Overview](architecture.md) - System design and components
- [Data Model](architecture.md#data-model) - Entity relationships
- [API Design](architecture.md#api-design) - REST API principles
- [Security Architecture](architecture.md#security-architecture) - Authentication and authorization

**Contributing**
- [Contributing Guidelines](../CONTRIBUTING.md) - How to contribute
- [Development Setup](../CONTRIBUTING.md#getting-started) - Local development environment
- [Coding Standards](../CONTRIBUTING.md#coding-standards) - Code style and best practices
- [Testing Requirements](../CONTRIBUTING.md#testing-requirements) - Test coverage expectations

**Project Information**
- [CHANGELOG](../CHANGELOG.md) - Version history and release notes
- [PROGRESS](../PROGRESS.md) - Implementation progress and roadmap

## Documentation Structure

```
docs/
├── README.md              # This file - documentation index
├── getting-started.md     # Quick start guide (500+ lines)
├── architecture.md        # Architecture overview (700+ lines)
├── api-reference.md       # Complete API documentation (1000+ lines)
└── operations.md          # Operational runbook (800+ lines)

Root Documentation:
├── README.md              # Project overview
├── CHANGELOG.md           # Version history
├── CONTRIBUTING.md        # Contribution guidelines
└── PROGRESS.md            # Implementation progress
```

## Common Tasks

### Getting Started

1. **Install NebulaGC**
   - [Build from Source](getting-started.md#option-1-build-from-source)
   - [Using Docker](getting-started.md#option-3-docker)

2. **Initial Setup**
   - [Configure Server](getting-started.md#initial-server-setup)
   - [Create First Tenant](getting-started.md#creating-your-first-tenant-and-cluster)
   - [Create First Node](getting-started.md#creating-your-first-node)

3. **Deploy Config Bundle**
   - [Upload Bundle](getting-started.md#uploading-a-config-bundle)
   - [Download Bundle](api-reference.md#get-apiv1bundlescluster_idversion)

### Operations

1. **Deploy to Production**
   - [systemd Service](operations.md#systemd-service)
   - [Docker Deployment](operations.md#docker-deployment)
   - [Kubernetes Deployment](operations.md#kubernetes-deployment)

2. **Configure HA**
   - [HA Setup](operations.md#high-availability-deployment)
   - [HAProxy Config](operations.md#haproxy-configuration)
   - [Database Replication](operations.md#database-replication-litestream)

3. **Monitor Service**
   - [Health Checks](operations.md#health-checks)
   - [Prometheus Metrics](operations.md#prometheus-metrics)
   - [Log Aggregation](operations.md#log-aggregation)

4. **Backup Database**
   - [Manual Backup](operations.md#manual-backup)
   - [Automated Backup](operations.md#automated-backup-script)
   - [Restore Database](operations.md#database-restore)

### Development

1. **Set Up Development Environment**
   - [Prerequisites](../CONTRIBUTING.md#prerequisites)
   - [Clone and Build](../CONTRIBUTING.md#getting-started)
   - [Run Tests](../CONTRIBUTING.md#running-tests)

2. **Make Changes**
   - [Create Branch](../CONTRIBUTING.md#create-a-branch)
   - [Make Changes](../CONTRIBUTING.md#make-your-changes)
   - [Write Tests](../CONTRIBUTING.md#testing-requirements)

3. **Submit Pull Request**
   - [Commit Guidelines](../CONTRIBUTING.md#commit-your-changes)
   - [Create PR](../CONTRIBUTING.md#submit-a-pull-request)
   - [Review Process](../CONTRIBUTING.md#code-review)

## API Quick Reference

### Authentication

```bash
# All requests require Bearer token
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/v1/nodes
```

### Common Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Health check |
| `/version` | GET | Version info |
| `/api/v1/nodes` | GET | List nodes |
| `/api/v1/nodes` | POST | Create node |
| `/api/v1/nodes/:id` | GET | Get node |
| `/api/v1/nodes/:id` | PUT | Update node |
| `/api/v1/nodes/:id` | DELETE | Delete node |
| `/api/v1/bundles/:cluster_id` | POST | Upload bundle |
| `/api/v1/bundles/:cluster_id/latest` | GET | Download latest bundle |
| `/api/v1/topology/:cluster_id` | GET | Get topology |

See [API Reference](api-reference.md) for complete documentation.

## Troubleshooting

### Service Issues

- **Service won't start**: [Troubleshooting Guide](operations.md#service-wont-start)
- **Authentication failures**: [Auth Troubleshooting](operations.md#authentication-failures)
- **High CPU usage**: [Performance Troubleshooting](operations.md#high-cpu-usage)
- **Database locked**: [Database Issues](operations.md#database-locked)

### Getting Help

- **GitHub Issues**: [Report a Bug](https://github.com/yaroslav-gwit/nebulagc/issues/new?template=bug_report.md)
- **Discussions**: [Ask a Question](https://github.com/yaroslav-gwit/nebulagc/discussions)
- **Documentation**: Search this documentation

## Version Information

**Current Version**: v1.0.0-rc (Release Candidate)

**Status**: Production Ready

**What's Included**:
- ✅ Complete REST API (20+ endpoints)
- ✅ High Availability support
- ✅ Multi-tenant architecture
- ✅ Security hardening
- ✅ Comprehensive testing (70+ tests)
- ✅ Complete documentation (3,500+ lines)

See [CHANGELOG](../CHANGELOG.md) for version history.

## Contributing

We welcome contributions! Please read our [Contributing Guidelines](../CONTRIBUTING.md) to get started.

**Key Points**:
- Follow the [Code of Conduct](../CONTRIBUTING.md#code-of-conduct)
- Write tests for all changes
- Follow Go coding standards
- Submit clear pull requests

## License

See [LICENSE](../LICENSE) file for details.

## Support

- **Documentation**: You're reading it!
- **Issues**: https://github.com/yaroslav-gwit/nebulagc/issues
- **Discussions**: https://github.com/yaroslav-gwit/nebulagc/discussions
- **Email**: support@example.com (if available)

---

**Last Updated**: November 22, 2025  
**Documentation Version**: v1.0.0
