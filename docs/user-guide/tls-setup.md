# TLS/HTTPS Setup Guide

This guide explains how to configure TLS/HTTPS support for the CLS Backend using GCP Managed Certificates with GKE Ingress.

## Overview

The CLS Backend supports automatic TLS certificate provisioning using GCP's ManagedCertificate resource with GKE Ingress. This provides:

- **Automatic certificate provisioning** - No manual certificate management
- **Automatic renewal** - Certificates are renewed automatically before expiration
- **Free SSL certificates** - No additional cost for certificates
- **Integration with GKE Ingress** - Seamless HTTPS termination at the Load Balancer

## Prerequisites

1. **Domain name** - You need a domain name for your API
2. **External DNS** - Recommended for automatic DNS record creation
3. **GKE cluster** - Must be running on Google Kubernetes Engine
4. **Patience** - Load Balancer takes 5-10 minutes, certificate takes 10-20 minutes after DNS is configured

## Configuration Steps

### Step 1: Enable Ingress with ManagedCertificate

Configure the Ingress with your domain and enable ManagedCertificate:

```yaml
# values.yaml or custom-values.yaml
ingress:
  enabled: true
  className: "gce"  # Required for GKE
  hosts:
    - host: "api.yourdomain.com"  # Your domain name
      paths:
        - path: /
          pathType: Prefix

  # GCP Managed Certificate configuration
  managedCertificate:
    enabled: true

# External DNS configuration (recommended for automatic DNS)
externalDns:
  enabled: true
  hostname: "api.yourdomain.com"  # Must match ingress host
  ttl: 300
```

**Important Notes:**
- The `className: "gce"` is **required** for GKE Ingress
- The hostname in `externalDns` must match the host in `ingress.hosts`
- Do NOT add a `tls:` section to the ingress when using ManagedCertificates

### Step 2: Deploy or Upgrade

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

### Step 3: Wait for Load Balancer IP

First, the GKE Ingress controller creates a Load Balancer. This takes **5-10 minutes**:

```bash
# Watch for IP address to appear
watch kubectl get ingress -n cls-system
```

Expected progression:
```
NAME            CLASS   HOSTS                    ADDRESS   PORTS     AGE
cls-backend     gce     api.yourdomain.com                 80, 443   1m   # Creating...
cls-backend     gce     api.yourdomain.com       34.x.x.x  80, 443   5m   # IP assigned!
```

### Step 4: Wait for Certificate Provisioning

After the Ingress has an IP and DNS is configured, GCP provisions the certificate. This takes **10-20 minutes**.

Check certificate status:

```bash
# Check ManagedCertificate status
kubectl get managedcertificate -n cls-system

# Get detailed certificate status
kubectl describe managedcertificate <name>-cert -n cls-system

# Check Ingress events for errors
kubectl describe ingress <name> -n cls-system
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

1. External DNS controller watches the Ingress resource
2. When an external IP is assigned to the Ingress, External DNS creates an A record
3. The A record points your hostname to the Ingress Load Balancer IP

**No manual DNS configuration required!**

### Option 2: Manual DNS

If not using External DNS, manually create a DNS A record:

1. Get the Ingress external IP:
   ```bash
   kubectl get ingress <name> -n cls-system
   ```

2. Create an A record in your DNS provider:
   - **Type**: A
   - **Name**: api (or your subdomain)
   - **Value**: Ingress ADDRESS (e.g., 34.117.197.222)
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
1. Ingress doesn't have an IP yet (Load Balancer still creating)
2. DNS not configured correctly
3. DNS propagation not complete (wait 5-10 minutes)
4. Domain doesn't point to the Ingress IP

**How to fix:**
```bash
# Check Ingress has an IP
kubectl get ingress -n cls-system

# Verify DNS resolution
nslookup api.yourdomain.com

# Verify DNS points to Ingress IP
dig api.yourdomain.com
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

Here's a complete example for enabling TLS with Ingress:

```yaml
# custom-values.yaml

# GCP Configuration
gcp:
  project: "my-gcp-project"
  region: "us-central1"

# Ingress Configuration
ingress:
  enabled: true
  className: "gce"  # Required for GKE
  hosts:
    - host: "api.mycompany.com"
      paths:
        - path: /
          pathType: Prefix

  # GCP Managed Certificate
  managedCertificate:
    enabled: true

# External DNS Configuration
externalDns:
  enabled: true
  hostname: "api.mycompany.com"  # Must match ingress host
  zone: "mycompany.com"
  ttl: 300

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

## Ingress Ports

When ManagedCertificate is enabled, the Ingress exposes:

| Port | Protocol | Purpose |
|------|----------|---------|
| 80   | HTTP     | HTTP traffic (can redirect to HTTPS) |
| 443  | HTTPS    | HTTPS traffic (TLS terminated at Load Balancer) |

**Note:**
- TLS termination happens at the GCP Load Balancer (not in the cluster)
- Traffic from Load Balancer to backend pods is unencrypted HTTP on port 8080
- The backend service is ClusterIP type, only accessible within the cluster

## Troubleshooting

### Ingress has no IP address

**Symptoms:** Ingress ADDRESS column is empty after 10+ minutes

**Common causes:**
1. Missing legacy annotation `kubernetes.io/ingress.class: gce`
2. Incorrect `ingressClassName` value
3. Empty secret reference error

**Solution:**
```bash
# Check Ingress configuration
kubectl get ingress <name> -n cls-system -o yaml

# Look for finalizer (proves GKE controller recognized it)
# Should see: networking.gke.io/ingress-finalizer-V2

# Check for errors in events
kubectl describe ingress <name> -n cls-system

# Common error: "secret \"\" does not exist"
# Fix: Ensure NO tls: section when using ManagedCertificates
```

### Certificate stuck in "Provisioning" status

**Wait at least 20 minutes** - Certificate provisioning is not instant.

If still stuck after 20 minutes:
```bash
# First, verify Ingress has an IP
kubectl get ingress -n cls-system

# Check ManagedCertificate events
kubectl describe managedcertificate <name>-cert -n cls-system

# Verify DNS is configured correctly
nslookup api.yourdomain.com

# Verify DNS points to Ingress IP
dig api.yourdomain.com
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

To add multiple domains, list them in the Ingress hosts and the ManagedCertificate will include all of them:

```yaml
ingress:
  enabled: true
  className: "gce"
  hosts:
    - host: "api.example.com"
      paths:
        - path: /
          pathType: Prefix
    - host: "api-v2.example.com"
      paths:
        - path: /
          pathType: Prefix
  managedCertificate:
    enabled: true
```

**Note:** All domains must point to the same Ingress IP.

### Common GKE Ingress Requirements

For GKE Ingress with ManagedCertificates to work, you need:

1. ✅ **Both annotation AND field**:
   ```yaml
   annotations:
     kubernetes.io/ingress.class: gce
   spec:
     ingressClassName: gce
   ```

2. ✅ **ManagedCertificate annotation**:
   ```yaml
   annotations:
     networking.gke.io/managed-certificates: <name>-cert
   ```

3. ❌ **NO tls: section** when using ManagedCertificates:
   ```yaml
   spec:
     # Don't add this when using ManagedCertificates:
     # tls: []  # This causes errors!
   ```

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

**Last Updated:** 2025-12-10
**Architecture:** GKE Ingress with ManagedCertificates (not LoadBalancer service)
