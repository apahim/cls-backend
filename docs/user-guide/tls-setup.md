# TLS/HTTPS Setup Guide

This guide explains how to configure TLS/HTTPS support for the CLS Backend using GCP Managed Certificates.

## Overview

The CLS Backend supports automatic TLS certificate provisioning using GCP's ManagedCertificate resource. This provides:

- **Automatic certificate provisioning** - No manual certificate management
- **Automatic renewal** - Certificates are renewed automatically before expiration
- **Free SSL certificates** - No additional cost for certificates
- **Integration with GKE LoadBalancer** - Seamless HTTPS termination

## Prerequisites

1. **Domain name configured** - You must have a domain name that points to your LoadBalancer
2. **External DNS** - Recommended to use External DNS for automatic DNS record creation
3. **GKE cluster** - Running on Google Kubernetes Engine
4. **Patience** - Certificate provisioning takes 10-20 minutes after initial deployment

## Configuration Steps

### Step 1: Configure DNS Hostname

First, set your DNS hostname in the values file:

```yaml
# values.yaml or custom-values.yaml
externalDns:
  enabled: true
  hostname: "api.yourdomain.com"  # Your domain name
  zone: "yourdomain.com"           # Your DNS zone (optional)
  ttl: 300
```

### Step 2: Enable TLS

Enable TLS and GCP ManagedCertificate:

```yaml
# values.yaml or custom-values.yaml
tls:
  enabled: true
  managedCertificate:
    enabled: true
    # Optional: Add additional domains to the certificate
    additionalDomains: []
      # - api.example.com
      # - backend.example.com
```

### Step 3: Deploy or Upgrade

Deploy the Helm chart with the new configuration:

```bash
# For new deployments
helm install cls-backend ./deploy/helm-application \
  --namespace cls-system \
  --values custom-values.yaml

# For upgrades
helm upgrade cls-backend ./deploy/helm-application \
  --namespace cls-system \
  --values custom-values.yaml
```

### Step 4: Wait for Certificate Provisioning

GCP will automatically provision the certificate. This process takes **10-20 minutes**.

Check certificate status:

```bash
# Check ManagedCertificate status
kubectl get managedcertificate -n cls-system

# Get detailed certificate status
kubectl describe managedcertificate cls-backend-application-cert -n cls-system
```

Expected output when ready:
```
Status:
  Certificate Status: Active
  Domain Status:
    Domain:  api.yourdomain.com
    Status:  Active
```

### Step 5: Verify HTTPS Access

Once the certificate is active, test HTTPS access:

```bash
# Test HTTPS endpoint
curl https://api.yourdomain.com/health

# Test with certificate details
curl -v https://api.yourdomain.com/api/v1/info
```

## DNS Configuration

### Option 1: Automatic DNS (Recommended)

If you're using External DNS, the DNS record will be created automatically:

1. External DNS controller watches the LoadBalancer service
2. When an external IP is assigned, External DNS creates an A record
3. The A record points your hostname to the LoadBalancer IP

**No manual DNS configuration required!**

### Option 2: Manual DNS

If not using External DNS, manually create a DNS A record:

1. Get the LoadBalancer external IP:
   ```bash
   kubectl get service cls-backend-application-external -n cls-system
   ```

2. Create an A record in your DNS provider:
   - **Type**: A
   - **Name**: api (or your subdomain)
   - **Value**: LoadBalancer external IP (e.g., 35.222.86.208)
   - **TTL**: 300 (or as desired)

## Certificate Status

### Status: Provisioning

During provisioning, you'll see:
```
Status:
  Certificate Status: Provisioning
  Domain Status:
    Domain:  api.yourdomain.com
    Status:  Provisioning
```

**What's happening:**
- GCP is validating domain ownership
- Certificate is being issued by Google's Certificate Authority
- This takes 10-20 minutes

### Status: FailedNotVisible

If you see this status:
```
Status:
  Domain Status:
    Domain:  api.yourdomain.com
    Status:  FailedNotVisible
```

