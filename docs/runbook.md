# Operations Runbook

## Service Overview

CF-Switch manages a single Cloudflare WAF Custom Rule that blocks traffic to configured hostnames. The service provides an HTTP API for toggling the rule and updating hostname lists.

## Pre-deployment Checklist

- [ ] Cloudflare API token created with correct permissions
- [ ] Zone ID identified and confirmed
- [ ] Kubernetes cluster access configured
- [ ] Helm installed and configured
- [ ] Target hostnames identified

## Deployment

### Initial Installation

```bash
# Create API token secret
kubectl create secret generic cloudflare-api-token \
  --from-literal=token=your-token-here

# Install with Helm
helm install cf-switch deploy/helm/cf-switch \
  --set env.DEST_HOSTNAMES.value="host1.example.com,host2.example.com" \
  --set env.CLOUDFLARE_ZONE_ID.value="your-zone-id"
```

### Upgrade Deployment

```bash
# Update hostname list
helm upgrade cf-switch deploy/helm/cf-switch \
  --set env.DEST_HOSTNAMES.value="new-hosts.example.com" \
  --reuse-values

# Update image version
helm upgrade cf-switch deploy/helm/cf-switch \
  --set image.tag=v0.2.0 \
  --reuse-values
```

## Common Operations

### Get API Token

```bash
kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d; echo
```

### Check Service Status

```bash
# Deployment status
kubectl get deployment cf-switch
kubectl describe deployment cf-switch

# Pod status
kubectl get pods -l app.kubernetes.io/name=cf-switch
kubectl logs -l app.kubernetes.io/name=cf-switch --tail=50
```

### Test API Connectivity

```bash
# Port forward for testing
kubectl port-forward svc/cf-switch 8080:8080 &

# Health check
curl http://localhost:8080/healthz

# Get rule status
TOKEN=$(kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/rule
```

### Enable/Disable Traffic Blocking

```bash
TOKEN=$(kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d)

# Enable blocking (deny traffic)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true}' \
  http://localhost:8080/v1/rule/enable

# Disable blocking (allow traffic)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":false}' \
  http://localhost:8080/v1/rule/enable
```

### Update Hostname List

```bash
TOKEN=$(kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d)

curl -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"hostnames":["new-host.example.com","another-host.example.com"]}' \
  http://localhost:8080/v1/rule/hosts
```

## Monitoring and Alerts

### Key Metrics

Monitor these Prometheus metrics:

- `cf_switch_rule_enabled` - Current rule state (0/1)
- `cf_switch_toggles_total` - Number of rule toggles
- `cf_switch_api_requests_total` - API request volume
- `cf_switch_cloudflare_api_duration_seconds` - Cloudflare API latency

### Recommended Alerts

```yaml
# Rule unexpectedly disabled
- alert: CFSwitchRuleDisabled
  expr: cf_switch_rule_enabled == 0
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "CF-Switch rule is disabled"

# High API error rate
- alert: CFSwitchHighErrorRate
  expr: rate(cf_switch_api_requests_total{status=~"5.."}[5m]) > 0.1
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "CF-Switch API error rate is high"

# Service unavailable
- alert: CFSwitchDown
  expr: up{job="cf-switch"} == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "CF-Switch service is down"
```

### Log Analysis

Key log patterns to monitor:

```bash
# Successful operations
kubectl logs -l app.kubernetes.io/name=cf-switch | grep "INFO"

# Errors and warnings
kubectl logs -l app.kubernetes.io/name=cf-switch | grep -E "(ERROR|WARN)"

# API requests
kubectl logs -l app.kubernetes.io/name=cf-switch | grep "HTTP request completed"

# Cloudflare API calls
kubectl logs -l app.kubernetes.io/name=cf-switch | grep "Cloudflare API"
```

## Troubleshooting

### Service Won't Start

**Symptoms:** Pod in CrashLoopBackOff or Error state

