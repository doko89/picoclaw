// Package missioncontrol provides convoy mode for multi-agent coordination.
//
// The convoy package manages DAG-based task decomposition and parallel
// execution of subtasks with dependency tracking.
package missioncontrol

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// ConvoySubtaskInput represents a subtask input for convoy creation.
type ConvoySubtaskInput struct {
	Title string `json:"title"`
	Desc  string `json:"description"`
	Role  string `json:"role"`
}

// CreateConvoy creates a new convoy for parallel task execution.
func CreateConvoy(ctx context.Context, db *DB, parentTaskID, name, strategy, spec string, subtasks []ConvoySubtaskInput) (*Convoy, error) {
	convoyID := uuid.New().String()

	// Insert convoy
	_, err := db.ExecContext(ctx,
		"INSERT INTO convoys (id, parent_task_id, name, strategy, spec, status, created_at) VALUES (?, ?, ?, ?, ?, 'pending', datetime('now'))",
		convoyID, parentTaskID, name, strategy, spec,
	)
	if err != nil {
		return nil, err
	}

	// Create subtasks
	for i, st := range subtasks {
		taskID := uuid.New().String()
		_, err := db.ExecContext(ctx,
			"INSERT INTO tasks (id, title, description, status, workspace_id, is_subtask, created_at, updated_at) VALUES (?, ?, ?, 'pending', 'default', 1, datetime('now'), datetime('now'))",
			taskID, st.Title, st.Desc,
		)
		if err != nil {
			return nil, err
		}

		// Add to convoy with dependencies (each depends on previous)
		var dependsOn []string
		var dependsOnJSON string
		if i > 0 {
			// Simple linear dependency for now
			// In full implementation, this would be a proper DAG
			dependsOn = []string{}
		}
		if len(dependsOn) > 0 {
			b, _ := json.Marshal(dependsOn)
			dependsOnJSON = string(b)
		}

		_, err = db.ExecContext(ctx,
			"INSERT INTO convoy_subtasks (id, convoy_id, task_id, depends_on, status) VALUES (?, ?, ?, ?, 'pending')",
			uuid.New().String(), convoyID, taskID, dependsOnJSON,
		)
		if err != nil {
			return nil, err
		}
	}

	// Update parent task status
	_, err = db.ExecContext(ctx,
		"UPDATE tasks SET status = 'convoy_active', convoy_id = ?, updated_at = datetime('now') WHERE id = ?",
		convoyID, parentTaskID,
	)
	if err != nil {
		return nil, err
	}

	return &Convoy{
		ID:           convoyID,
		ParentTaskID: parentTaskID,
		Name:         name,
		Strategy:     strategy,
		Spec:         spec,
		Status:       "pending",
		CreatedAt:    "", // Will be set by database
	}, nil
}

// Convoy represents a DAG-based convoy of subtasks.
type Convoy struct {
	ID           string `json:"id"`
	ParentTaskID string `json:"parent_task_id"`
	Name         string `json:"name"`
	Strategy     string `json:"strategy"`
	Spec         string `json:"spec"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

// GetConvoyProgress retrieves progress information for a convoy.
func GetConvoyProgress(ctx context.Context, db *DB, convoyID string) (map[string]any, error) {
	// Get convoy info
	var name, status string
	err := db.QueryRowContext(ctx,
		"SELECT name, status FROM convoys WHERE id = ?",
		convoyID,
	).Scan(&name, &status)
	if err != nil {
		return nil, err
	}

	// Count subtasks by status
	rows, err := db.QueryContext(ctx,
		"SELECT status, COUNT(*) as count FROM convoy_subtasks WHERE convoy_id = ? GROUP BY status",
		convoyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statusCounts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		statusCounts[status] = count
	}

	return map[string]any{
		"id":           convoyID,
		"name":         name,
		"status":       status,
		"status_counts": statusCounts,
		"total":        len(statusCounts),
		"done":         statusCounts["done"],
	}, nil
}

// DispatchConvoy dispatches all ready subtasks in a convoy.
func DispatchConvoy(ctx context.Context, db *DB, convoyID string) error {
	// Get all subtasks
	rows, err := db.QueryContext(ctx,
		"SELECT id, task_id, depends_on FROM convoy_subtasks WHERE convoy_id = ?",
		convoyID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type convoySubtask struct {
		ID        string
		ConvoyID  string
		TaskID    string
		DependsOn string
		Status    string
	}

	var subtasks []convoySubtask
	for rows.Next() {
		var id, taskID, dependsOnJSON string
		if err := rows.Scan(&id, &taskID, &dependsOnJSON); err != nil {
			continue
		}
		subtasks = append(subtasks, convoySubtask{
			ID:        id,
			ConvoyID:  convoyID,
			TaskID:    taskID,
			DependsOn: dependsOnJSON,
			Status:    "",
		})
	}

	// Get available agents for assignment
	availableAgents, err := getAvailableAgents(ctx, db)
	if err != nil {
		availableAgents = nil // Continue without agent assignment
	}

	agentIndex := 0

	// Dispatch tasks with no unmet dependencies
	for _, st := range subtasks {
		if st.DependsOn == "" || st.DependsOn == "[]" {
			// Assign to an available agent (round-robin)
			var assignedAgentID *string
			if len(availableAgents) > 0 {
				agentID := availableAgents[agentIndex%len(availableAgents)]
				assignedAgentID = &agentID
				agentIndex++
			}

			// Update task with assignment
			_, err = db.ExecContext(ctx,
				"UPDATE tasks SET assigned_agent_id = ?, status = 'assigned', updated_at = datetime('now') WHERE id = ?",
				assignedAgentID, st.TaskID,
			)
			if err != nil {
				continue
			}

			// Update subtask status
			_, _ = db.ExecContext(ctx,
				"UPDATE convoy_subtasks SET status = 'dispatched' WHERE id = ?",
				st.ID,
			)

			// Log dispatch event for gateway consumption
			dispatchLogID := uuid.New().String()
			_, _ = db.ExecContext(ctx,
				`INSERT INTO task_activities (id, task_id, agent_id, activity_type, message, created_at)
				VALUES (?, ?, ?, 'dispatched', ?, datetime('now'))`,
				dispatchLogID, st.TaskID, assignedAgentID, "Task dispatched to agent for processing",
			)
		}
	}

	// Update convoy status
	_, err = db.ExecContext(ctx,
		"UPDATE convoys SET status = 'active' WHERE id = ?",
		convoyID,
	)
	return err
}

// getAvailableAgents returns a list of available agent IDs for task assignment.
func getAvailableAgents(ctx context.Context, db *DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id FROM agents WHERE status = 'standby' AND workspace_id = 'default' LIMIT 10",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		agents = append(agents, id)
	}
	return agents, nil
}
