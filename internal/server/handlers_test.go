//nolint:testpackage // Using same package as implementation to test unexported functions
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/meyeringh/cf-switch/pkg/types"
)

// MockReconciler implements RuleReconciler for testing.
type MockReconciler struct {
	rule          *types.Rule
	toggleErr     error
	updateErr     error
	getCurrentErr error
}

func (m *MockReconciler) GetCurrentRule(_ context.Context) (*types.Rule, error) {
	if m.getCurrentErr != nil {
		return nil, m.getCurrentErr
	}
	if m.rule == nil {
		return &types.Rule{
			ID:          "test-rule-id",
			Enabled:     false,
			Expression:  `http.host in {"test.com"}`,
			Hostnames:   []string{"test.com"},
			Description: "cf-switch:global",
			Version:     1,
		}, nil
	}
	return m.rule, nil
}

func (m *MockReconciler) ToggleRule(ctx context.Context, enabled bool) (*types.Rule, error) {
	if m.toggleErr != nil {
		return nil, m.toggleErr
	}
	rule, _ := m.GetCurrentRule(ctx)
	rule.Enabled = enabled
	rule.Version++
	m.rule = rule
	return rule, nil
}

func (m *MockReconciler) UpdateHosts(ctx context.Context, hostnames []string) (*types.Rule, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	rule, _ := m.GetCurrentRule(ctx)
	rule.Hostnames = hostnames
	rule.Expression = types.BuildExpression(hostnames)
	rule.Version++
	m.rule = rule
	return rule, nil
}

//nolint:gocognit // Comprehensive authentication middleware test covering multiple scenarios
func TestAuthMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Suppress logs during tests.
	}))

	token := "test-token"
	middleware := NewAuthMiddleware(token, logger)

	// Create a test handler.
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authorized"))
	})

	handler := middleware.Middleware(testHandler)

	tests := []struct {
		name           string
		path           string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "health endpoint no auth required",
			path:           "/healthz",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectedBody:   "authorized",
		},
		{
			name:           "readiness endpoint no auth required",
			path:           "/readyz",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectedBody:   "authorized",
		},
		{
			name:           "metrics endpoint no auth required",
			path:           "/metrics",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectedBody:   "authorized",
		},
		{
			name:           "missing auth header",
			path:           "/v1/rule",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Missing Authorization header",
		},
		{
			name:           "invalid auth header format",
			path:           "/v1/rule",
			authHeader:     "Basic dGVzdA==",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid Authorization header format",
		},
		{
			name:           "invalid token",
			path:           "/v1/rule",
			authHeader:     "Bearer wrong-token",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid token",
		},
		{
			name:           "valid token",
			path:           "/v1/rule",
			authHeader:     "Bearer test-token",
			expectedStatus: http.StatusOK,
			expectedBody:   "authorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				if rr.Body.String() != tt.expectedBody {
					t.Errorf("expected body %q, got %q", tt.expectedBody, rr.Body.String())
				}
			} else {
				// Check error response structure.
				var errorResp ErrorResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
					t.Fatalf("failed to unmarshal error response: %v", err)
				}
				if errorResp.Message != tt.expectedBody {
					t.Errorf("expected error message %q, got %q", tt.expectedBody, errorResp.Message)
				}
			}
		})
	}
}

func TestRuleHandler_GetRule(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	t.Run("success", func(t *testing.T) {
		reconciler := &MockReconciler{}
		handler := NewRuleHandler(reconciler, logger)

		req := httptest.NewRequest(http.MethodGet, "/v1/rule", nil)
		rr := httptest.NewRecorder()

		handler.GetRule(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response types.RuleResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if response.RuleID != "test-rule-id" {
			t.Errorf("expected rule ID %q, got %q", "test-rule-id", response.RuleID)
		}
	})

	t.Run("reconciler error", func(t *testing.T) {
		reconciler := &MockReconciler{getCurrentErr: &MockError{"test error"}}
		handler := NewRuleHandler(reconciler, logger)

		req := httptest.NewRequest(http.MethodGet, "/v1/rule", nil)
		rr := httptest.NewRecorder()

		handler.GetRule(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestRuleHandler_ToggleRule(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	t.Run("success", func(t *testing.T) {
		reconciler := &MockReconciler{}
		handler := NewRuleHandler(reconciler, logger)

		reqBody := types.ToggleRequest{Enabled: true}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/v1/rule/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ToggleRule(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response types.RuleResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if !response.Enabled {
			t.Error("expected rule to be enabled")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		reconciler := &MockReconciler{}
		handler := NewRuleHandler(reconciler, logger)

		req := httptest.NewRequest(http.MethodPost, "/v1/rule/enable", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ToggleRule(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

func TestRuleHandler_UpdateHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	t.Run("success", func(t *testing.T) {
		reconciler := &MockReconciler{}
		handler := NewRuleHandler(reconciler, logger)

		reqBody := types.UpdateHostsRequest{
			Hostnames: []string{"new-host.com", "another.com"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/v1/rule/hosts", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.UpdateHosts(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response types.RuleResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(response.Hostnames) != 2 {
			t.Errorf("expected 2 hostnames, got %d", len(response.Hostnames))
		}
	})

	t.Run("empty hostnames", func(t *testing.T) {
		reconciler := &MockReconciler{}
		handler := NewRuleHandler(reconciler, logger)

		reqBody := types.UpdateHostsRequest{Hostnames: []string{}}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/v1/rule/hosts", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.UpdateHosts(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	handler := NewHealthHandler(logger)

	t.Run("health check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rr := httptest.NewRecorder()

		handler.Health(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if response["status"] != "ok" {
			t.Errorf("expected status %q, got %q", "ok", response["status"])
		}
	})

	t.Run("readiness check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rr := httptest.NewRecorder()

		handler.Ready(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if response["status"] != "ready" {
			t.Errorf("expected status %q, got %q", "ready", response["status"])
		}
	})
}

// MockError implements error interface for testing.
type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
}
