# Examples

This directory contains practical examples for using and integrating with CLS Backend.

## Directory Structure

```
examples/
├── api/                    # API usage examples
│   ├── curl-examples.sh    # cURL command examples
│   ├── python-client.py    # Python integration example
│   └── javascript-client.js # JavaScript/Node.js example
├── kubernetes/             # Kubernetes deployment examples
│   ├── basic-deployment/   # Simple deployment example
│   ├── production-ready/   # Production-ready configuration
│   └── monitoring/         # Monitoring and observability setup
├── local-dev/             # Local development examples
│   ├── docker-compose.yml # Complete local development stack
│   ├── .env.example       # Environment variable examples
│   └── setup.sh           # Local setup script
└── controllers/           # Controller integration examples
    ├── simple-controller/ # Basic controller implementation
    └── platform-controller/ # Platform-specific controller
```

## Quick Start Examples

### 1. API Usage with cURL

```bash
# See examples/api/curl-examples.sh for complete API examples
./examples/api/curl-examples.sh
```

### 2. Local Development

```bash
# Set up complete local development environment
cd examples/local-dev
docker-compose up -d
./setup.sh
```

### 3. Kubernetes Deployment

```bash
# Deploy to Kubernetes with production-ready configuration
kubectl apply -f examples/kubernetes/production-ready/
```

### 4. Python Integration

```python
# See examples/api/python-client.py
from cls_client import CLSClient

client = CLSClient(base_url="http://localhost:8080", user_email="user@example.com")
clusters = client.list_clusters()
```

## Example Categories

### API Examples

Practical examples of using the CLS Backend REST API:
- Complete CRUD operations
- Error handling patterns
- Authentication examples
- Client library implementations

### Deployment Examples

Real-world deployment configurations:
- Basic Kubernetes deployment
- Production-ready setup with monitoring
- High-availability configuration
- Security best practices

### Local Development

Complete local development environment:
- Docker Compose stack
- Database setup and migrations
- Pub/Sub emulator configuration
- Development tools and debugging

### Controller Examples

Examples for building controllers:
- Simple status-reporting controller
- Platform-specific controller (GCP, AWS, Azure)
- Event filtering and processing
- Status aggregation patterns

## Getting Started

1. **Choose your use case** from the examples above
2. **Copy the relevant example** to your project
3. **Customize** the configuration for your needs
4. **Follow the README** in each example directory

Each example includes:
- Complete working code
- Configuration files
- Setup instructions
- Best practices
- Troubleshooting tips

## Contributing Examples

We welcome contributions of new examples! Please:

1. Follow the existing structure and naming conventions
2. Include complete, working examples
3. Add comprehensive README documentation
4. Test examples thoroughly
5. Submit a pull request

See [Contributing Guidelines](../CONTRIBUTING.md) for more details.