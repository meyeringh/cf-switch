package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meyeringh/cf-switch/internal/cloudflare"
	"github.com/meyeringh/cf-switch/internal/kube"
	"github.com/meyeringh/cf-switch/internal/reconcile"
	"github.com/meyeringh/cf-switch/internal/server"
	"github.com/meyeringh/cf-switch/pkg/types"
)

const (
	// Shutdown timeout for graceful server shutdown.
	shutdownTimeout = 30 * time.Second
)

func main() {
	// Set up structured logging.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Load configuration.
	config, err := types.LoadConfig()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("Starting cf-switch",
		"version", "v0.1.0",
		"zone_id", config.CloudflareZoneID,
		"hostnames", config.DestHostnames,
		"http_addr", config.HTTPAddr,
		"reconcile_interval", config.ReconcileInterval,
		"running_locally", config.RunningLocally)

	var authToken string

	if config.RunningLocally {
		// When running locally, use a static development token
		authToken = generateLocalToken()
		logger.Info("Running in local development mode", "auth_token", authToken)
	} else {
		// Initialize Kubernetes client for secret management.
		kubeClient, err := kube.NewClient(config.Namespace, logger)
		if err != nil {
			logger.Error("Failed to create Kubernetes client", "error", err)
			os.Exit(1)
		}

		// Ensure authentication secret exists and get token.
		authToken, err = kubeClient.EnsureAuthSecret(ctx)
		if err != nil {
			logger.Error("Failed to ensure authentication secret", "error", err)
			os.Exit(1)
		}

		logger.Info("Authentication token ready",
			"secret_name", kube.SecretName,
			"namespace", config.Namespace)
	}

	// Initialize Cloudflare client.
	cfClient := cloudflare.NewClient(config.CloudflareAPIToken, logger)

	// Initialize reconciler.
	reconciler := reconcile.NewReconciler(cfClient, config, logger)

	// Start reconciler.
	if startErr := reconciler.Start(ctx); startErr != nil {
		logger.Error("Failed to start reconciler", "error", startErr)
		os.Exit(1)
	}

	logger.Info("Reconciler started successfully")

	// Initialize HTTP server.
	httpServer := server.NewServer(config.HTTPAddr, authToken, reconciler, logger)

	// Start HTTP server in a goroutine.
	serverErr := make(chan error, 1)
	go func() {
		if startErr := httpServer.Start(); startErr != nil {
			serverErr <- startErr
		}
	}()

	// Set up signal handling for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("cf-switch service is ready",
		"http_addr", config.HTTPAddr,
		"api_auth_required", true,
		"health_endpoints", []string{"/healthz", "/readyz", "/metrics"})

	// Wait for shutdown signal or server error.
	select {
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", "signal", sig)
	case serverError := <-serverErr:
		if serverError != nil {
			logger.Error("HTTP server error", "error", serverError)
		}
	}

	// Graceful shutdown.
	logger.Info("Shutting down cf-switch service")

	// Create shutdown context with timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Stop reconciler.
	reconciler.Stop()
	logger.Info("Reconciler stopped")

	// Shutdown HTTP server.
	if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
		logger.Error("Failed to shutdown HTTP server gracefully", "error", shutdownErr)
	} else {
		logger.Info("HTTP server stopped")
	}

	logger.Info("cf-switch service shutdown complete")
}

// generateLocalToken generates a simple token for local development.
func generateLocalToken() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a static token if random generation fails
		return "local-dev-static"
	}
	return base64.URLEncoding.EncodeToString(bytes)[:22] // Truncate to 22 chars
}
