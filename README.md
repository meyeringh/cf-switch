# CF-Switch

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

## Use the API

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