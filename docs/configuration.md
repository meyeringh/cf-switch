# Configuration Guide

## Cloudflare API Token Setup

### Required Permissions

Your Cloudflare API token needs the following permissions:

1. **Zone:Zone:Read** - To read zone information
2. **Zone:Zone Settings:Edit** - To manage WAF Custom Rules

### Token Scope

- Include your specific zone(s) in the token scope
- Do not use a Global API Key (less secure)

### Creating the Token

1. Go to [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens)
2. Click "Create Token"
3. Use "Custom token" template
4. Set permissions:
   - `Zone:Zone:Read`
   - `Zone:Zone Settings:Edit`
5. Set zone resources to include your zone
6. Add client IP address restrictions if desired
7. Set TTL if desired
8. Create token and save it securely

## Environment Variables

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `DEST_HOSTNAMES` | Comma-separated list of hostnames | `"app.example.com,api.example.com"` |
| `CLOUDFLARE_ZONE_ID` | Your Cloudflare zone ID | `"abcdef1234567890abcdef1234567890"` |
| `CLOUDFLARE_API_TOKEN` | Cloudflare API token | Via Kubernetes secret |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CF_RULE_DEFAULT_ENABLED` | `false` | Default rule state |
| `HTTP_ADDR` | `:8080` | HTTP server address |
| `RECONCILE_INTERVAL` | `60s` | Reconciliation interval |

### Finding Your Zone ID

1. Log in to Cloudflare dashboard
2. Select your domain
3. Scroll down to "API" section in the right sidebar
4. Copy the Zone ID

## Kubernetes Configuration

### Creating the API Token Secret

```bash
kubectl create secret generic cloudflare-api-token \
  --from-literal=token=your-token-here \
  --namespace your-namespace
```

### Helm Values Configuration

```yaml
env:
  DEST_HOSTNAMES:
    value: "paperless.meyeringh.org,photos.example.com"
  CLOUDFLARE_ZONE_ID:
    value: "your-zone-id-here"
  CLOUDFLARE_API_TOKEN:
    valueFrom:
      secretKeyRef:
        name: cloudflare-api-token
        key: token
```

## Security Considerations

### RBAC Permissions

The service needs minimal RBAC permissions:

```yaml
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["cf-switch-auth"]
  verbs: ["get", "create", "patch", "update"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["create"]
```

### Container Security

The deployment uses security best practices:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 65532
  capabilities:
    drop: ["ALL"]
```

### Network Policies

Consider adding network policies to restrict egress:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cf-switch-egress
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: cf-switch
  policyTypes:
  - Egress
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS to Cloudflare API
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53   # DNS
```
