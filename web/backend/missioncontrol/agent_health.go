// Package missioncontrol provides agent health monitoring for Mission Control.
//
// The health package monitors agent sessions, detects stuck/zombie agents,
// and provides nudge functionality for recovery.
package missioncontrol

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// Health thresholds
	StallThreshold    = 5 * time.Minute  // Agent considered stalled after no activity
	StuckThreshold    = 15 * time.Minute // Agent considered stuck after no activity
	AutoNudgeCount     = 3                // Auto-nudge after 3 consecutive stalls
	HealthCheckInterval = 6 * time.Minute // Run health check cycle every 6 minutes
)

// AgentHealthState represents the health status of an agent.
type AgentHealthState string

const (
	HealthOffline   AgentHealthState = "offline"
	HealthIdle      AgentHealthState = "idle"
	HealthZombie    AgentHealthState = "zombie" // Session exists but no response
	HealthStuck     AgentHealthState = "stuck"  // No progress for too long
	HealthStalled   AgentHealthState = "stalled" // Temporary stall
	HealthWorking   AgentHealthState = "working" // Active and making progress
)

// AgentHealthInfo represents the health status of an agent.
type AgentHealthInfo struct {
	AgentID        string           `json:"agent_id"`
	AgentName      string           `json:"agent_name"`
	Status         string           `json:"status"`
	HealthState    AgentHealthState `json:"health_state"`
	LastActivity   *time.Time       `json:"last_activity"`
	SessionKey     *string          `json:"session_key"`
	CurrentTaskID  *string          `json:"current_task_id"`
	StallCount     int              `json:"stall_count"`
	LastCheckTime  time.Time        `json:"last_check_time"`
}

// CheckAgentHealth determines the health state of an agent using a decision tree.
func CheckAgentHealth(ctx context.Context, db *DB, agentID string) (*AgentHealthInfo, error) {
	// Get agent info
	var name, status string
	var lastUpdate sql.NullString
	err := db.QueryRowContext(ctx,
		"SELECT name, status, updated_at FROM agents WHERE id = ?",
		agentID,
	).Scan(&name, &status, &lastUpdate)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}

	info := &AgentHealthInfo{
		AgentID:       agentID,
		AgentName:     name,
		Status:        status,
		HealthState:   HealthWorking,
		LastCheckTime: time.Now(),
	}

	// Agent is offline
	if status == "offline" {
		info.HealthState = HealthOffline
		return info, nil
	}

	// Check last activity from events
	var lastEventTime sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT MAX(created_at) FROM events WHERE agent_id = ?",
		agentID,
	).Scan(&lastEventTime)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get last event: %w", err)
	}

	if lastEventTime.Valid {
		t, err := time.Parse(time.RFC3339, lastEventTime.String)
		if err == nil {
			info.LastActivity = &t
		}
	}

	// Check for active session/task
	var currentTaskID sql.NullString
	var sessionKey sql.NullString
	_ = db.QueryRowContext(ctx,
		"SELECT id, planning_session_key FROM tasks WHERE assigned_agent_id = ? AND status NOT IN ('done', 'failed') LIMIT 1",
		agentID,
	).Scan(&currentTaskID, &sessionKey)

	if currentTaskID.Valid {
		info.CurrentTaskID = &currentTaskID.String
	}
	if sessionKey.Valid {
		info.SessionKey = &sessionKey.String
	}

	// Decision tree for health state
	now := time.Now()

	// No recent activity
	if info.LastActivity != nil {
		inactive := now.Sub(*info.LastActivity)

		if inactive > StuckThreshold {
			info.HealthState = HealthStuck
		} else if inactive > StallThreshold {
			info.HealthState = HealthStalled
		}
	}

	// If assigned but no activity
	if info.CurrentTaskID != nil && info.LastActivity == nil {
		info.HealthState = HealthZombie
	}

	// If not assigned and no activity
	if info.CurrentTaskID == nil && (info.LastActivity == nil || now.Sub(*info.LastActivity) > 24*time.Hour) {
		info.HealthState = HealthIdle
	}

	return info, nil
}