**Diagnosis:**
```bash
kubectl describe pod -l app.kubernetes.io/name=cf-switch
kubectl logs -l app.kubernetes.io/name=cf-switch
```

**Common Causes:**
1. Missing environment variables
2. Invalid Cloudflare API token
3. Network connectivity issues
4. RBAC permissions missing

### API Returns 401 Unauthorized

**Diagnosis:**
```bash
# Check if secret exists
kubectl get secret cf-switch-auth

# Verify token
kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d
```

**Solutions:**
1. Recreate the service to regenerate token
2. Check RBAC permissions for secret management

### Cloudflare API Errors

**Symptoms:** 403/429 errors from Cloudflare

**Diagnosis:**
```bash
kubectl logs -l app.kubernetes.io/name=cf-switch | grep "Cloudflare API"
```

**Solutions:**
1. Verify API token permissions in Cloudflare dashboard
2. Check rate limiting (429 errors)
3. Verify zone ID is correct

### Rule Not Found/Created

**Symptoms:** Service logs show rule creation attempts

**Diagnosis:**
```bash
# Check reconciliation logs
kubectl logs -l app.kubernetes.io/name=cf-switch | grep -E "(reconcil|rule)"
```

**Solutions:**
1. Verify zone ID and API token permissions
2. Check if zone has WAF Custom Rules enabled
3. Manually verify rule in Cloudflare dashboard

### High Memory/CPU Usage

**Diagnosis:**
```bash
kubectl top pods -l app.kubernetes.io/name=cf-switch
```

**Solutions:**
1. Increase resource requests/limits
2. Check for memory leaks in logs
3. Reduce reconciliation frequency

## Backup and Recovery

### Configuration Backup

```bash
# Export current Helm values
helm get values cf-switch > cf-switch-values-backup.yaml

# Export Kubernetes manifests
kubectl get deployment,service,secret,role,rolebinding \
  -l app.kubernetes.io/name=cf-switch -o yaml > cf-switch-backup.yaml
```

### Disaster Recovery

```bash
# Restore from backup
helm install cf-switch deploy/helm/cf-switch -f cf-switch-values-backup.yaml

# Or apply manifests directly
kubectl apply -f cf-switch-backup.yaml
```

### Cloudflare Rule Recovery

If the managed rule is accidentally deleted:

1. CF-Switch will automatically recreate it on next reconciliation
2. Manual recreation via Cloudflare dashboard:
   - Go to Security > WAF > Custom Rules
   - Create rule with description "cf-switch:global"
   - Set expression to match hostnames
   - CF-Switch will adopt the rule on next reconciliation

## Maintenance

### Regular Tasks

**Daily:**
- Check service health and logs
- Verify metrics are being collected

**Weekly:**
- Review Cloudflare rule in dashboard
- Check for service updates

**Monthly:**
- Review and update API token if needed
- Audit hostname list
- Review resource usage and scaling

### Updates

```bash
# Update to latest version
helm repo update
helm upgrade cf-switch cf-switch/cf-switch

# Or update image tag
helm upgrade cf-switch deploy/helm/cf-switch \
  --set image.tag=latest \
  --reuse-values
```

## Emergency Procedures

### Emergency Rule Disable

If you need to immediately disable the rule:

```bash
# Via API (fastest)
TOKEN=$(kubectl get secret cf-switch-auth -o jsonpath='{.data.apiToken}' | base64 -d)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":false}' \
  http://localhost:8080/v1/rule/enable

# Via Cloudflare dashboard
# 1. Go to Security > WAF > Custom Rules
# 2. Find rule with description "cf-switch:global"
# 3. Disable the rule
```

### Emergency Service Shutdown

```bash
# Scale down deployment
kubectl scale deployment cf-switch --replicas=0

# Or delete entirely
helm uninstall cf-switch
```

Note: The Cloudflare rule will remain in its last state when the service is shut down.
