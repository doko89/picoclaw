// Package missioncontrol provides automated rollback functionality.
//
// The rollback package manages post-merge health monitoring and automatic reverts.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RollbackTriggerType represents the type of rollback trigger.
type RollbackTriggerType string

const (
	RollbackTriggerHealthCheck  RollbackTriggerType = "health_check"
	RollbackTriggerCIFailure    RollbackTriggerType = "ci_failure"
	RollbackTriggerManual       RollbackTriggerType = "manual"
	RollbackTriggerCostExceeded RollbackTriggerType = "cost_exceeded"
)

// Rollback represents a rollback event.
type Rollback struct {
	ID          string             `json:"id"`
	ProductID   string             `json:"product_id"`
	TaskID      string             `json:"task_id"`
	TriggerType RollbackTriggerType `json:"trigger_type"`
	Details     string             `json:"details"`
	PRURL       string             `json:"pr_url"`
	RevertPRURL string             `json:"revert_pr_url"`
	Status      string             `json:"status"`       // pending, acknowledged, resolved
	CreatedAt   time.Time          `json:"created_at"`
	AckedAt     *time.Time         `json:"acked_at,omitempty"`
	ResolvedAt  *time.Time         `json:"resolved_at,omitempty"`
}

const (
	// Health check interval after merge
	RollbackHealthCheckInterval = 5 * time.Minute
	// Health check duration
	RollbackHealthCheckDuration = 30 * time.Minute
)

// MonitorPostMerge starts monitoring a product after a merge.
func MonitorPostMerge(ctx context.Context, db *DB, productID, taskID string) {
	go monitorHealth(ctx, db, productID, taskID)
}

// monitorHealth monitors product health after merge and triggers rollback if needed.
func monitorHealth(ctx context.Context, db *DB, productID, taskID string) {
	endTime := time.Now().Add(RollbackHealthCheckDuration)
	ticker := time.NewTicker(RollbackHealthCheckInterval)
	defer ticker.Stop()

	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check health score
			healthy, err := checkProductHealth(ctx, db, productID)
			if err != nil {
				continue
			}

			if !healthy {
				// Trigger rollback
				_, _ = TriggerRollback(ctx, db, productID, taskID, RollbackTriggerHealthCheck, "Health score dropped below threshold")
				return
			}
		}
	}
}

// checkProductHealth checks if a product is healthy.
func checkProductHealth(ctx context.Context, db *DB, productID string) (bool, error) {
	// Get latest health score
	var score sql.NullFloat64
	err := db.QueryRowContext(ctx,
		`SELECT score FROM product_health_scores
		WHERE product_id = ? ORDER BY recorded_at DESC LIMIT 1`,
		productID,
	).Scan(&score)

	if err != nil {
		return true, nil // No score yet, assume healthy
	}

	if !score.Valid {
		return true, nil
	}

	// Health score below 50 is unhealthy
	return score.Float64 >= 50, nil
}

// TriggerRollback triggers an automated rollback.
func TriggerRollback(ctx context.Context, db *DB, productID, taskID string, triggerType RollbackTriggerType, details string) (string, error) {
	// Get task info for PR URL
	var prURL SQLNullString
	err := db.QueryRowContext(ctx,
		"SELECT pr_url FROM workspace_merges WHERE task_id = ?",
		taskID,
	).Scan(&prURL)

	originalPR := ""
	if prURL.Valid {
		originalPR = prURL.String
	}

	// Create revert PR using gh
	revertPRURL, err := createRevertPR(ctx, db, taskID)
	if err != nil {
		return "", fmt.Errorf("create revert PR: %w", err)
	}

	// Record rollback
	rollbackID := uuid.New().String()
	_, err = db.ExecContext(ctx,
		`INSERT INTO rollbacks
		(id, product_id, task_id, trigger_type, details, pr_url, revert_pr_url, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', datetime('now'))`,
		rollbackID, productID, taskID, triggerType, details, originalPR, revertPRURL,
	)
	if err != nil {
		return "", fmt.Errorf("record rollback: %w", err)
	}

	// Pause automation
	_ = SetAutomationTier(ctx, db, productID, AutomationTierSupervised)

	// Broadcast SSE event
	// TODO: Broadcast rollback_triggered event

	return rollbackID, nil
}