// RunHealthCheckCycle performs a full health check on all agents and updates the database.
func RunHealthCheckCycle(ctx context.Context, db *DB) ([]*AgentHealthInfo, error) {
	// Get all active agents
	rows, err := db.QueryContext(ctx,
		"SELECT id FROM agents WHERE status != 'offline' ORDER BY name",
	)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agentIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		agentIDs = append(agentIDs, id)
	}

	var results []*AgentHealthInfo
	for _, agentID := range agentIDs {
		info, err := CheckAgentHealth(ctx, db, agentID)
		if err != nil {
			continue
		}
		results = append(results, info)

		// Update agent_health table
		_, _ = db.ExecContext(ctx,
			`INSERT OR REPLACE INTO agent_health (id, agent_id, health_state, last_activity, current_task_id, stall_count, checked_at)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
			uuid.New().String(), info.AgentID, string(info.HealthState),
			formatTimePtr(info.LastActivity), info.CurrentTaskID, info.StallCount,
		)

		// Auto-nudge if consecutively stalled
		if info.HealthState == HealthStalled && info.StallCount >= AutoNudgeCount {
			if info.CurrentTaskID != nil {
				_ = NudgeAgent(ctx, db, agentID, *info.CurrentTaskID)
			}
		}
	}

	return results, nil
}

// NudgeAgent kills an agent's session, saves a checkpoint, and re-dispatches the task.
func NudgeAgent(ctx context.Context, db *DB, agentID, taskID string) error {
	// Get task session key and current status
	var sessionKey sql.NullString
	var currentStatus string
	err := db.QueryRowContext(ctx,
		"SELECT planning_session_key, status FROM tasks WHERE id = ?",
		taskID,
	).Scan(&sessionKey, &currentStatus)
	if err != nil {
		return fmt.Errorf("get task session: %w", err)
	}

	// Save checkpoint before nudge
	if sessionKey.Valid {
		_ = SaveCheckpoint(ctx, db, taskID, agentID, "nudge", map[string]any{
			"reason":      "Agent nudged due to stall",
			"session_key": sessionKey.String,
			"prev_status": currentStatus,
		})
	}

	// Log the nudge activity
	_, _ = db.ExecContext(ctx,
		"INSERT INTO task_activities (id, task_id, activity_type, message, agent_id, created_at) VALUES (?, ?, 'nudged', 'Agent nudged and task reset for re-dispatch', ?, datetime('now'))",
		uuid.New().String(), taskID, agentID,
	)

	// Reset task status to allow re-dispatch
	if currentStatus == "in_progress" || currentStatus == "testing" || currentStatus == "review" {
		_, err := db.ExecContext(ctx,
			"UPDATE tasks SET status = 'assigned', updated_at = datetime('now') WHERE id = ?",
			taskID,
		)
		if err != nil {
			return fmt.Errorf("reset task status: %w", err)
		}
	}

	// Clear the planning session key to reset any stuck planning state
	if sessionKey.Valid {
		_, _ = db.ExecContext(ctx,
			"UPDATE tasks SET planning_session_key = NULL, updated_at = datetime('now') WHERE id = ?",
			taskID,
		)
	}

	// Update agent health status
	_, _ = db.ExecContext(ctx,
		"UPDATE agent_health SET status = 'nudged', last_checked = datetime('now') WHERE agent_id = ?",
		agentID,
	)

	return nil
}

// StartHealthMonitor starts the background goroutine for periodic health checks.
// This should be called once at startup.
func StartHealthMonitor(ctx context.Context, db *DB) {
	go func() {
		ticker := time.NewTicker(HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := RunHealthCheckCycle(ctx, db)
				if err != nil {
					// Log but continue
				}
			}
		}
	}()
}

// SaveCheckpoint saves a checkpoint for a task.
func SaveCheckpoint(ctx context.Context, db *DB, taskID, agentID, checkpointType string, state map[string]any) error {
	stateJSON, _ := json.Marshal(state)

	_, err := db.ExecContext(ctx,
		"INSERT INTO work_checkpoints (id, task_id, agent_id, checkpoint_type, state_summary, created_at) VALUES (?, ?, ?, ?, ?, datetime('now'))",
		uuid.New().String(), taskID, agentID, checkpointType, string(stateJSON),
	)
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}

	return nil
}

// GetCheckpoints retrieves all checkpoints for a task.
func GetCheckpoints(ctx context.Context, db *DB, taskID string) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, task_id, agent_id, checkpoint_type, state_summary, created_at FROM work_checkpoints WHERE task_id = ? ORDER BY created_at DESC",
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("get checkpoints: %w", err)
	}
	defer rows.Close()

	var checkpoints []map[string]any
	for rows.Next() {
		var id, taskID, agentID, checkpointType, stateSummary, createdAt string
		if err := rows.Scan(&id, &taskID, &agentID, &checkpointType, &stateSummary, &createdAt); err != nil {
			return nil, fmt.Errorf("scan checkpoint: %w", err)
		}
		checkpoints = append(checkpoints, map[string]any{
			"id":             id,
			"task_id":        taskID,
			"agent_id":       agentID,
			"checkpoint_type": checkpointType,
			"state_summary":  stateSummary,
			"created_at":     createdAt,
		})
	}
	if checkpoints == nil {
		checkpoints = []map[string]any{}
	}
	return checkpoints, nil
}

// RestoreCheckpoint restores a task from a checkpoint.
func RestoreCheckpoint(ctx context.Context, db *DB, taskID, checkpointID string) error {
	// Get checkpoint data
	var stateSummary, checkpointType string
	var agentID sql.NullString
	err := db.QueryRowContext(ctx,
		"SELECT state_summary, checkpoint_type, agent_id FROM work_checkpoints WHERE id = ? AND task_id = ?",
		checkpointID, taskID,
	).Scan(&stateSummary, &checkpointType, &agentID)
	if err != nil {
		return fmt.Errorf("get checkpoint: %w", err)
	}

	// Parse state summary to get task context
	var state map[string]any
	if stateSummary != "" {
		if err := json.Unmarshal([]byte(stateSummary), &state); err != nil {
			return fmt.Errorf("parse checkpoint state: %w", err)
		}
	}

	// Update task status to in_progress for retry
	_, err = db.ExecContext(ctx,
		"UPDATE tasks SET status = 'in_progress', updated_at = datetime('now') WHERE id = ?",
		taskID,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	// Log the restore activity
	activityMsg := fmt.Sprintf("Restored from checkpoint %s (%s)", checkpointID[:8], checkpointType)
	if agentID.Valid {
		activityMsg += fmt.Sprintf(", agent: %s", agentID.String[:8])
	}

	_, _ = db.ExecContext(ctx,
		"INSERT INTO task_activities (id, task_id, agent_id, activity_type, message, created_at) VALUES (?, ?, ?, 'checkpoint_restored', ?, datetime('now'))",
		uuid.New().String(), taskID, agentID, activityMsg,
	)

	return nil
}

func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.RFC3339), Valid: true}
}
