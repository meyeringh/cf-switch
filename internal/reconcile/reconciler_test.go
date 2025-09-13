package reconcile

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/meyeringh/cf-switch/internal/cloudflare"
	"github.com/meyeringh/cf-switch/pkg/types"
)

func TestNewReconciler(t *testing.T) {
	config := &types.Config{
		CloudflareZoneID:     "test-zone",
		DestHostnames:        []string{"test.com"},
		CFRuleDefaultEnabled: false,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := cloudflare.NewClient("test-token", logger)

	reconciler := NewReconciler(client, config, logger)

	if reconciler == nil {
		t.Fatal("expected reconciler, got nil")
	}

	if reconciler.config != config {
		t.Error("config not set correctly")
	}

	if reconciler.cfClient != client {
		t.Error("client not set correctly")
	}

	if reconciler.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestReconciler_ConfigAccess(t *testing.T) {
	config := &types.Config{
		CloudflareZoneID:     "test-zone-123",
		DestHostnames:        []string{"example.com", "test.com"},
		CFRuleDefaultEnabled: true,
		HTTPAddr:             ":8080",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := cloudflare.NewClient("test-token", logger)
	reconciler := NewReconciler(client, config, logger)

	// Test that config is accessible and correct.
	if reconciler.config.CloudflareZoneID != "test-zone-123" {
		t.Errorf("expected zone ID %q, got %q", "test-zone-123", reconciler.config.CloudflareZoneID)
	}

	if len(reconciler.config.DestHostnames) != 2 {
		t.Errorf("expected 2 hostnames, got %d", len(reconciler.config.DestHostnames))
	}

	if !reconciler.config.CFRuleDefaultEnabled {
		t.Error("expected CFRuleDefaultEnabled to be true")
	}
}

func TestReconciler_StopChannels(t *testing.T) {
	config := &types.Config{
		CloudflareZoneID:  "test-zone",
		DestHostnames:     []string{"test.com"},
		ReconcileInterval: 100 * time.Millisecond,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := cloudflare.NewClient("test-token", logger)
	reconciler := NewReconciler(client, config, logger)

	// Test that stop channels are initialized.
	if reconciler.stopCh == nil {
		t.Error("stopCh should be initialized")
	}

	if reconciler.stoppedCh == nil {
		t.Error("stoppedCh should be initialized")
	}

	// Test that we can close the stopCh without panic.
	select {
	case reconciler.stopCh <- struct{}{}:
		t.Error("stopCh should be unbuffered and block")
	default:
		// Expected behavior - channel should block since nothing is reading.
	}

	// Test that the channels are properly typed.
	var stopCh = reconciler.stopCh
	var stoppedCh = reconciler.stoppedCh

	if stopCh == nil || stoppedCh == nil {
		t.Error("channels should be properly initialized")
	}
}
