// Package missioncontrol provides cost tracking for autopilot operations.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CostEvent represents a cost event record.
type CostEvent struct {
	ID           string    `json:"id"`
	ProductID    *string   `json:"product_id,omitempty"`
	WorkspaceID  string    `json:"workspace_id"`
	TaskID       *string   `json:"task_id,omitempty"`
	CycleID      *string   `json:"cycle_id,omitempty"`
	AgentID      *string   `json:"agent_id,omitempty"`
	EventType    string    `json:"event_type"`
	Provider     *string   `json:"provider,omitempty"`
	Model        *string   `json:"model,omitempty"`
	TokensInput  int       `json:"tokens_input"`
	TokensOutput int       `json:"tokens_output"`
	CostUSD      float64   `json:"cost_usd"`
	Metadata     *string   `json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// CostCap represents a spending limit.
type CostCap struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	ProductID       *string   `json:"product_id,omitempty"`
	CapType         string    `json:"cap_type"` // per_cycle, per_task, daily, monthly, per_product_monthly
	LimitUSD        float64   `json:"limit_usd"`
	CurrentSpendUSD float64   `json:"current_spend_usd"`
	PeriodStart     *string   `json:"period_start,omitempty"`
	PeriodEnd       *string   `json:"period_end,omitempty"`
	Status          string    `json:"status"` // active, paused, exceeded
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RecordCostEvent records a cost event and updates task/agent costs.
func RecordCostEvent(ctx context.Context, db *DB, input CostEvent) error {
	if input.ID == "" {
		input.ID = uuid.New().String()
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now()
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO cost_events
		(id, product_id, workspace_id, task_id, cycle_id, agent_id, event_type,
		provider, model, tokens_input, tokens_output, cost_usd, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.ProductID, input.WorkspaceID, input.TaskID, input.CycleID,
		input.AgentID, input.EventType, input.Provider, input.Model,
		input.TokensInput, input.TokensOutput, input.CostUSD, input.Metadata,
		input.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record cost event: %w", err)
	}

	// Update task actual_cost_usd if task_id provided
	if input.TaskID != nil {
		db.ExecContext(ctx,
			`UPDATE tasks SET actual_cost_usd = COALESCE(actual_cost_usd, 0) + ? WHERE id = ?`,
			input.CostUSD, *input.TaskID,
		)
	}

	// Update agent total_cost_usd if agent_id provided
	if input.AgentID != nil {
		db.ExecContext(ctx,
			`UPDATE agents SET total_cost_usd = COALESCE(total_cost_usd, 0) + ? WHERE id = ?`,
			input.CostUSD, *input.AgentID,
		)
	}

	return nil
}

// GetProductCosts retrieves cost events for a product with optional time range.
func GetProductCosts(ctx context.Context, db *DB, productID string, from, to *time.Time) ([]CostEvent, error) {
	query := `SELECT id, product_id, workspace_id, task_id, cycle_id, agent_id, event_type,
		provider, model, tokens_input, tokens_output, cost_usd, metadata, created_at
		FROM cost_events WHERE product_id = ?`
	args := []interface{}{productID}

	if from != nil {
		query += " AND created_at >= ?"
		args = append(args, from.Format(time.RFC3339))
	}
	if to != nil {
		query += " AND created_at <= ?"
		args = append(args, to.Format(time.RFC3339))
	}

	query += " ORDER BY created_at DESC LIMIT 500"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get product costs: %w", err)
	}
	defer rows.Close()

	var events []CostEvent
	for rows.Next() {
		var e CostEvent
		var pid, tid, cid, aid, provider, model, metadata sql.NullString

		if err := rows.Scan(&e.ID, &pid, &e.WorkspaceID, &tid, &cid, &aid,
			&e.EventType, &provider, &model, &e.TokensInput, &e.TokensOutput,
			&e.CostUSD, &metadata, &e.CreatedAt); err != nil {
			continue
		}
		if pid.Valid {
			e.ProductID = &pid.String
		}
		if tid.Valid {
			e.TaskID = &tid.String
		}
		if cid.Valid {
			e.CycleID = &cid.String
		}
		if aid.Valid {
			e.AgentID = &aid.String
		}
		if provider.Valid {
			e.Provider = &provider.String
		}
		if model.Valid {
			e.Model = &model.String
		}
		if metadata.Valid {
			e.Metadata = &metadata.String
		}
		events = append(events, e)
	}
	if events == nil {
		events = []CostEvent{}
	}
	return events, nil
}

// CreateCostCap creates a new cost cap.
func CreateCostCap(ctx context.Context, db *DB, workspaceID string, capType string, limitUSD float64, productID *string, periodStart, periodEnd *string) (*CostCap, error) {
	cap := &CostCap{
		ID:              uuid.New().String(),
		WorkspaceID:     workspaceID,
		ProductID:       productID,
		CapType:         capType,
		LimitUSD:        limitUSD,
		CurrentSpendUSD: 0,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		Status:          "active",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO cost_caps
		(id, workspace_id, product_id, cap_type, limit_usd, current_spend_usd,
		period_start, period_end, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cap.ID, cap.WorkspaceID, cap.ProductID, cap.CapType, cap.LimitUSD, cap.CurrentSpendUSD,
		cap.PeriodStart, cap.PeriodEnd, cap.Status, cap.CreatedAt.Format(time.RFC3339), cap.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create cost cap: %w", err)
	}

	return cap, nil
}

// ListCostCaps returns cost caps for a workspace (optionally filtered by product).
func ListCostCaps(ctx context.Context, db *DB, workspaceID string, productID *string) ([]CostCap, error) {
	query := `SELECT id, workspace_id, product_id, cap_type, limit_usd, current_spend_usd,
		period_start, period_end, status, created_at, updated_at
		FROM cost_caps WHERE workspace_id = ?`
	args := []interface{}{workspaceID}

	if productID != nil {
		query += " AND (product_id IS NULL OR product_id = ?)"
		args = append(args, *productID)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cost caps: %w", err)
	}
	defer rows.Close()

	var caps []CostCap
	for rows.Next() {
		var c CostCap
		var pid, periodStart, periodEnd sql.NullString

		if err := rows.Scan(&c.ID, &c.WorkspaceID, &pid, &c.CapType, &c.LimitUSD,
			&c.CurrentSpendUSD, &periodStart, &periodEnd, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		if pid.Valid {
			c.ProductID = &pid.String
		}
		if periodStart.Valid {
			c.PeriodStart = &periodStart.String
		}
		if periodEnd.Valid {
			c.PeriodEnd = &periodEnd.String
		}
		caps = append(caps, c)
	}
	if caps == nil {
		caps = []CostCap{}
	}
	return caps, nil
}

// UpdateCostCap updates a cost cap's limit, status, or current spend.
func UpdateCostCap(ctx context.Context, db *DB, capID string, limitUSD *float64, status *string) error {
	setClauses := []string{"updated_at = datetime('now')"}
	args := []interface{}{}

	if limitUSD != nil {
		setClauses = append(setClauses, "limit_usd = ?")
		args = append(args, *limitUSD)
	}
	if status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *status)
	}

	query := "UPDATE cost_caps SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = ?"
	args = append(args, capID)

	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update cost cap: %w", err)
	}
	return nil
}

// DeleteCostCap deletes a cost cap.
func DeleteCostCap(ctx context.Context, db *DB, capID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM cost_caps WHERE id = ?`, capID)
	if err != nil {
		return fmt.Errorf("delete cost cap: %w", err)
	}
	return nil
}

