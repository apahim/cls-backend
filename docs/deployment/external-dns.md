# External DNS Configuration Guide

This document provides comprehensive guidance for configuring external-dns integration with the CLS Backend service to automatically manage DNS records for your LoadBalancer service.

## Overview

External DNS is a Kubernetes controller that automatically creates and manages DNS records for Kubernetes services and ingresses. When enabled, it will automatically create DNS A records pointing to your LoadBalancer's external IP address.

## Prerequisites

Before enabling external-dns integration, ensure you have:

1. **External DNS Controller**: External DNS must be installed and running in your cluster
2. **DNS Provider Access**: External DNS must have permissions to manage DNS records in your provider
3. **LoadBalancer Service**: The cls-backend external service must be of type LoadBalancer (already configured)

### Installing External DNS Controller

#### Google Cloud DNS

```bash
# Create service account for external-dns
gcloud iam service-accounts create external-dns \
  --display-name="External DNS Service Account"

# Grant DNS admin permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:external-dns@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/dns.admin"

# Install external-dns with Helm
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm install external-dns external-dns/external-dns \
  --set provider=google \
  --set google.project=$PROJECT_ID \
  --set serviceAccount.annotations."iam\.gke\.io/gcp-service-account"="external-dns@$PROJECT_ID.iam.gserviceaccount.com"
```

#### AWS Route53

```bash
# Install external-dns for AWS
helm install external-dns external-dns/external-dns \
  --set provider=aws \
  --set aws.zoneType=public \
  --set txtOwnerId=cls-backend
```

#### Cloudflare

```bash
# Create secret with Cloudflare API token
kubectl create secret generic cloudflare-api-token \
  --from-literal=api-token=your-cloudflare-api-token

# Install external-dns for Cloudflare
helm install external-dns external-dns/external-dns \
  --set provider=cloudflare \
  --set cloudflare.secretName=cloudflare-api-token
```

## Configuration

### Helm Values Configuration

Enable external-dns in your `values.yaml`:

```yaml
# External DNS configuration (optional)
externalDns:
  # Enable external-dns integration for automatic DNS record creation
  enabled: true
  # DNS hostname for the service (required if enabled)
  hostname: "cls-backend.example.com"
  # DNS zone (optional, used for validation)
  zone: "example.com"
  # TTL for DNS records in seconds (default: 300)
  ttl: 300
  # Additional annotations to pass to external-dns
  annotations:
    # Cloudflare-specific annotations
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
    # AWS Route53-specific annotations
    external-dns.alpha.kubernetes.io/aws-health-check-id: "health-check-id"
```

### Environment-Specific Examples

#### Development Environment

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend-dev.dev.example.com"
  zone: "dev.example.com"
  ttl: 60  # Short TTL for development
```

#### Staging Environment

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend-staging.staging.example.com"
  zone: "staging.example.com"
  ttl: 300
```

#### Production Environment

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  ttl: 300
  annotations:
    # Enable health checks for production
    external-dns.alpha.kubernetes.io/aws-health-check-id: "prod-health-check"
```

## DNS Provider-Specific Configuration

### Google Cloud DNS

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # Set record type (A is default)
    external-dns.alpha.kubernetes.io/set-identifier: "cls-backend-primary"
```

### AWS Route53

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # Enable health checks
    external-dns.alpha.kubernetes.io/aws-health-check-id: "cls-backend-health"
    # Set weight for weighted routing
    external-dns.alpha.kubernetes.io/aws-weight: "100"
    # Set geolocation
    external-dns.alpha.kubernetes.io/aws-geolocation-continent-code: "NA"
```

### Cloudflare

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # Disable Cloudflare proxy (recommended for API services)
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
    # Set priority for SRV records
    external-dns.alpha.kubernetes.io/cloudflare-priority: "10"
```

### Azure DNS

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # Set resource group for Azure DNS
    external-dns.alpha.kubernetes.io/azure-resource-group: "my-resource-group"
```

## Deployment

### Deploy with External DNS Enabled

```bash
# Deploy with external-dns enabled
helm upgrade --install cls-backend ./deploy/helm-application \
  --set externalDns.enabled=true \
  --set externalDns.hostname=cls-backend.example.com \
  --set externalDns.zone=example.com \
  --namespace cls-system
```

### Verify DNS Record Creation

```bash
# Check service for external-dns annotations
kubectl get service cls-backend-external -n cls-system -o yaml

# Check external-dns logs
kubectl logs -l app.kubernetes.io/name=external-dns -n default

# Verify DNS record was created
nslookup cls-backend.example.com
dig cls-backend.example.com
```

## Advanced Configuration

### Multiple Hostnames

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com,api.example.com"
  zone: "example.com"
  ttl: 300
```

### Subdomain Delegation

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.api.example.com"
  zone: "api.example.com"  # Delegated subdomain
  ttl: 300
```

### Custom DNS Record Types

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # Create CNAME instead of A record
    external-dns.alpha.kubernetes.io/target: "lb.example.com"
```

## Security Considerations

