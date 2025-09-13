package kube

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// SecretName is the name of the secret containing the API token
	SecretName = "cf-switch-auth" // #nosec G101 -- This is a secret name, not a credential
	// TokenKey is the key in the secret data containing the token
	TokenKey = "apiToken"
	// TokenLength is the length of the generated token in bytes
	TokenLength = 32
)

// Client wraps the Kubernetes client for secret management
type Client struct {
	clientset *kubernetes.Clientset
	namespace string
	logger    *slog.Logger
}

// NewClient creates a new Kubernetes client for secret management
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

// EnsureAuthSecret ensures the authentication secret exists and returns the token
func (c *Client) EnsureAuthSecret(ctx context.Context) (string, error) {
	// Try to get existing secret
	secret, err := c.clientset.CoreV1().Secrets(c.namespace).Get(ctx, SecretName, metav1.GetOptions{})
	if err == nil {
		// Secret exists, extract token
		if tokenBytes, exists := secret.Data[TokenKey]; exists {
			token := string(tokenBytes)
			if len(token) > 0 {
				c.logger.Info("Using existing authentication secret", "secret", SecretName)
				return token, nil
			}
		}
		c.logger.Warn("Existing secret has no valid token, regenerating", "secret", SecretName)
	} else if !errors.IsNotFound(err) {
		return "", fmt.Errorf("failed to get secret %s: %w", SecretName, err)
	}

	// Generate new token
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Create or update secret
	secretData := map[string][]byte{
		TokenKey: []byte(token),
	}

	secretObj := &corev1.Secret{
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
		Data: secretData,
	}

	if secret != nil {
		// Update existing secret
		secretObj.ObjectMeta.ResourceVersion = secret.ObjectMeta.ResourceVersion
		_, err = c.clientset.CoreV1().Secrets(c.namespace).Update(ctx, secretObj, metav1.UpdateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to update secret %s: %w", SecretName, err)
		}
		c.logger.Info("Updated authentication secret", "secret", SecretName)
	} else {
		// Create new secret
		_, err = c.clientset.CoreV1().Secrets(c.namespace).Create(ctx, secretObj, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to create secret %s: %w", SecretName, err)
		}
		c.logger.Info("Created authentication secret", "secret", SecretName)
	}

	c.logger.Info("Authentication token ready",
		"secret", SecretName,
		"namespace", c.namespace,
		"retrieve_command", fmt.Sprintf("kubectl -n %s get secret %s -o jsonpath='{.data.%s}' | base64 -d", c.namespace, SecretName, TokenKey))

	return token, nil
}

// generateToken generates a cryptographically secure random token
func generateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
