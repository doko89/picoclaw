// Package missioncontrol provides automation tier management.
//
// The automation tier package manages automation levels for products.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
)

// AutomationTier represents the automation level for a product.
type AutomationTier string

const (
	AutomationTierSupervised AutomationTier = "supervised"   // Human approval for dispatch + merge
	AutomationTierSemiAuto  AutomationTier = "semi_auto"    // Auto-dispatch, human merge approval
	AutomationTierFullAuto  AutomationTier = "full_auto"    // Auto-dispatch + auto-merge with rollback
)

// GetAutomationTier retrieves the automation tier for a product.
func GetAutomationTier(ctx context.Context, db *DB, productID string) (AutomationTier, error) {
	var tier string
	err := db.QueryRowContext(ctx,
		"SELECT automation_tier FROM products WHERE id = ?",
		productID,
	).Scan(&tier)

	if err == sql.ErrNoRows {
		return AutomationTierSupervised, nil // Default
	}
	if err != nil {
		return "", fmt.Errorf("get automation tier: %w", err)
	}

	return AutomationTier(tier), nil
}

// SetAutomationTier sets the automation tier for a product.
func SetAutomationTier(ctx context.Context, db *DB, productID string, tier AutomationTier) error {
	_, err := db.ExecContext(ctx,
		"UPDATE products SET automation_tier = ?, updated_at = datetime('now') WHERE id = ?",
		tier, productID,
	)
	if err != nil {
		return fmt.Errorf("set automation tier: %w", err)
	}
	return nil
}

// CanAutoDispatch checks if a product can auto-dispatch tasks.
func CanAutoDispatch(ctx context.Context, db *DB, productID string) (bool, error) {
	tier, err := GetAutomationTier(ctx, db, productID)
	if err != nil {
		return false, err
	}
	return tier == AutomationTierSemiAuto || tier == AutomationTierFullAuto, nil
}

// CanAutoMerge checks if a product can auto-merge (with rollback).
func CanAutoMerge(ctx context.Context, db *DB, productID string) (bool, error) {
	tier, err := GetAutomationTier(ctx, db, productID)
	if err != nil {
		return false, err
	}
	return tier == AutomationTierFullAuto, nil
}
