package cloudflare

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/meyeringh/cf-switch/pkg/types"
)

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := NewClient("test-token", logger)

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	if client.apiToken != "test-token" {
		t.Errorf("expected token %q, got %q", "test-token", client.apiToken)
	}

	if client.baseURL != "https://api.cloudflare.com/client/v4" {
		t.Errorf("expected baseURL %q, got %q", "https://api.cloudflare.com/client/v4", client.baseURL)
	}
}

func TestClient_GetEntrypointRuleset(t *testing.T) {
	tests := []struct {
		name           string
		response       string
		statusCode     int
		expectedError  bool
		expectedNil    bool
		expectedResult *types.CloudflareRuleset
	}{
		{
			name: "success",
			response: `{
				"success": true,
				"result": {
					"id": "test-ruleset-id",
					"name": "http_request_firewall_managed entrypoint",
					"kind": "zone",
					"phase": "http_request_firewall_managed",
					"rules": []
				}
			}`,
			statusCode: http.StatusOK,
			expectedResult: &types.CloudflareRuleset{
				ID:    "test-ruleset-id",
				Name:  "http_request_firewall_managed entrypoint",
				Kind:  "zone",
				Phase: "http_request_firewall_managed",
				Rules: []types.CloudflareRule{},
			},
		},
		{
			name:        "not found",
			response:    ``,
			statusCode:  http.StatusNotFound,
			expectedNil: true,
		},
		{
			name: "API error",
			response: `{
				"success": false,
				"errors": [{"code": 10000, "message": "Authentication error"}]
			}`,
			statusCode:    http.StatusUnauthorized,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := "/zones/test-zone/rulesets/phases/http_request_firewall_managed/entrypoint"
				if r.URL.Path != expectedPath {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if tt.response != "" {
					w.Write([]byte(tt.response))
				}
			}))
			defer server.Close()

			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelError,
			}))

			client := NewClient("test-token", logger)
			client.baseURL = server.URL

			result, err := client.GetEntrypointRuleset(context.Background(), "test-zone", "http_request_firewall_managed")

			if tt.expectedError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.expectedNil {
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				return
			}

			if result.ID != tt.expectedResult.ID {
				t.Errorf("expected ID %q, got %q", tt.expectedResult.ID, result.ID)
			}
			if result.Name != tt.expectedResult.Name {
				t.Errorf("expected Name %q, got %q", tt.expectedResult.Name, result.Name)
			}
		})
	}
}

func TestClient_CreateEntrypointRuleset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones/test-zone/rulesets" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify request body structure.
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req["phase"] != "http_request_firewall_managed" {
			t.Errorf("unexpected phase: %v", req["phase"])
		}

		response := `{
			"success": true,
			"result": {
				"id": "new-ruleset-id",
				"name": "http_request_firewall_managed entrypoint",
				"kind": "zone",
				"phase": "http_request_firewall_managed"
			}
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := NewClient("test-token", logger)
	client.baseURL = server.URL

	result, err := client.CreateEntrypointRuleset(context.Background(), "test-zone", "http_request_firewall_managed")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.ID != "new-ruleset-id" {
		t.Errorf("expected ID %q, got %q", "new-ruleset-id", result.ID)
	}
}

func TestClient_AddRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/zones/test-zone/rulesets/test-ruleset/rules"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify request body.
		var req types.CloudflareRule
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Expression != `http.host in {"test.com"}` {
			t.Errorf("unexpected expression: %s", req.Expression)
		}

		response := `{
			"success": true,
			"result": {
				"id": "new-rule-id",
				"expression": "http.host in {\"test.com\"}",
				"action": "block",
				"enabled": true,
				"description": "cf-switch:global"
			}
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := NewClient("test-token", logger)
	client.baseURL = server.URL

	rule := types.CloudflareRule{
		Expression:  `http.host in {"test.com"}`,
		Action:      "block",
		Enabled:     true,
		Description: "cf-switch:global",
	}

	result, err := client.AddRule(context.Background(), "test-zone", "test-ruleset", rule)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.ID != "new-rule-id" {
		t.Errorf("expected ID %q, got %q", "new-rule-id", result.ID)
	}
	if result.Expression != `http.host in {"test.com"}` {
		t.Errorf("expected expression %q, got %q", `http.host in {"test.com"}`, result.Expression)
	}
}

func TestClient_UpdateRule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/zones/test-zone/rulesets/test-ruleset/rules/test-rule"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}

		// Verify request body.
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if enabled, ok := req["enabled"].(bool); !ok || enabled != false {
			t.Errorf("expected enabled to be false, got %v", req["enabled"])
		}

		response := `{
			"success": true,
			"result": {
				"id": "test-rule",
				"expression": "http.host in {\"test.com\"}",
				"action": "block",
				"enabled": false,
				"description": "cf-switch:global"
			}
		}`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := NewClient("test-token", logger)
	client.baseURL = server.URL

	updates := map[string]interface{}{
		"enabled": false,
	}

	result, err := client.UpdateRule(context.Background(), "test-zone", "test-ruleset", "test-rule", updates)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.ID != "test-rule" {
		t.Errorf("expected ID %q, got %q", "test-rule", result.ID)
	}
	if result.Enabled != false {
		t.Errorf("expected enabled to be false, got %v", result.Enabled)
	}
}

func TestFindRuleByDescription(t *testing.T) {
	ruleset := &types.CloudflareRuleset{
		Rules: []types.CloudflareRule{
			{
				ID:          "rule1",
				Description: "other rule",
			},
			{
				ID:          "rule2",
				Description: "cf-switch:global",
			},
			{
				ID:          "rule3",
				Description: "another rule",
			},
		},
	}

	tests := []struct {
		name        string
		description string
		expectedID  string
		expectNil   bool
	}{
		{
			name:        "found",
			description: "cf-switch:global",
			expectedID:  "rule2",
		},
		{
			name:        "not found",
			description: "non-existent",
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindRuleByDescription(ruleset, tt.description)

			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Error("expected rule, got nil")
				return
			}

			if result.ID != tt.expectedID {
				t.Errorf("expected ID %q, got %q", tt.expectedID, result.ID)
			}
		})
	}
}