// createRevertPR creates a revert PR using gh CLI.
func createRevertPR(ctx context.Context, db *DB, taskID string) (string, error) {
	// Get workspace info for this task
	var worktreePath, baseBranch, originalPRURL string
	err := db.QueryRowContext(ctx,
		`SELECT wm.worktree_path, wm.base_branch, wm.pr_url
		FROM workspace_merges wm
		JOIN tasks t ON t.id = wm.task_id
		WHERE t.id = ?`,
		taskID,
	).Scan(&worktreePath, &baseBranch, &originalPRURL)

	if err != nil {
		return "", fmt.Errorf("get workspace info: %w", err)
	}

	if worktreePath == "" {
		return "", fmt.Errorf("no worktree path found for task %s", taskID)
	}

	// Check if gh CLI is available
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh CLI not found: %w", err)
	}

	// Create a revert branch
	revertBranchName := fmt.Sprintf("revert-%s", taskID[:8])

	// Check if revert branch already exists and delete it
	checkCmd := exec.Command("git", "-C", worktreePath, "checkout", baseBranch)
	if _, err := checkCmd.CombinedOutput(); err != nil {
		// Try to proceed anyway
	}

	// Create new branch for revert
	branchCmd := exec.Command("git", "-C", worktreePath, "checkout", "-b", revertBranchName)
	if output, err := branchCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create revert branch: %w, output: %s", err, string(output))
	}

	// Find the merge commit to revert
	// Assuming the last commit is the merge commit
	logCmd := exec.Command("git", "-C", worktreePath, "log", "--oneline", "-2")
	logOutput, err := logCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get commit log: %w", err)
	}

	// Get the first commit hash (the merge commit)
	var mergeCommitHash string
	fmt.Sscanf(string(logOutput), "%s", &mergeCommitHash)

	if mergeCommitHash != "" {
		// Create revert commit
		revertCmd := exec.Command("git", "-C", worktreePath, "revert", "--no-commit", mergeCommitHash)
		if _, err := revertCmd.CombinedOutput(); err != nil {
			// If revert fails (maybe not a merge commit), try to cherry-pick or just commit the changes
			// For now, we'll create an empty commit with revert message
		}

		// Commit the revert
		commitCmd := exec.Command("git", "-C", worktreePath, "commit", "-m", fmt.Sprintf("Revert task %s", taskID[:8]))
		if _, err := commitCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("commit revert: %w", err)
		}
	}

	// Push the revert branch
	pushCmd := exec.Command("git", "-C", worktreePath, "push", "-u", "origin", revertBranchName)
	if output, err := pushCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("push revert branch: %w, output: %s", err, string(output))
	}

	// Create PR using gh
	prTitle := fmt.Sprintf("Revert: %s", taskID[:8])
	prBody := fmt.Sprintf("Automated revert for task %s\n\nOriginal PR: %s", taskID, originalPRURL)

	// Get the repo URL for gh
	repoURL, _ := getRepoURL(worktreePath)
	if repoURL == "" {
		repoURL = "." // Use current directory if no remote
	}

	prCmd := exec.Command(ghPath, "pr", "create",
		"--base", baseBranch,
		"--head", revertBranchName,
		"--title", prTitle,
		"--body", prBody,
		"--repo", repoURL,
	)
	prOutput, err := prCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create revert PR: %w, output: %s", err, string(prOutput))
	}

	return strings.TrimSpace(string(prOutput)), nil
}

// AcknowledgeRollback acknowledges a rollback and resumes automation.
func AcknowledgeRollback(ctx context.Context, db *DB, rollbackID string, previousTier AutomationTier) error {
	// Get product ID
	var productID string
	err := db.QueryRowContext(ctx,
		"SELECT product_id FROM rollbacks WHERE id = ?",
		rollbackID,
	).Scan(&productID)
	if err != nil {
		return fmt.Errorf("get rollback: %w", err)
	}

	// Update rollback status
	_, err = db.ExecContext(ctx,
		"UPDATE rollbacks SET status = 'acknowledged', acked_at = datetime('now') WHERE id = ?",
		rollbackID,
	)
	if err != nil {
		return fmt.Errorf("update rollback: %w", err)
	}

	// Restore automation tier
	_ = SetAutomationTier(ctx, db, productID, previousTier)

	return nil
}

// GetRollbacks retrieves rollbacks for a product.
func GetRollbacks(ctx context.Context, db *DB, productID string) ([]Rollback, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, task_id, trigger_type, details, pr_url, revert_pr_url, status, created_at, acked_at, resolved_at
		FROM rollbacks WHERE product_id = ? ORDER BY created_at DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query rollbacks: %w", err)
	}
	defer rows.Close()

	var rollbacks []Rollback
	for rows.Next() {
		var r Rollback
		var ackedAt, resolvedAt sqlNullTime
		err := rows.Scan(&r.ID, &r.ProductID, &r.TaskID, &r.TriggerType, &r.Details,
			&r.PRURL, &r.RevertPRURL, &r.Status, &r.CreatedAt, &ackedAt, &resolvedAt)
		if err != nil {
			continue
		}
		if ackedAt.Valid {
			r.AckedAt = &ackedAt.Time
		}
		if resolvedAt.Valid {
			r.ResolvedAt = &resolvedAt.Time
		}
		rollbacks = append(rollbacks, r)
	}

	return rollbacks, nil
}

// sqlNullTime wraps sql.NullTime for convenience.
type sqlNullTime struct {
	Time  time.Time
	Valid bool
}

// Scan implements sql.Scanner interface.
func (t *sqlNullTime) Scan(value any) error {
	if value == nil {
		t.Time, t.Valid = time.Time{}, false
		return nil
	}
	t.Valid = true
	switch v := value.(type) {
	case []byte:
		var err error
		t.Time, err = time.Parse(time.RFC3339, string(v))
		return err
	case string:
		var err error
		t.Time, err = time.Parse(time.RFC3339, v)
		return err
	default:
		return fmt.Errorf("unsupported type for NullTime: %T", value)
	}
}
