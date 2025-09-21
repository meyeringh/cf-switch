package kube

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// SecretName is the name of the secret containing the API token.
	SecretName = "cf-switch-auth" // #nosec G101 -- This is a secret name, not a credential.
	// TokenKey is the key in the secret data containing the token.
	TokenKey = "apiToken"
	// TokenLength is the length of the generated token in bytes.
	TokenLength = 32
)

// Client wraps the Kubernetes client for secret management.
type Client struct {
	clientset *kubernetes.Clientset
	namespace string
	logger    *slog.Logger
}

// NewClient creates a new Kubernetes client for secret management.
func NewClient(namespace string, logger *slog.Logger) (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		clientset: clientset,
		namespace: namespace,
		logger:    logger,
	}, nil
}

// EnsureAuthSecret ensures the authentication secret exists and returns the token.
func (c *Client) EnsureAuthSecret(ctx context.Context) (string, error) {
	// Try to get existing secret and extract valid token.
	if token, exists := c.getExistingToken(ctx); exists {
		return token, nil
	}

	// Generate new token and create/update secret.
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	err = c.createOrUpdateSecret(ctx, token)
	if err != nil {
		return "", err
	}

	c.logTokenReady(ctx)
	return token, nil
}

// getExistingToken attempts to get an existing secret and extract a valid token.
func (c *Client) getExistingToken(ctx context.Context) (string, bool) {
	secret, err := c.clientset.CoreV1().Secrets(c.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			c.logger.WarnContext(ctx, "Failed to get secret", "secret", SecretName, "error", err)
		}
		return "", false
	}

	// Secret exists, check for valid token.
	if tokenBytes, exists := secret.Data[TokenKey]; exists {
		token := string(tokenBytes)
		if len(token) > 0 {
			c.logger.InfoContext(ctx, "Using existing authentication secret", "secret", SecretName)
			return token, true
		}
	}

	c.logger.WarnContext(ctx, "Existing secret has no valid token, regenerating", "secret", SecretName)
	return "", false
}

// createOrUpdateSecret creates or updates the authentication secret with the given token.
func (c *Client) createOrUpdateSecret(ctx context.Context, token string) error {
	secretObj := c.buildSecretObject(token)

	// Try to get existing secret to determine if we should create or update.
	existing, err := c.clientset.CoreV1().Secrets(c.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing secret %s: %w", SecretName, err)
	}

	if existing != nil {
		return c.updateSecret(ctx, secretObj, existing)
	}

	return c.createSecret(ctx, secretObj)
}

// buildSecretObject creates a Secret object with the given token.
func (c *Client) buildSecretObject(token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName,
			Namespace: c.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "cf-switch",
				"app.kubernetes.io/component":  "auth",
				"app.kubernetes.io/managed-by": "cf-switch",
			},
			Annotations: map[string]string{
				"cf-switch.io/description": "API authentication token for cf-switch service",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			TokenKey: []byte(token),
		},
	}
}

// updateSecret updates an existing secret, with fallback to create if it was deleted.
func (c *Client) updateSecret(ctx context.Context, secretObj *corev1.Secret, existing *corev1.Secret) error {
	secretObj.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
	_, err := c.clientset.CoreV1().Secrets(c.namespace).Update(ctx, secretObj, metav1.UpdateOptions{})
	if err != nil {
		// If the secret was deleted between Get() and Update(), create a new one
		if errors.IsNotFound(err) {
			c.logger.WarnContext(ctx, "Secret was deleted during update, creating new one", "secret", SecretName)
			return c.createSecret(ctx, secretObj)
		}
		return fmt.Errorf("failed to update secret %s: %w", SecretName, err)
	}

	c.logger.InfoContext(ctx, "Updated authentication secret", "secret", SecretName)
	return nil
}

// createSecret creates a new secret.
func (c *Client) createSecret(ctx context.Context, secretObj *corev1.Secret) error {
	// Ensure ResourceVersion is not set for create operations
	secretObj.ObjectMeta.ResourceVersion = ""
	_, err := c.clientset.CoreV1().Secrets(c.namespace).Create(ctx, secretObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret %s: %w", SecretName, err)
	}

	c.logger.InfoContext(ctx, "Created authentication secret", "secret", SecretName)
	return nil
}

// logTokenReady logs information about the ready authentication token.
func (c *Client) logTokenReady(ctx context.Context) {
	c.logger.InfoContext(ctx, "Authentication token ready",
		"secret", SecretName,
		"namespace", c.namespace,
		"retrieve_command", fmt.Sprintf(
			"kubectl -n %s get secret %s -o jsonpath='{.data.%s}' | base64 -d",
			c.namespace, SecretName, TokenKey,
		))
}

// generateToken generates a cryptographically secure random token.
func generateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	// Convert raw bytes directly to string - Kubernetes will base64 encode automatically
	return string(bytes), nil
}
