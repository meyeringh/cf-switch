# CF-Switch

[![CI/CD Pipeline](https://github.com/meyeringh/cf-switch/actions/workflows/ci.yaml/badge.svg)](https://github.com/meyeringh/cf-switch/actions/workflows/ci.yaml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

A Kubernetes service for managing Cloudflare WAF Custom Rules. CF-Switch manages a **single Cloudflare rule** that blocks traffic to a configurable set of hostnames, providing an API to toggle the rule on/off and update the hostname list.

## What This Does

CF-Switch creates and manages exactly **one Cloudflare WAF Custom Rule** in your zone's `http_request_firewall_custom` entry point ruleset. The rule:

- **Action**: `block` (returns 403 Forbidden)
- **Expression**: `http.host in {"host1.example.com" "host2.example.com" ...}` 
- **Description**: `cf-switch:global` (used to identify the managed rule)
- **Enabled**: Configurable (default: `false`)

The service provides an HTTP API to:
- Toggle the rule on/off (enable/disable blocking)
- Update the list of hostnames the rule applies to
- Get current rule status

## Prerequisites

1. **Cloudflare Zone**: You need a Cloudflare zone with an active domain
2. **Cloudflare API Token**: Create a token with the following permissions:
   - `Zone:Zone:Read` (to read zone information)
   - `Zone:Zone Settings:Edit` (to manage WAF Custom Rules)
   - Include your specific zone(s) in the token scope
3. **Kubernetes Cluster**: CF-Switch runs as a Kubernetes deployment

## Quick Start

### 1. Create Cloudflare API Token Secret

```bash
kubectl create secret generic cloudflare-api-token \
  --from-literal=token=your-cloudflare-api-token-here
```

### 2. Install with Helm

```bash
# Add the Helm repository (if available)
# helm repo add cf-switch https://meyeringh.github.io/cf-switch

# Or install directly from source
git clone https://github.com/meyeringh/cf-switch.git
cd cf-switch

helm install cf-switch deploy/helm/cf-switch \
  --set env.DEST_HOSTNAMES.value="paperless.meyeringh.org,photos.example.com" \
  --set env.CLOUDFLARE_ZONE_ID.value="your-zone-id-here" \
  --set env.CLOUDFLARE_API_TOKEN.valueFrom.secretKeyRef.name="cloudflare-api-token" \
  --set env.CLOUDFLARE_API_TOKEN.valueFrom.secretKeyRef.key="token"
```

### 3. Get the API Token

```bash
# Retrieve the auto-generated API token for CF-Switch API access
kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d; echo
```

### 4. Use the API

```bash
TOKEN=$(kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d)

# Get current rule status
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/rule

# Enable blocking (rule becomes active)
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"enabled":true}' http://localhost:8080/v1/rule/enable

# Disable blocking (rule becomes inactive)
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"enabled":false}' http://localhost:8080/v1/rule/enable

# Update hostnames
curl -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"hostnames":["paperless.meyeringh.org","photos.example.com","api.example.org"]}' \
  http://localhost:8080/v1/rule/hosts
```

## Configuration

All configuration is via environment variables, exposed through Helm values:

| Environment Variable | Required | Default | Description |
|---------------------|----------|---------|-------------|
| `DEST_HOSTNAMES` | ✅ | - | Comma-separated list of hostnames to apply the rule to |
| `CLOUDFLARE_ZONE_ID` | ✅ | - | Your Cloudflare zone ID |
| `CLOUDFLARE_API_TOKEN` | ✅ | - | Cloudflare API token (via secret) |
| `CF_RULE_DEFAULT_ENABLED` | ❌ | `false` | Whether the rule should be enabled by default |
| `HTTP_ADDR` | ❌ | `:8080` | HTTP server listen address |
| `RECONCILE_INTERVAL` | ❌ | `60s` | How often to reconcile rule state |

### Helm Values Example

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
  CF_RULE_DEFAULT_ENABLED:
    value: "false"
```

## API Reference

### Authentication

All `/v1/*` endpoints require Bearer token authentication:

```bash
Authorization: Bearer <token>
```

The token is automatically generated and stored in the `cf-switch-auth` Kubernetes secret.

### Endpoints

#### GET /v1/rule
Get current rule status.

**Response:**
```json
{
  "rule_id": "12345678-1234-1234-1234-123456789abc",
  "enabled": true,
  "expression": "http.host in {\"paperless.meyeringh.org\" \"photos.example.com\"}",
  "hostnames": ["paperless.meyeringh.org", "photos.example.com"],
  "description": "cf-switch:global",
  "version": 2
}
```

#### POST /v1/rule/enable
Enable or disable the rule.

**Request:**
```json
{
  "enabled": true
}
```

#### PUT /v1/rule/hosts
Update the list of hostnames.

**Request:**
```json
{
  "hostnames": ["paperless.meyeringh.org", "photos.example.com", "api.example.org"]
}
```

#### Health Endpoints (no auth required)
- `GET /healthz` - Health check
- `GET /readyz` - Readiness check  
- `GET /metrics` - Prometheus metrics

### OpenAPI Specification

The complete API specification is available at [`api/openapi.yaml`](./api/openapi.yaml).

## Operations

### Adding/Removing Hosts

**Option 1: Update via API**
```bash
curl -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"hostnames":["new-host.example.com","existing-host.example.com"]}' \
  http://localhost:8080/v1/rule/hosts
```

**Option 2: Update Helm values and upgrade**
```bash
helm upgrade cf-switch deploy/helm/cf-switch \
  --set env.DEST_HOSTNAMES.value="new-host.example.com,existing-host.example.com"
```

### Enabling/Disabling the Rule

```bash
# Block traffic to all configured hosts
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"enabled":true}' http://localhost:8080/v1/rule/enable

# Allow traffic to all configured hosts  
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"enabled":false}' http://localhost:8080/v1/rule/enable
```

### Monitoring

**Prometheus Metrics:**
- `cf_switch_toggles_total{enabled}` - Counter of rule toggles
- `cf_switch_api_requests_total{method,path,status}` - Counter of API requests
- `cf_switch_rule_enabled` - Gauge showing current rule state (1=enabled, 0=disabled)
- `cf_switch_cloudflare_api_duration_seconds{method,endpoint}` - Histogram of Cloudflare API call durations

**Logs:**
```bash
kubectl logs -l app.kubernetes.io/name=cf-switch -f
```

**ServiceMonitor (Prometheus Operator):**
```yaml
serviceMonitor:
  enabled: true
  interval: 30s
```

### Troubleshooting

**Check deployment status:**
```bash
kubectl get deployment cf-switch
kubectl describe deployment cf-switch
```

**View recent logs:**
```bash
kubectl logs -l app.kubernetes.io/name=cf-switch --tail=100
```

**Verify RBAC permissions:**
```bash
kubectl get role,rolebinding
kubectl auth can-i create secrets --as=system:serviceaccount:default:cf-switch
```

**Test Cloudflare connectivity:**
```bash
# Port-forward and check health
kubectl port-forward svc/cf-switch 8080:8080
curl http://localhost:8080/healthz
```

**Common Issues:**

1. **"Failed to ensure authentication secret"**: RBAC permissions missing
2. **"Failed to communicate with Cloudflare API"**: Check API token permissions
3. **"Rule not found"**: Service will automatically create the rule on startup
4. **API returns 401**: Check the Bearer token from `cf-switch-auth` secret

## Development

### Building Locally

```bash
# Build binary
make build

# Run tests
make test

# Build Docker image
make docker-build

# Lint code
make lint
```

### Running Locally

```bash
# Set environment variables
export DEST_HOSTNAMES="test.example.com"
export CLOUDFLARE_ZONE_ID="your-zone-id"
export CLOUDFLARE_API_TOKEN="your-api-token"

# Build and run
make dev-run
```

### Project Structure

```
├── cmd/cf-switch/           # Main application entry point
├── internal/
│   ├── cloudflare/          # Cloudflare API client
│   ├── kube/                # Kubernetes secret management
│   ├── reconcile/           # Rule reconciliation logic  
│   └── server/              # HTTP server and handlers
├── pkg/types/               # Shared types and configuration
├── api/openapi.yaml         # OpenAPI specification
├── deploy/helm/cf-switch/   # Helm chart
├── docs/                    # Additional documentation
└── .github/workflows/       # CI/CD pipeline
```

## Security

- **Container Security**: Runs as non-root user (65532) with read-only root filesystem
- **RBAC**: Minimal permissions (only secret management in the release namespace)
- **API Authentication**: Mandatory Bearer token for all management endpoints
- **Network Policies**: Consider adding network policies to restrict traffic
- **Secrets**: API tokens managed via Kubernetes secrets with proper RBAC

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/meyeringh/cf-switch/issues)
- **Documentation**: [docs/](./docs/)
- **API Spec**: [api/openapi.yaml](./api/openapi.yaml)
