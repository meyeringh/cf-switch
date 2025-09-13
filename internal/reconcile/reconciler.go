package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/meyeringh/cf-switch/internal/cloudflare"
	"github.com/meyeringh/cf-switch/pkg/types"
)

// Reconciler manages the Cloudflare WAF Custom Rule
type Reconciler struct {
	cfClient    *cloudflare.Client
	config      *types.Config
	logger      *slog.Logger
	mutex       sync.RWMutex
	currentRule *types.Rule
	rulesetID   string
	stopCh      chan struct{}
	stoppedCh   chan struct{}
}

// NewReconciler creates a new reconciler
func NewReconciler(cfClient *cloudflare.Client, config *types.Config, logger *slog.Logger) *Reconciler {
	return &Reconciler{
		cfClient:  cfClient,
		config:    config,
		logger:    logger,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Start begins the reconciliation process
func (r *Reconciler) Start(ctx context.Context) error {
	// Initial reconciliation
	if err := r.reconcileOnce(ctx); err != nil {
		return fmt.Errorf("initial reconciliation failed: %w", err)
	}

	// Start periodic reconciliation
	go r.reconcileLoop()
	return nil
}

// Stop stops the reconciliation process
func (r *Reconciler) Stop() {
	close(r.stopCh)
	<-r.stoppedCh
}

// GetCurrentRule returns the current rule state
func (r *Reconciler) GetCurrentRule(ctx context.Context) (*types.Rule, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.currentRule == nil {
		return nil, fmt.Errorf("no rule available")
	}

	// Return a copy to avoid external modifications
	rule := *r.currentRule
	return &rule, nil
}

// ToggleRule enables or disables the rule
func (r *Reconciler) ToggleRule(ctx context.Context, enabled bool) (*types.Rule, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.currentRule == nil || r.rulesetID == "" {
		return nil, fmt.Errorf("rule not initialized")
	}

	updates := map[string]interface{}{
		"enabled": enabled,
	}

	updatedRule, err := r.cfClient.UpdateRule(ctx, r.config.CloudflareZoneID, r.rulesetID, r.currentRule.ID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	// Update cached rule
	r.currentRule.Enabled = updatedRule.Enabled
	r.currentRule.Version = updatedRule.Version

	r.logger.Info("Rule toggled successfully",
		"rule_id", r.currentRule.ID,
		"enabled", enabled,
		"version", r.currentRule.Version)

	rule := *r.currentRule
	return &rule, nil
}

// UpdateHosts updates the hostnames in the rule
func (r *Reconciler) UpdateHosts(ctx context.Context, hostnames []string) (*types.Rule, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.currentRule == nil || r.rulesetID == "" {
		return nil, fmt.Errorf("rule not initialized")
	}

	// Normalize hostnames
	normalizedHosts := types.ParseHostnames(fmt.Sprintf("%v", hostnames))
	if len(normalizedHosts) == 0 {
		return nil, fmt.Errorf("no valid hostnames provided")
	}

	// Build new expression
	expression := types.BuildExpression(normalizedHosts)

	updates := map[string]interface{}{
		"expression": expression,
	}

	updatedRule, err := r.cfClient.UpdateRule(ctx, r.config.CloudflareZoneID, r.rulesetID, r.currentRule.ID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update rule expression: %w", err)
	}

	// Update cached rule
	r.currentRule.Expression = updatedRule.Expression
	r.currentRule.Hostnames = normalizedHosts
	r.currentRule.Version = updatedRule.Version

	r.logger.Info("Rule hosts updated successfully",
		"rule_id", r.currentRule.ID,
		"hostnames", normalizedHosts,
		"expression", expression,
		"version", r.currentRule.Version)

	rule := *r.currentRule
	return &rule, nil
}

// reconcileLoop runs the periodic reconciliation
func (r *Reconciler) reconcileLoop() {
	defer close(r.stoppedCh)

	ticker := time.NewTicker(r.config.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			r.logger.Info("Reconciliation loop stopped")
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			if err := r.reconcileOnce(ctx); err != nil {
				r.logger.Error("Reconciliation failed", "error", err)
			}
			cancel()
		}
	}
}

// reconcileOnce performs a single reconciliation
func (r *Reconciler) reconcileOnce(ctx context.Context) error {
	r.logger.Debug("Starting reconciliation")

	// Get or create entrypoint ruleset
	ruleset, err := r.ensureEntrypointRuleset(ctx)
	if err != nil {
		return fmt.Errorf("failed to ensure entrypoint ruleset: %w", err)
	}

	r.mutex.Lock()
	r.rulesetID = ruleset.ID
	r.mutex.Unlock()

	// Ensure our rule exists and is up to date
	if err := r.ensureRule(ctx, ruleset); err != nil {
		return fmt.Errorf("failed to ensure rule: %w", err)
	}

	r.logger.Debug("Reconciliation completed successfully")
	return nil
}

// ensureEntrypointRuleset ensures the entrypoint ruleset exists
func (r *Reconciler) ensureEntrypointRuleset(ctx context.Context) (*types.CloudflareRuleset, error) {
	phase := types.HTTPRequestFirewallCustomPhase

	// Try to get existing entrypoint
	ruleset, err := r.cfClient.GetEntrypointRuleset(ctx, r.config.CloudflareZoneID, phase)
	if err != nil {
		return nil, fmt.Errorf("failed to get entrypoint ruleset: %w", err)
	}

	if ruleset != nil {
		r.logger.Debug("Found existing entrypoint ruleset", "ruleset_id", ruleset.ID)
		return ruleset, nil
	}

	// Create entrypoint ruleset
	r.logger.Info("Creating entrypoint ruleset", "phase", phase)
	ruleset, err = r.cfClient.CreateEntrypointRuleset(ctx, r.config.CloudflareZoneID, phase)
	if err != nil {
		return nil, fmt.Errorf("failed to create entrypoint ruleset: %w", err)
	}

	r.logger.Info("Created entrypoint ruleset", "ruleset_id", ruleset.ID)
	return ruleset, nil
}

// ensureRule ensures our rule exists and is configured correctly
func (r *Reconciler) ensureRule(ctx context.Context, ruleset *types.CloudflareRuleset) error {
	// Look for existing rule
	existingRule := cloudflare.FindRuleByDescription(ruleset, types.RuleDescription)

	// Build expected expression
	expectedExpression := types.BuildExpression(r.config.DestHostnames)

	if existingRule == nil {
		// Create new rule
		rule := types.CloudflareRule{
			Action:      types.BlockAction,
			Expression:  expectedExpression,
			Description: types.RuleDescription,
			Enabled:     r.config.CFRuleDefaultEnabled,
		}

		createdRule, err := r.cfClient.AddRule(ctx, r.config.CloudflareZoneID, ruleset.ID, rule)
		if err != nil {
			return fmt.Errorf("failed to create rule: %w", err)
		}

		r.mutex.Lock()
		r.currentRule = &types.Rule{
			ID:          createdRule.ID,
			Enabled:     createdRule.Enabled,
			Expression:  createdRule.Expression,
			Hostnames:   r.config.DestHostnames,
			Description: createdRule.Description,
			Version:     createdRule.Version,
		}
		r.mutex.Unlock()

		r.logger.Info("Created new rule",
			"rule_id", createdRule.ID,
			"enabled", createdRule.Enabled,
			"expression", createdRule.Expression)

		return nil
	}

	// Update existing rule if needed
	needsUpdate := false
	updates := make(map[string]interface{})

	if existingRule.Expression != expectedExpression {
		updates["expression"] = expectedExpression
		needsUpdate = true
		r.logger.Info("Rule expression needs update",
			"rule_id", existingRule.ID,
			"current", existingRule.Expression,
			"expected", expectedExpression)
	}

	if needsUpdate {
		updatedRule, err := r.cfClient.UpdateRule(ctx, r.config.CloudflareZoneID, ruleset.ID, existingRule.ID, updates)
		if err != nil {
			return fmt.Errorf("failed to update rule: %w", err)
		}

		r.mutex.Lock()
		r.currentRule = &types.Rule{
			ID:          updatedRule.ID,
			Enabled:     updatedRule.Enabled,
			Expression:  updatedRule.Expression,
			Hostnames:   r.config.DestHostnames,
			Description: updatedRule.Description,
			Version:     updatedRule.Version,
		}
		r.mutex.Unlock()

		r.logger.Info("Updated rule",
			"rule_id", updatedRule.ID,
			"expression", updatedRule.Expression,
			"version", updatedRule.Version)
	} else {
		// No update needed, just cache current state
		r.mutex.Lock()
		r.currentRule = &types.Rule{
			ID:          existingRule.ID,
			Enabled:     existingRule.Enabled,
			Expression:  existingRule.Expression,
			Hostnames:   r.config.DestHostnames,
			Description: existingRule.Description,
			Version:     existingRule.Version,
		}
		r.mutex.Unlock()

		r.logger.Debug("Rule is up to date", "rule_id", existingRule.ID)
	}

	return nil
}
