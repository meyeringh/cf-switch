//nolint:testpackage // Using same package as implementation to test unexported functions
package reconcile

import (
	"context"
	"log/slog"
	"os"
	"sync"
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
		ReconcileInterval: 50 * time.Millisecond,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := cloudflare.NewClient("test-token", logger)
	reconciler := NewReconciler(client, config, logger)

	// Test that channels are properly initialized and have the right behavior.
	if reconciler.stopCh == nil {
		t.Error("stopCh should be initialized")
	}

	if reconciler.stoppedCh == nil {
		t.Error("stoppedCh should be initialized")
	}

	// Test that we can send to stopCh (but don't wait for response).
	go func() {
		reconciler.stopCh <- struct{}{}
	}()

	// Test that we can receive from stopCh.
	select {
	case <-reconciler.stopCh:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("should be able to receive from stopCh")
	}
}

func TestReconciler_GetCurrentRule(t *testing.T) {
	config := &types.Config{
		CloudflareZoneID: "test-zone",
		DestHostnames:    []string{"test.com"},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := cloudflare.NewClient("test-token", logger)
	reconciler := NewReconciler(client, config, logger)

	ctx := context.Background()

	// Test when no rule is set.
	rule, err := reconciler.GetCurrentRule(ctx)
	if err == nil {
		t.Error("expected error when no rule is set")
	}
	if rule != nil {
		t.Error("expected nil rule when none is set")
	}

	// Test when rule is set.
	testRule := &types.Rule{
		ID:          "test-rule-id",
		Enabled:     true,
		Expression:  "http.host in {\"test.com\"}",
		Hostnames:   []string{"test.com"},
		Description: "cf-switch:global",
		Version:     1,
	}

	reconciler.mutex.Lock()
	reconciler.currentRule = testRule
	reconciler.mutex.Unlock()

	rule, err = reconciler.GetCurrentRule(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule == nil {
		t.Fatal("expected rule, got nil")
	}

	if rule.ID != testRule.ID {
		t.Errorf("expected rule ID %q, got %q", testRule.ID, rule.ID)
	}

	if rule.Enabled != testRule.Enabled {
		t.Errorf("expected enabled %v, got %v", testRule.Enabled, rule.Enabled)
	}
}

func TestReconciler_UpdateCurrentRule(t *testing.T) {
	config := &types.Config{
		CloudflareZoneID: "test-zone",
		DestHostnames:    []string{"test.com"},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := cloudflare.NewClient("test-token", logger)
	reconciler := NewReconciler(client, config, logger)

	testRule := &types.Rule{
		ID:          "test-rule-id",
		Enabled:     true,
		Expression:  "http.host in {\"test.com\"}",
		Hostnames:   []string{"test.com"},
		Description: "cf-switch:global",
		Version:     1,
	}

	// Test concurrent access safety.
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reconciler.updateCurrentRule(testRule)
		}()
	}

	wg.Wait()

	rule, err := reconciler.GetCurrentRule(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule == nil {
		t.Fatal("expected rule to be set")
	}

	if rule.ID != testRule.ID {
		t.Errorf("expected rule ID %q, got %q", testRule.ID, rule.ID)
	}
}

func TestReconciler_StopBehavior(_ *testing.T) {
	config := &types.Config{
		CloudflareZoneID:  "test-zone",
		DestHostnames:     []string{"test.com"},
		ReconcileInterval: 50 * time.Millisecond,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	client := cloudflare.NewClient("test-token", logger)
	reconciler := NewReconciler(client, config, logger)

	// Start the reconcile loop (without the initial reconcileOnce that makes HTTP calls).
	go reconciler.reconcileLoop()

	// Give it a moment to start.
	time.Sleep(10 * time.Millisecond)

	// Stop the reconciler.
	reconciler.Stop()

	// The test should complete without hanging.
}
