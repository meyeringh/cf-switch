//nolint:testpackage // Using same package as implementation to test unexported functions
package kube

import (
	"log/slog"
	"os"
	"testing"
)

func TestGenerateToken(t *testing.T) {
	token, err := generateToken()
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	if len(token) == 0 {
		t.Error("expected non-empty token")
	}

	// Test that tokens are different.
	token2, err := generateToken()
	if err != nil {
		t.Fatalf("unexpected error generating second token: %v", err)
	}

	if token == token2 {
		t.Error("expected different tokens on subsequent calls")
	}

	// Test minimum length (32 raw bytes).
	if len(token) < TokenLength {
		t.Errorf("expected token length >= %d, got %d", TokenLength, len(token))
	}
}

func TestClientConstants(t *testing.T) {
	if SecretName != "cf-switch-auth" {
		t.Errorf("expected SecretName to be %q, got %q", "cf-switch-auth", SecretName)
	}

	if TokenKey != "apiToken" {
		t.Errorf("expected TokenKey to be %q, got %q", "apiToken", TokenKey)
	}

	if TokenLength != 32 {
		t.Errorf("expected TokenLength to be %d, got %d", 32, TokenLength)
	}
}

func TestNewClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Suppress logs during tests.
	}))

	// This test would require a real Kubernetes cluster or mock.
	// For now, we'll test that it fails gracefully when not in-cluster.
	_, err := NewClient("test-namespace", logger)
	if err == nil {
		t.Skip("skipping test - appears to be running in a Kubernetes cluster")
	}

	// When not in cluster, should return an error.
	if err == nil {
		t.Error("expected error when not running in Kubernetes cluster")
	}
}

// MockKubernetesTest would be used with a mock Kubernetes client.
// This is left as a placeholder for more comprehensive testing.
func TestEnsureAuthSecretMock(t *testing.T) {
	t.Skip("Mock Kubernetes testing not implemented - would require kubernetes/client-go test framework")

	// In a real implementation, this would:.
	// 1. Create a mock clientset
	// 2. Test secret creation
	// 3. Test secret retrieval
	// 4. Test token generation
	// 5. Test error scenarios
}
