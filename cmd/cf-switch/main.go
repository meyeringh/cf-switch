package main

import (
	"context"
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

	logger.Info("Starting cf-switch service",
		"version", "v0.1.0",
		"zone_id", config.CloudflareZoneID,
		"hostnames", config.DestHostnames,
		"http_addr", config.HTTPAddr,
		"reconcile_interval", config.ReconcileInterval)

	// Initialize Kubernetes client for secret management.
	kubeClient, err := kube.NewClient(config.Namespace, logger)
	if err != nil {
		logger.Error("Failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	// Ensure authentication secret exists and get token.
	authToken, err := kubeClient.EnsureAuthSecret(ctx)
	if err != nil {
		logger.Error("Failed to ensure authentication secret", "error", err)
		os.Exit(1)
	}

	logger.Info("Authentication token ready",
		"secret_name", kube.SecretName,
		"namespace", config.Namespace)

	// Initialize Cloudflare client.
	cfClient := cloudflare.NewClient(config.CloudflareAPIToken, logger)

	// Initialize reconciler.
	reconciler := reconcile.NewReconciler(cfClient, config, logger)

	// Start reconciler.
	if err := reconciler.Start(ctx); err != nil {
		logger.Error("Failed to start reconciler", "error", err)
		os.Exit(1)
	}

	logger.Info("Reconciler started successfully")

	// Initialize HTTP server.
	httpServer := server.NewServer(config.HTTPAddr, authToken, reconciler, logger)

	// Start HTTP server in a goroutine.
	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.Start(); err != nil {
			serverErr <- err
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
	case err := <-serverErr:
		if err != nil {
			logger.Error("HTTP server error", "error", err)
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
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Failed to shutdown HTTP server gracefully", "error", err)
	} else {
		logger.Info("HTTP server stopped")
	}

	logger.Info("cf-switch service shutdown complete")
}