// CheckCaps checks all active caps and broadcasts warnings/exceedances via SSE.
func CheckCaps(ctx context.Context, db *DB) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, workspace_id, product_id, cap_type, limit_usd, current_spend_usd,
		period_start, period_end, status
		FROM cost_caps WHERE status = 'active'`,
	)
	if err != nil {
		return fmt.Errorf("check caps: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c CostCap
		var pid, periodStart, periodEnd sql.NullString

		if err := rows.Scan(&c.ID, &c.WorkspaceID, &pid, &c.CapType, &c.LimitUSD,
			&c.CurrentSpendUSD, &periodStart, &periodEnd, &c.Status); err != nil {
			continue
		}
		if pid.Valid {
			c.ProductID = &pid.String
		}

		// Calculate current spend for this period
		currentSpend := c.CurrentSpendUSD
		if periodStart.Valid && periodEnd.Valid {
			currentSpend = calculateSpendInPeriod(ctx, db, c.WorkspaceID, c.ProductID, periodStart.String, periodEnd.String)
		}

		ratio := currentSpend / c.LimitUSD

		var eventType string
		if ratio >= 1.0 {
			eventType = "cost_cap_exceeded"
			db.ExecContext(ctx,
				`UPDATE cost_caps SET status = 'exceeded', current_spend_usd = ? WHERE id = ?`,
				currentSpend, c.ID)
		} else if ratio >= 0.8 {
			eventType = "cost_cap_warning"
		}

		if eventType != "" {
			go broadcastSSEEvent(eventType, map[string]interface{}{
				"cap_id":         c.ID,
				"cap_type":       c.CapType,
				"current_spend":   currentSpend,
				"limit":          c.LimitUSD,
				"ratio":          ratio,
			})
		}
	}

	return nil
}

func calculateSpendInPeriod(ctx context.Context, db *DB, workspaceID string, productID *string, periodStart, periodEnd string) float64 {
	query := `SELECT COALESCE(SUM(cost_usd), 0) FROM cost_events
		WHERE workspace_id = ? AND created_at >= ? AND created_at <= ?`
	args := []interface{}{workspaceID, periodStart, periodEnd}

	if productID != nil {
		query += " AND product_id = ?"
		args = append(args, *productID)
	}

	var total float64
	db.QueryRowContext(ctx, query, args...).Scan(&total)
	return total
}