// Package missioncontrol provides workflow orchestration for Mission Control.
//
// The workflow package manages task workflows with stages, role assignments,
// and stage transitions based on deliverables and activities.
package missioncontrol

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// WorkflowStage represents a stage in a task workflow.
type WorkflowStage struct {
	Stage   string `json:"stage"`
	Role    string `json:"role"`
	Timeout int    `json:"timeout"` // in minutes
}

// WorkflowTemplate defines a reusable workflow configuration.
type WorkflowTemplate struct {
	ID         string          `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Name       string          `json:"name"`
	Stages     json.RawMessage `json:"stages"`
	Category   string          `json:"category"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

// GetTaskWorkflow retrieves the active workflow for a task.
func GetTaskWorkflow(ctx context.Context, db *DB, taskID string) (*WorkflowTemplate, []WorkflowStage, error) {
	var workspaceID sql.NullString
	var templateID sql.NullString

	err := db.QueryRowContext(ctx,
		"SELECT workspace_id, workflow_template_id FROM tasks WHERE id = ?",
		taskID,
	).Scan(&workspaceID, &templateID)
	if err != nil {
		return nil, nil, fmt.Errorf("get task workflow: %w", err)
	}

	// If no template assigned, use default
	if !templateID.Valid {
		tpl, _ := getDefaultWorkflow()
		return tpl, nil, nil
	}

	// Get template
	var template WorkflowTemplate
	var stagesJSON sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT id, workspace_id, name, stages, category, created_at, updated_at FROM workflow_templates WHERE id = ?",
		templateID.String,
	).Scan(&template.ID, &template.WorkspaceID, &template.Name, &stagesJSON, &template.Category, &template.CreatedAt, &template.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("get workflow template: %w", err)
	}

	// Parse stages
	var stages []WorkflowStage
	if stagesJSON.Valid && stagesJSON.String != "" {
		if err := json.Unmarshal([]byte(stagesJSON.String), &stages); err != nil {
			return &template, nil, fmt.Errorf("parse stages: %w", err)
		}
	}

	return &template, stages, nil
}

// GetTaskRoles retrieves all role assignments for a task.
func GetTaskRoles(ctx context.Context, db *DB, taskID string) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, task_id, role, agent_id, created_at FROM task_roles WHERE task_id = ?",
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("get task roles: %w", err)
	}
	defer rows.Close()

	var roles []map[string]any
	for rows.Next() {
		var id, taskID, role, agentID, createdAt string
		if err := rows.Scan(&id, &taskID, &role, &agentID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, map[string]any{
			"id":         id,
			"task_id":    taskID,
			"role":       role,
			"agent_id":   agentID,
			"created_at": createdAt,
		})
	}
	if roles == nil {
		roles = []map[string]any{}
	}
	return roles, nil
}

// TransitionStage handles workflow stage transitions with role-based handoffs.
func TransitionStage(ctx context.Context, db *DB, taskID, fromStage, toStage string) error {
	// Get current task
	var currentStatus string
	err := db.QueryRowContext(ctx, "SELECT status FROM tasks WHERE id = ?", taskID).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("get task status: %w", err)
	}

	if currentStatus != fromStage {
		return fmt.Errorf("task not in expected stage: current=%s, expected=%s", currentStatus, fromStage)
	}

	// Update task status
	_, err = db.ExecContext(ctx,
		"UPDATE tasks SET status = ?, updated_at = datetime('now') WHERE id = ?",
		toStage, taskID,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	return nil
}

// HandleStageFailure handles failures during a stage with fail-loopback routing.
func HandleStageFailure(ctx context.Context, db *DB, taskID, stage string) error {
	// Get failure count for this stage in this task
	var failureCount int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM task_activities WHERE task_id = ? AND activity_type = 'stage_failure' AND message LIKE ?",
		taskID, "%"+stage+"%",
	).Scan(&failureCount)
	if err != nil {
		return fmt.Errorf("get failure count: %w", err)
	}

	// If too many failures, mark as failed
	if failureCount >= 3 {
		_, err = db.ExecContext(ctx,
			"UPDATE tasks SET status = 'failed', status_reason = ?, updated_at = datetime('now') WHERE id = ?",
			fmt.Sprintf("Failed after %d attempts in stage: %s", failureCount, stage), taskID,
		)
		if err != nil {
			return fmt.Errorf("mark task failed: %w", err)
		}
		return nil
	}

	// Log failure
	_, _ = db.ExecContext(ctx,
		"INSERT INTO task_activities (id, task_id, activity_type, message, created_at) VALUES (?, ?, 'stage_failure', ?, datetime('now'))",
		uuid.New().String(), taskID, fmt.Sprintf("Stage failure in %s (attempt %d)", stage, failureCount+1),
	)

	// For now, just return to previous stage
	// TODO: Implement intelligent fail-loopback based on stage
	returnStageMap := map[string]string{
		"in_progress": "assigned",
		"testing":     "in_progress",
		"review":      "testing",
		"verification":"review",
	}

	if prevStage, ok := returnStageMap[stage]; ok {
		return TransitionStage(ctx, db, taskID, stage, prevStage)
	}

	return nil
}

// DrainQueue checks for tasks in review awaiting approval and processes them.
func DrainQueue(ctx context.Context, db *DB, triggeringTaskID, workspaceID string) error {
	// This is a placeholder for queue-aware review draining
	// In full implementation, this would check for multiple tasks in review
	// and batch-process them when possible
	return nil
}

// getDefaultWorkflow returns the default workflow template.
func getDefaultWorkflow() (*WorkflowTemplate, []WorkflowStage) {
	stages := []WorkflowStage{
		{Stage: "assigned", Role: "", Timeout: 0},
		{Stage: "in_progress", Role: "", Timeout: 0},
		{Stage: "testing", Role: "tester", Timeout: 30},
		{Stage: "review", Role: "reviewer", Timeout: 15},
		{Stage: "done", Role: "", Timeout: 0},
	}
	stagesJSON, _ := json.Marshal(stages)

	return &WorkflowTemplate{
		ID:         "default",
		WorkspaceID: "default",
		Name:       "Standard",
		Stages:     json.RawMessage(stagesJSON),
		Category:   "standard",
	}, stages
}
