package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/meyeringh/cf-switch/pkg/types"
)

// AuthMiddleware provides Bearer token authentication.
type AuthMiddleware struct {
	token  string
	logger *slog.Logger
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(token string, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		token:  token,
		logger: logger,
	}
}

// Middleware returns the HTTP middleware function.
func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoints.
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for Bearer token.
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			a.logger.Warn("Missing Authorization header", "path", r.URL.Path, "method", r.Method)
			writeErrorResponse(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			a.logger.Warn("Invalid Authorization header format", "path", r.URL.Path, "method", r.Method)
			writeErrorResponse(w, http.StatusUnauthorized, "Invalid Authorization header format")
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)
		if token != a.token {
			a.logger.Warn("Invalid token", "path", r.URL.Path, "method", r.Method)
			writeErrorResponse(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RuleHandler handles rule-related operations.
type RuleHandler struct {
	reconciler RuleReconciler
	logger     *slog.Logger
}

// RuleReconciler interface for rule management operations.
type RuleReconciler interface {
	GetCurrentRule(ctx context.Context) (*types.Rule, error)
	ToggleRule(ctx context.Context, enabled bool) (*types.Rule, error)
	UpdateHosts(ctx context.Context, hostnames []string) (*types.Rule, error)
}

// NewRuleHandler creates a new rule handler.
func NewRuleHandler(reconciler RuleReconciler, logger *slog.Logger) *RuleHandler {
	return &RuleHandler{
		reconciler: reconciler,
		logger:     logger,
	}
}

// GetRule handles GET /v1/rule.
func (h *RuleHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	rule, err := h.reconciler.GetCurrentRule(r.Context())
	if err != nil {
		h.logger.Error("Failed to get current rule", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to get rule")
		return
	}

	response := types.RuleResponse{
		RuleID:      rule.ID,
		Enabled:     rule.Enabled,
		Expression:  rule.Expression,
		Hostnames:   rule.Hostnames,
		Description: rule.Description,
		Version:     rule.Version,
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// ToggleRule handles POST /v1/rule/enable.
func (h *RuleHandler) ToggleRule(w http.ResponseWriter, r *http.Request) {
	var req types.ToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid request body for toggle", "error", err)
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	rule, err := h.reconciler.ToggleRule(r.Context(), req.Enabled)
	if err != nil {
		h.logger.Error("Failed to toggle rule", "enabled", req.Enabled, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to toggle rule")
		return
	}

	h.logger.Info("Rule toggled successfully", "enabled", req.Enabled, "rule_id", rule.ID)

	response := types.RuleResponse{
		RuleID:      rule.ID,
		Enabled:     rule.Enabled,
		Expression:  rule.Expression,
		Hostnames:   rule.Hostnames,
		Description: rule.Description,
		Version:     rule.Version,
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// UpdateHosts handles PUT /v1/rule/hosts.
func (h *RuleHandler) UpdateHosts(w http.ResponseWriter, r *http.Request) {
	var req types.UpdateHostsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid request body for update hosts", "error", err)
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Hostnames) == 0 {
		h.logger.Warn("Empty hostnames list in request")
		writeErrorResponse(w, http.StatusBadRequest, "Hostnames list cannot be empty")
		return
	}

	rule, err := h.reconciler.UpdateHosts(r.Context(), req.Hostnames)
	if err != nil {
		h.logger.Error("Failed to update hosts", "hostnames", req.Hostnames, "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to update hosts")
		return
	}

	h.logger.Info("Rule hosts updated successfully", "hostnames", req.Hostnames, "rule_id", rule.ID)

	response := types.RuleResponse{
		RuleID:      rule.ID,
		Enabled:     rule.Enabled,
		Expression:  rule.Expression,
		Hostnames:   rule.Hostnames,
		Description: rule.Description,
		Version:     rule.Version,
	}

	writeJSONResponse(w, http.StatusOK, response)
}

// HealthHandler handles health checks.
type HealthHandler struct {
	logger *slog.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(logger *slog.Logger) *HealthHandler {
	return &HealthHandler{logger: logger}
}

// Health handles GET /healthz.
func (h *HealthHandler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready handles GET /readyz.
func (h *HealthHandler) Ready(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeJSONResponse writes a JSON response.
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log the error but don't change the response since headers are already written.
		// Using context.Background() since we don't have request context here.
		//nolint:sloglint // Global logger acceptable for JSON encoding errors after response started
		slog.ErrorContext(context.Background(), "Failed to encode JSON response", "error", err)
	}
}

// writeErrorResponse writes an error response.
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	}
	writeJSONResponse(w, statusCode, response)
}
