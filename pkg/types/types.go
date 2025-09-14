//nolint:revive // Package name "types" is conventional for configuration and data types
package types

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	// Cloudflare configuration.
	CloudflareZoneID     string   `json:"cloudflare_zone_id"`
	CloudflareAPIToken   string   `json:"-"` // Never log this.
	DestHostnames        []string `json:"dest_hostnames"`
	CFRuleDefaultEnabled bool     `json:"cf_rule_default_enabled"`

	// Server configuration.
	HTTPAddr          string        `json:"http_addr"`
	ReconcileInterval time.Duration `json:"reconcile_interval"`

	// Kubernetes configuration (derived from environment).
	Namespace          string `json:"namespace"`
	ServiceAccountName string `json:"service_account_name"`
}

// Rule represents the Cloudflare WAF Custom Rule.
type Rule struct {
	ID          string   `json:"rule_id"`
	Enabled     bool     `json:"enabled"`
	Expression  string   `json:"expression"`
	Hostnames   []string `json:"hostnames"`
	Description string   `json:"description"`
	Version     int      `json:"version"`
}

// ToggleRequest represents the request to enable/disable the rule.
type ToggleRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateHostsRequest represents the request to update hostnames.
type UpdateHostsRequest struct {
	Hostnames []string `json:"hostnames"`
}

// RuleResponse represents the response for rule status.
type RuleResponse struct {
	RuleID      string   `json:"rule_id"`
	Enabled     bool     `json:"enabled"`
	Expression  string   `json:"expression"`
	Hostnames   []string `json:"hostnames"`
	Description string   `json:"description"`
	Version     int      `json:"version"`
}

// CloudflareRuleset represents a Cloudflare ruleset.
type CloudflareRuleset struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Kind        string           `json:"kind"`
	Phase       string           `json:"phase"`
	Rules       []CloudflareRule `json:"rules"`
}

// CloudflareRule represents a single Cloudflare rule.
type CloudflareRule struct {
	ID          string `json:"id"`
	Action      string `json:"action"`
	Expression  string `json:"expression"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Version     int    `json:"version,omitempty"`
}

// CloudflareAPIError represents an error response from Cloudflare API.
type CloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CloudflareAPIResponse represents a generic Cloudflare API response.
type CloudflareAPIResponse struct {
	Success bool                 `json:"success"`
	Errors  []CloudflareAPIError `json:"errors"`
	Result  interface{}          `json:"result"`
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	config := &Config{
		HTTPAddr:             getEnvOrDefault("HTTP_ADDR", ":8080"),
		CFRuleDefaultEnabled: getEnvBoolOrDefault("CF_RULE_DEFAULT_ENABLED", false),
		Namespace:            getEnvOrDefault("KUBERNETES_NAMESPACE", "default"),
		ServiceAccountName:   getEnvOrDefault("KUBERNETES_SERVICE_ACCOUNT", "cf-switch"),
	}

	// Parse required fields.
	config.CloudflareZoneID = os.Getenv("CLOUDFLARE_ZONE_ID")
	if config.CloudflareZoneID == "" {
		return nil, errors.New("CLOUDFLARE_ZONE_ID is required")
	}

	config.CloudflareAPIToken = os.Getenv("CLOUDFLARE_API_TOKEN")
	if config.CloudflareAPIToken == "" {
		return nil, errors.New("CLOUDFLARE_API_TOKEN is required")
	}

	destHostnamesStr := os.Getenv("DEST_HOSTNAMES")
	if destHostnamesStr == "" {
		return nil, errors.New("DEST_HOSTNAMES is required")
	}

	config.DestHostnames = ParseHostnames(destHostnamesStr)
	if len(config.DestHostnames) == 0 {
		return nil, errors.New("DEST_HOSTNAMES must contain at least one hostname")
	}

	// Parse reconcile interval.
	reconcileIntervalStr := getEnvOrDefault("RECONCILE_INTERVAL", "60s")
	interval, err := time.ParseDuration(reconcileIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RECONCILE_INTERVAL: %w", err)
	}
	config.ReconcileInterval = interval

	return config, nil
}

// ParseHostnames parses and normalizes a comma-separated list of hostnames.
func ParseHostnames(hostnames string) []string {
	if hostnames == "" {
		return nil
	}

	var result []string
	seen := make(map[string]bool)

	for _, hostname := range strings.Split(hostnames, ",") {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		if hostname != "" && !seen[hostname] {
			result = append(result, hostname)
			seen[hostname] = true
		}
	}

	// Sort for consistent ordering.
	sort.Strings(result)
	return result
}

// BuildExpression builds a Cloudflare expression for the given hostnames.
func BuildExpression(hostnames []string) string {
	if len(hostnames) == 0 {
		return "false"
	}

	// Build the expression: http.host in {"host1" "host2" ...}
	var quoted []string
	for _, hostname := range hostnames {
		quoted = append(quoted, fmt.Sprintf(`"%s"`, hostname))
	}

	return fmt.Sprintf(`http.host in {%s}`, strings.Join(quoted, " "))
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBoolOrDefault returns the environment variable as a boolean or a default.
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

const (
	// RuleDescription is the description used for the managed rule.
	RuleDescription = "cf-switch:global"

	// HTTPRequestFirewallCustomPhase is the phase for Cloudflare WAF Custom Rules.
	HTTPRequestFirewallCustomPhase = "http_request_firewall_custom"

	// BlockAction is the action for blocking requests.
	BlockAction = "block"
)