### DNS Security

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # Enable DNSSEC validation
    external-dns.alpha.kubernetes.io/dnssec: "true"
    # Set security policy
    external-dns.alpha.kubernetes.io/policy: "sync"
```

### Access Control

Ensure external-dns has minimal required permissions:

```yaml
# For Google Cloud DNS
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-dns
  annotations:
    iam.gke.io/gcp-service-account: external-dns@PROJECT_ID.iam.gserviceaccount.com
---
# IAM policy binding (apply via gcloud)
# gcloud projects add-iam-policy-binding PROJECT_ID \
#   --member="serviceAccount:external-dns@PROJECT_ID.iam.gserviceaccount.com" \
#   --role="roles/dns.admin"
```

### Network Security

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # Restrict to specific source IPs (provider-dependent)
    external-dns.alpha.kubernetes.io/source-restriction: "1.2.3.4/32,5.6.7.8/32"
```

## Monitoring and Troubleshooting

### Health Checks

```bash
# Check external-dns controller status
kubectl get pods -l app.kubernetes.io/name=external-dns

# Check external-dns logs for errors
kubectl logs -l app.kubernetes.io/name=external-dns --tail=100

# Verify service annotations
kubectl describe service cls-backend-external -n cls-system
```

### DNS Validation

```bash
# Test DNS resolution
dig cls-backend.example.com +short

# Check DNS propagation
dig cls-backend.example.com @8.8.8.8
dig cls-backend.example.com @1.1.1.1

# Test HTTP connectivity
curl -v http://cls-backend.example.com/health
```

### Common Issues

#### 1. DNS Record Not Created

```bash
# Check external-dns permissions
kubectl logs -l app.kubernetes.io/name=external-dns | grep -i permission

# Verify hostname annotation
kubectl get service cls-backend-external -n cls-system -o jsonpath='{.metadata.annotations}'

# Check external-dns configuration
kubectl get configmap external-dns -o yaml
```

#### 2. DNS Record Points to Wrong IP

```bash
# Check service external IP
kubectl get service cls-backend-external -n cls-system

# Force external-dns sync
kubectl annotate service cls-backend-external external-dns.alpha.kubernetes.io/force-update=$(date +%s) -n cls-system

# Check external-dns events
kubectl get events --field-selector involvedObject.name=cls-backend-external -n cls-system
```

#### 3. DNS TTL Issues

```bash
# Check current TTL
dig cls-backend.example.com | grep -E "^cls-backend.*IN.*A"

# Update TTL annotation
kubectl annotate service cls-backend-external external-dns.alpha.kubernetes.io/ttl=60 -n cls-system
```

## Best Practices

### Production Recommendations

1. **Use appropriate TTL values**:
   - Development: 60-300 seconds
   - Staging: 300-600 seconds
   - Production: 300-3600 seconds

2. **Monitor DNS changes**:
   - Set up alerts for DNS record modifications
   - Log all external-dns activities
   - Backup DNS zone configurations

3. **Security hardening**:
   - Use least-privilege service accounts
   - Enable DNSSEC where supported
   - Monitor for unauthorized DNS changes

4. **High availability**:
   - Deploy external-dns with multiple replicas
   - Use health checks for production services
   - Configure proper backup DNS providers

### Development Workflow

```bash
# Enable external-dns for development
helm upgrade cls-backend ./deploy/helm-application \
  --set externalDns.enabled=true \
  --set externalDns.hostname=cls-backend-dev.dev.example.com \
  --set externalDns.ttl=60 \
  --reuse-values

# Test DNS resolution
curl http://cls-backend-dev.dev.example.com/health

# Disable external-dns when done
helm upgrade cls-backend ./deploy/helm-application \
  --set externalDns.enabled=false \
  --reuse-values
```

## Integration Examples

### With API Gateway

```yaml
externalDns:
  enabled: true
  hostname: "api.example.com"
  zone: "example.com"
  annotations:
    # Point to API Gateway instead of direct service
    external-dns.alpha.kubernetes.io/target: "gateway.example.com"
```

### With Load Balancer Health Checks

```yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  annotations:
    # AWS health check integration
    external-dns.alpha.kubernetes.io/aws-health-check-id: "hc-123456"
    external-dns.alpha.kubernetes.io/aws-health-check-type: "HTTP"
```

### With Multiple Environments

```yaml
# values-dev.yaml
externalDns:
  enabled: true
  hostname: "cls-backend-dev.dev.example.com"
  zone: "dev.example.com"
  ttl: 60

# values-staging.yaml
externalDns:
  enabled: true
  hostname: "cls-backend-staging.staging.example.com"
  zone: "staging.example.com"
  ttl: 300

# values-prod.yaml
externalDns:
  enabled: true
  hostname: "cls-backend.example.com"
  zone: "example.com"
  ttl: 300
  annotations:
    external-dns.alpha.kubernetes.io/aws-health-check-id: "prod-health-check"
```

This external-dns integration provides automatic DNS management for your CLS Backend service, enabling seamless access via friendly hostnames while maintaining flexibility for different environments and DNS providers.