**Common causes:**
1. DNS not configured correctly
2. DNS propagation not complete (wait 5-10 minutes)
3. Domain doesn't point to the LoadBalancer IP

**How to fix:**
```bash
# Verify DNS resolution
nslookup api.yourdomain.com

# Verify it points to LoadBalancer IP
kubectl get service cls-backend-application-external -n cls-system
```

### Status: Active

When ready:
```
Status:
  Certificate Status: Active
  Domain Status:
    Domain:  api.yourdomain.com
    Status:  Active
```

**Your HTTPS endpoint is now ready!**

## Complete Example Configuration

Here's a complete example for enabling TLS:

```yaml
# custom-values.yaml

# GCP Configuration
gcp:
  project: "my-gcp-project"
  region: "us-central1"

# External DNS Configuration
externalDns:
  enabled: true
  hostname: "api.mycompany.com"
  zone: "mycompany.com"
  ttl: 300

# TLS Configuration
tls:
  enabled: true
  managedCertificate:
    enabled: true
    # Additional domains (optional)
    additionalDomains:
      - api-v2.mycompany.com

# Other configurations...
```

Deploy:
```bash
helm upgrade --install cls-backend ./deploy/helm-application \
  --namespace cls-system \
  --create-namespace \
  --values custom-values.yaml \
  --wait
```

## Service Ports

When TLS is enabled, the LoadBalancer service exposes:

| Port | Protocol | Target Port | Purpose |
|------|----------|-------------|---------|
| 80   | HTTP     | 8080        | HTTP traffic (can be used for redirects) |
| 443  | HTTPS    | 8080        | HTTPS traffic (TLS terminated at LoadBalancer) |

**Note:** TLS termination happens at the GCP LoadBalancer level, so the backend still receives HTTP traffic on port 8080.

## Troubleshooting

### Certificate stuck in "Provisioning" status

**Wait at least 20 minutes** - Certificate provisioning is not instant.

If still stuck after 20 minutes:
```bash
# Check ManagedCertificate events
kubectl describe managedcertificate cls-backend-application-cert -n cls-system

# Verify DNS is configured correctly
nslookup api.yourdomain.com

# Check LoadBalancer status
kubectl describe service cls-backend-application-external -n cls-system
```

### "FailedNotVisible" error

**Cause:** GCP cannot verify domain ownership (usually DNS issue)

**Solution:**
1. Verify DNS record exists and points to LoadBalancer IP
2. Wait 5-10 minutes for DNS propagation
3. Check with multiple DNS servers:
   ```bash
   nslookup api.yourdomain.com 8.8.8.8
   dig api.yourdomain.com
   ```

### Multiple domains on one certificate

Add additional domains to the certificate:

```yaml
tls:
  enabled: true
  managedCertificate:
    enabled: true
    additionalDomains:
      - api.example.com
      - backend.example.com
      - api-v2.example.com
```

**Note:** All domains must point to the same LoadBalancer IP.

### Disable HTTP port 80

If you only want HTTPS traffic, you can modify the service template to remove port 80, but this is **not recommended** as it breaks health checks and makes initial setup harder.

## Security Considerations

1. **TLS 1.2+** - GCP ManagedCertificates use TLS 1.2 and 1.3 by default
2. **Strong Ciphers** - Google uses strong cipher suites by default
3. **Automatic Renewal** - Certificates are renewed automatically before expiration
4. **No Private Key Management** - Google manages private keys securely

## Additional Resources

- [GCP ManagedCertificate Documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/managed-certs)
- [External DNS Documentation](https://github.com/kubernetes-sigs/external-dns)
- [GKE LoadBalancer Documentation](https://cloud.google.com/kubernetes-engine/docs/concepts/service-load-balancer)

## Related Documentation

- [User Guide Examples](./examples.md) - Example API calls using HTTPS
- [README](../README.md) - Main documentation index

---

**Last Updated:** 2025-12-09
