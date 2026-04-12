// Package missioncontrol provides activity logging and SSE broadcasting.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ActivityEntry represents one entry in the autopilot activity log.
type ActivityEntry struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	CycleID   string    `json:"cycle_id"`
	CycleType string    `json:"cycle_type"` // "research" or "ideation"
	EventType string    `json:"event_type"`
	Message   string    `json:"message"`
	Detail    *string   `json:"detail,omitempty"`
	CostUSD   *float64  `json:"cost_usd,omitempty"`
	TokensUsed *int64   `json:"tokens_used,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// EmitActivity logs an activity entry and broadcasts via SSE.
func EmitActivity(ctx context.Context, db *DB, input ActivityEntry) error {
	if input.ID == "" {
		input.ID = uuid.New().String()
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now()
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO autopilot_activity_log
		(id, product_id, cycle_id, cycle_type, event_type, message, detail, cost_usd, tokens_used, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.ProductID, input.CycleID, input.CycleType,
		input.EventType, input.Message, input.Detail, input.CostUSD, input.TokensUsed,
		input.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}

	// Broadcast via SSE (non-blocking)
	go broadcastSSEEvent("autopilot_activity", map[string]interface{}{
		"id":         input.ID,
		"product_id": input.ProductID,
		"cycle_id":   input.CycleID,
		"cycle_type": input.CycleType,
		"event_type": input.EventType,
		"message":    input.Message,
		"detail":     input.Detail,
		"cost_usd":   input.CostUSD,
		"tokens_used": input.TokensUsed,
		"created_at": input.CreatedAt.Format(time.RFC3339),
	})

	return nil
}

// GetActivityLog retrieves activity entries for a product.
func GetActivityLog(ctx context.Context, db *DB, productID string, limit, after int) ([]ActivityEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `SELECT id, product_id, cycle_id, cycle_type, event_type, message, detail, cost_usd, tokens_used, created_at
		FROM autopilot_activity_log WHERE product_id = ?`
	args := []interface{}{productID}

	if after > 0 {
		query += " AND created_at > datetime('now', '-" + fmt.Sprintf("%d seconds', %d seconds)" , after, after) + ")"
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get activity log: %w", err)
	}
	defer rows.Close()

	var entries []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		var detail sql.NullString
		var costUSD sql.NullFloat64
		var tokensUsed sql.NullInt64

		if err := rows.Scan(&e.ID, &e.ProductID, &e.CycleID, &e.CycleType,
			&e.EventType, &e.Message, &detail, &costUSD, &tokensUsed, &e.CreatedAt); err != nil {
			continue
		}
		if detail.Valid {
			e.Detail = &detail.String
		}
		if costUSD.Valid {
			e.CostUSD = &costUSD.Float64
		}
		if tokensUsed.Valid {
			e.TokensUsed = &tokensUsed.Int64
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []ActivityEntry{}
	}
	return entries, nil
}

// broadcastSSEEvent sends an event to all SSE clients.
func broadcastSSEEvent(eventType string, payload map[string]interface{}) {
	BroadcastEvent(eventType, payload)
}