package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/meyeringh/cf-switch/pkg/types"
)

// ErrEntrypointNotFound indicates that the requested entrypoint ruleset does not exist.
var ErrEntrypointNotFound = errors.New("entrypoint ruleset not found")

const (
	// HTTP timeout for Cloudflare API requests.
	defaultTimeout = 30 * time.Second
	// Maximum number of retry attempts for rate limited requests.
	maxRetries = 3
	// Maximum wait time for rate limit retry in seconds.
	maxRetryWaitSeconds = 60
)

// Client represents a Cloudflare API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiToken   string
	logger     *slog.Logger
}

// NewClient creates a new Cloudflare API client.
func NewClient(apiToken string, logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL:  "https://api.cloudflare.com/client/v4",
		apiToken: apiToken,
		logger:   logger,
	}
}

// GetEntrypointRuleset gets the entrypoint ruleset for the given zone and phase.
func (c *Client) GetEntrypointRuleset(ctx context.Context, zoneID, phase string) (*types.CloudflareRuleset, error) {
	url := fmt.Sprintf("%s/zones/%s/rulesets/phases/%s/entrypoint", c.baseURL, zoneID, phase)

	resp, err := c.makeRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get entrypoint ruleset: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrEntrypointNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp types.CloudflareAPIResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&apiResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %v", apiResp.Errors)
	}

	var ruleset types.CloudflareRuleset
	if resultBytes, marshalErr := json.Marshal(apiResp.Result); marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", marshalErr)
	} else if unmarshalErr := json.Unmarshal(resultBytes, &ruleset); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal ruleset: %w", unmarshalErr)
	}

	return &ruleset, nil
}

// CreateEntrypointRuleset creates a new entrypoint ruleset for the given zone and phase.
func (c *Client) CreateEntrypointRuleset(ctx context.Context, zoneID, phase string) (*types.CloudflareRuleset, error) {
	url := fmt.Sprintf("%s/zones/%s/rulesets", c.baseURL, zoneID)

	payload := map[string]interface{}{
		"kind":        "zone",
		"phase":       phase,
		"name":        fmt.Sprintf("%s entrypoint", phase),
		"description": fmt.Sprintf("Managed by cf-switch for %s phase", phase),
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, url, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create entrypoint ruleset: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp types.CloudflareAPIResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&apiResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %v", apiResp.Errors)
	}

	var ruleset types.CloudflareRuleset
	if resultBytes, marshalErr := json.Marshal(apiResp.Result); marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", marshalErr)
	} else if unmarshalErr := json.Unmarshal(resultBytes, &ruleset); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal ruleset: %w", unmarshalErr)
	}

	return &ruleset, nil
}

// AddRule adds a new rule to the given ruleset.
func (c *Client) AddRule(
	ctx context.Context,
	zoneID, rulesetID string,
	rule types.CloudflareRule,
) (*types.CloudflareRule, error) {
	url := fmt.Sprintf("%s/zones/%s/rulesets/%s/rules", c.baseURL, zoneID, rulesetID)

	resp, err := c.makeRequest(ctx, http.MethodPost, url, rule)
	if err != nil {
		return nil, fmt.Errorf("failed to add rule: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp types.CloudflareAPIResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&apiResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %v", apiResp.Errors)
	}

	var createdRule types.CloudflareRule
	if resultBytes, marshalErr := json.Marshal(apiResp.Result); marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", marshalErr)
	} else if unmarshalErr := json.Unmarshal(resultBytes, &createdRule); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal rule: %w", unmarshalErr)
	}

	return &createdRule, nil
}

// UpdateRule updates an existing rule.
func (c *Client) UpdateRule(
	ctx context.Context,
	zoneID, rulesetID, ruleID string,
	updates map[string]interface{},
) (*types.CloudflareRule, error) {
	url := fmt.Sprintf("%s/zones/%s/rulesets/%s/rules/%s", c.baseURL, zoneID, rulesetID, ruleID)

	resp, err := c.makeRequest(ctx, http.MethodPatch, url, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp types.CloudflareAPIResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&apiResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %v", apiResp.Errors)
	}

	var updatedRule types.CloudflareRule
	if resultBytes, marshalErr := json.Marshal(apiResp.Result); marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", marshalErr)
	} else if unmarshalErr := json.Unmarshal(resultBytes, &updatedRule); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal rule: %w", unmarshalErr)
	}

	return &updatedRule, nil
}

// FindRuleByDescription finds a rule in the ruleset by its description.
func FindRuleByDescription(ruleset *types.CloudflareRuleset, description string) *types.CloudflareRule {
	for i := range ruleset.Rules {
		if ruleset.Rules[i].Description == description {
			return &ruleset.Rules[i]
		}
	}
	return nil
}

// makeRequest makes an HTTP request to the Cloudflare API with retry logic.
//nolint:gocognit // Complex retry logic with rate limiting requires multiple conditions
func (c *Client) makeRequest(ctx context.Context, method, url string, payload interface{}) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	// Add request ID for logging.
	reqID := fmt.Sprintf("cf-%d", time.Now().UnixNano())
	req.Header.Set("X-Request-ID", reqID)

	start := time.Now()

	// Simple retry logic for rate limiting.
	var resp *http.Response
	for attempt := range maxRetries {
		resp, err = c.httpClient.Do(req)
		if err != nil {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, err)
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		// Handle rate limiting.
		//nolint:nestif // Rate limiting logic requires nested conditions for proper retry handling
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					if seconds <= maxRetryWaitSeconds { // Don't wait more than 60 seconds.
						if closeErr := resp.Body.Close(); closeErr != nil {
							c.logger.WarnContext(ctx, "Failed to close response body", "error", closeErr)
						}
						c.logger.WarnContext(ctx, "Rate limited, retrying",
							"attempt", attempt+1,
							"retry_after", seconds,
							"request_id", reqID)
						time.Sleep(time.Duration(seconds) * time.Second)
						continue
					}
				}
			}
			if closeErr := resp.Body.Close(); closeErr != nil {
				c.logger.WarnContext(ctx, "Failed to close response body", "error", closeErr)
			}
			return nil, errors.New("rate limited and retry would take too long")
		}

		break
	}

	duration := time.Since(start)
	c.logger.DebugContext(ctx, "Cloudflare API request completed",
		"method", method,
		"url", url,
		"status", resp.StatusCode,
		"duration_ms", duration.Milliseconds(),
		"request_id", reqID)

	return resp, nil
}
