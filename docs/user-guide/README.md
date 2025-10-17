# User Guide

This section contains documentation for users who want to interact with the CLS Backend API to manage clusters.

## Getting Started

- **[Quick Start](quick-start.md)** - Get up and running in 5 minutes
- **[API Usage](api-usage.md)** - Common API patterns and examples
- **[NodePools](nodepools.md)** - Manage groups of compute nodes within clusters
- **[Troubleshooting](troubleshooting.md)** - Solutions to common issues

## What You'll Learn

- How to create, update, and delete clusters
- Managing nodepool lifecycle and scaling
- Understanding cluster status and conditions
- Working with the simplified API endpoints
- Best practices for API usage

## Prerequisites

Before using CLS Backend, you should have:
- Basic understanding of REST APIs
- Access to a CLS Backend instance
- User credentials (X-User-Email header for development)

## Quick Reference

### Essential API Endpoints

```bash
# Health check
GET /health

# Clusters
GET /api/v1/clusters              # List clusters
POST /api/v1/clusters             # Create cluster
GET /api/v1/clusters/{id}         # Get cluster details
PUT /api/v1/clusters/{id}         # Update cluster
DELETE /api/v1/clusters/{id}      # Delete cluster
GET /api/v1/clusters/{id}/status  # Get cluster status

# NodePools
GET /api/v1/clusters/{clusterId}/nodepools    # List cluster nodepools
POST /api/v1/clusters/{clusterId}/nodepools   # Create nodepool
GET /api/v1/nodepools/{id}                    # Get nodepool details
PUT /api/v1/nodepools/{id}                    # Update nodepool
DELETE /api/v1/nodepools/{id}                 # Delete nodepool
GET /api/v1/nodepools/{id}/status             # Get nodepool status
```

### Authentication

All API requests require the `X-User-Email` header:

```bash
curl -H "X-User-Email: user@example.com" \
  http://localhost:8080/api/v1/clusters
```

## Next Steps

1. Start with the [Quick Start](quick-start.md) guide
2. Explore [API Usage](api-usage.md) patterns
3. Try the [Examples](examples.md) for real-world scenarios
4. Learn about [NodePools](nodepools.md) for managing compute resources
5. Check the [Troubleshooting](troubleshooting.md) guide if you encounter issues
6. See the [API Reference](../reference/api.md) for complete documentation

## Related Documentation

### For Deployment
- **[Deployment Guide](../deployment/)** - Production deployment instructions
- **[Kubernetes Deployment](../deployment/kubernetes.md)** - Complete Kubernetes setup
- **[Monitoring](../deployment/monitoring.md)** - Health checks and observability

### For Development
- **[Developer Guide](../developer-guide/)** - Contributing and extending CLS Backend
- **[Architecture Overview](../developer-guide/architecture.md)** - System design and components
- **[Local Setup](../developer-guide/local-setup.md)** - Development environment

---
**Quick Links**: [📖 Documentation Home](../README.md) | [🚀 Quick Start](quick-start.md) | [📚 API Reference](../reference/api.md)