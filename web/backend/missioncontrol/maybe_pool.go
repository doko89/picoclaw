// Package missioncontrol provides maybe pool management.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaybePoolEntry represents an idea in the maybe pool for re-evaluation.
type MaybePoolEntry struct {
	ID              string     `json:"id"`
	IdeaID          string     `json:"idea_id"`
	ProductID       string     `json:"product_id"`
	LastEvaluatedAt *time.Time `json:"last_evaluated_at,omitempty"`
	NextEvaluateAt  *time.Time `json:"next_evaluate_at,omitempty"`
	EvaluationCount int        `json:"evaluation_count"`
	EvaluationNotes *string    `json:"evaluation_notes,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// MaybePoolItem joins maybe_pool with idea data.
type MaybePoolItem struct {
	MaybePoolEntry
	IdeaTitle       string  `json:"idea_title"`
	IdeaDescription string  `json:"idea_description"`
	IdeaCategory    string  `json:"idea_category"`
	IdeaPriority    float64 `json:"idea_priority"`
}

// GetMaybePool returns all entries in the maybe pool for a product.
func GetMaybePool(ctx context.Context, db *DB, productID string) ([]MaybePoolItem, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT m.id, m.idea_id, m.product_id, m.last_evaluated_at, m.next_evaluate_at,
		m.evaluation_count, m.evaluation_notes, m.created_at,
		i.title, i.description, i.category, COALESCE(i.priority, 0)
		FROM maybe_pool m
		JOIN ideas i ON i.id = m.idea_id
		WHERE m.product_id = ?
		ORDER BY m.next_evaluate_at ASC NULLS FIRST, m.created_at DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("get maybe pool: %w", err)
	}
	defer rows.Close()

	var items []MaybePoolItem
	for rows.Next() {
		var m MaybePoolItem
		var lastEval, nextEval sql.NullString
		var evalNotes sql.NullString

		err := rows.Scan(&m.ID, &m.IdeaID, &m.ProductID, &lastEval, &nextEval,
			&m.EvaluationCount, &evalNotes, &m.CreatedAt,
			&m.IdeaTitle, &m.IdeaDescription, &m.IdeaCategory, &m.IdeaPriority)
		if err != nil {
			continue
		}
		if lastEval.Valid {
			t, _ := time.Parse(time.RFC3339, lastEval.String)
			m.LastEvaluatedAt = &t
		}
		if nextEval.Valid {
			t, _ := time.Parse(time.RFC3339, nextEval.String)
			m.NextEvaluateAt = &t
		}
		if evalNotes.Valid {
			m.EvaluationNotes = &evalNotes.String
		}
		items = append(items, m)
	}
	if items == nil {
		items = []MaybePoolItem{}
	}
	return items, nil
}

// ResurfaceIdea clones an idea from the maybe pool back into the pipeline.
func ResurfaceIdea(ctx context.Context, db *DB, productID, maybePoolID, reason string) (*Idea, error) {
	// Get the original idea
	var ideaID string
	err := db.QueryRowContext(ctx,
		`SELECT idea_id FROM maybe_pool WHERE id = ? AND product_id = ?`,
		maybePoolID, productID,
	).Scan(&ideaID)
	if err != nil {
		return nil, fmt.Errorf("find maybe pool entry: %w", err)
	}

	// Fetch original idea
	var orig Idea
	err = db.QueryRowContext(ctx,
		`SELECT id, product_id, title, description, category, priority, source, status, suppressed, created_at, updated_at
		FROM ideas WHERE id = ?`,
		ideaID,
	).Scan(&orig.ID, &orig.ProductID, &orig.Title, &orig.Description,
		&orig.Category, &orig.Priority, &orig.Source, &orig.Status, &orig.Suppressed,
		&orig.CreatedAt, &orig.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetch original idea: %w", err)
	}

	// Create resurfaced clone
	newIdea := &Idea{
		ID:              uuid.New().String(),
		ProductID:       productID,
		Title:           orig.Title,
		Description:     orig.Description,
		Category:        orig.Category,
		Priority:        orig.Priority,
		Source:          "resurfaced",
		Status:          "pending",
		Suppressed:      false,
		ResurfacedFrom:  &ideaID,
		ResurfacedReason: &reason,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO ideas
		(id, product_id, title, description, category, priority, source, status, suppressed,
		resurfaced_from, resurfaced_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newIdea.ID, newIdea.ProductID, newIdea.Title, newIdea.Description,
		newIdea.Category, newIdea.Priority, newIdea.Source, newIdea.Status, newIdea.Suppressed,
		ideaID, reason, newIdea.CreatedAt.Format(time.RFC3339), newIdea.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create resurfaced idea: %w", err)
	}

	// Delete from maybe pool
	_, err = db.ExecContext(ctx, `DELETE FROM maybe_pool WHERE id = ?`, maybePoolID)
	if err != nil {
		return nil, fmt.Errorf("remove from maybe pool: %w", err)
	}

	return newIdea, nil
}

// EvaluateMaybePool re-evaluates all due entries in the maybe pool.
func EvaluateMaybePool(ctx context.Context, db *DB, productID string) (int, error) {
	// Get all entries that are due (next_evaluate_at <= now or NULL)
	rows, err := db.QueryContext(ctx,
		`SELECT m.id, m.idea_id, m.evaluation_count, m.created_at
		FROM maybe_pool m
		WHERE m.product_id = ?
		AND (m.next_evaluate_at IS NULL OR m.next_evaluate_at <= datetime('now'))`,
		productID,
	)
	if err != nil {
		return 0, fmt.Errorf("find due entries: %w", err)
	}
	defer rows.Close()

	resurfaced := 0
	for rows.Next() {
		var id, ideaID string
		var evalCount int
		var createdAt time.Time
		if err := rows.Scan(&id, &ideaID, &evalCount, &createdAt); err != nil {
			continue
		}

		age := time.Since(createdAt).Hours() / 24

		// Resurface after 2 evaluations OR age > 30 days
		if evalCount >= 2 || age > 30 {
			_, err := ResurfaceIdea(ctx, db, productID, id, fmt.Sprintf("Automatic re-evaluation (count: %d, age: %.0f days)", evalCount, age))
			if err == nil {
				resurfaced++
			}
		} else {
			// Bump next_evaluate_at by 7 days
			_, err := db.ExecContext(ctx,
				`UPDATE maybe_pool
				SET evaluation_count = evaluation_count + 1,
				last_evaluated_at = datetime('now'),
				next_evaluate_at = datetime('now', '+7 days')
				WHERE id = ?`,
				id,
			)
			if err != nil {
				continue
			}
		}
	}

	return resurfaced, nil
}