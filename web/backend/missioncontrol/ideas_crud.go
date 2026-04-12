// Package missioncontrol provides ideas CRUD operations for autopilot products.
package missioncontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateManualIdeaInput is the input for creating a manual idea.
type CreateManualIdeaInput struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	ImpactScore float64 `json:"impact_score,omitempty"`
	Source      string  `json:"source"` // "manual"
}

// UpdateIdeaInput is the input for updating an idea.
type UpdateIdeaInput struct {
	Title          *string `json:"title,omitempty"`
	Description    *string `json:"description,omitempty"`
	Category       *string `json:"category,omitempty"`
	Status         *string `json:"status,omitempty"`
	UserNotes      *string `json:"user_notes,omitempty"`
	ImpactScore    *float64 `json:"impact_score,omitempty"`
	FeasibilityScore *float64 `json:"feasibility_score,omitempty"`
}

// ListIdeasOptions holds filter options for listing ideas.
type ListIdeasOptions struct {
	Status   string
	Category string
	Source   string
	Limit    int
}

// ListIdeas returns ideas for a product with optional filters.
func ListIdeas(ctx context.Context, db *DB, productID string, opts ListIdeasOptions) ([]Idea, error) {
	query := `SELECT id, product_id, title, description, category, priority, source, status, suppressed, created_at, updated_at
		FROM ideas WHERE product_id = ?`
	args := []interface{}{productID}

	if opts.Status != "" {
		query += " AND status = ?"
		args = append(args, opts.Status)
	}
	if opts.Category != "" {
		query += " AND category = ?"
		args = append(args, opts.Category)
	}
	if opts.Source != "" {
		query += " AND source = ?"
		args = append(args, opts.Source)
	}

	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	} else {
		query += " LIMIT 100"
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ideas: %w", err)
	}
	defer rows.Close()

	var ideas []Idea
	for rows.Next() {
		var i Idea
		err := rows.Scan(&i.ID, &i.ProductID, &i.Title, &i.Description,
			&i.Category, &i.Priority, &i.Source, &i.Status, &i.Suppressed,
			&i.CreatedAt, &i.UpdatedAt)
		if err != nil {
			continue
		}
		ideas = append(ideas, i)
	}
	if ideas == nil {
		ideas = []Idea{}
	}
	return ideas, nil
}

// GetPendingIdeas returns pending ideas ordered by priority.
func GetPendingIdeas(ctx context.Context, db *DB, productID string) ([]Idea, error) {
	return ListIdeas(ctx, db, productID, ListIdeasOptions{Status: "pending"})
}

// GetIdeaByID returns a single idea by ID.
func GetIdeaByID(ctx context.Context, db *DB, ideaID string) (*Idea, error) {
	var i Idea
	err := db.QueryRowContext(ctx,
		`SELECT id, product_id, title, description, category, priority, source, status, suppressed, created_at, updated_at
		FROM ideas WHERE id = ?`,
		ideaID,
	).Scan(&i.ID, &i.ProductID, &i.Title, &i.Description,
		&i.Category, &i.Priority, &i.Source, &i.Status, &i.Suppressed,
		&i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get idea: %w", err)
	}
	return &i, nil
}

// CreateManualIdea inserts a manually-created idea with similarity check.
func CreateManualIdea(ctx context.Context, db *DB, productID string, input CreateManualIdeaInput) (*Idea, error) {
	// Check similarity
	similar, score := CheckSimilarity(ctx, db, productID, input.Title, input.Description)
	if similar {
		return nil, fmt.Errorf("idea is too similar to an existing one (score: %.2f)", score)
	}

	idea := &Idea{
		ID:          uuid.New().String(),
		ProductID:   productID,
		Title:       input.Title,
		Description: input.Description,
		Category:    input.Category,
		Priority:    input.ImpactScore,
		Source:      "manual",
		Status:      "pending",
		Suppressed:  false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO ideas
		(id, product_id, title, description, category, priority, source, status, suppressed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idea.ID, idea.ProductID, idea.Title, idea.Description, idea.Category,
		idea.Priority, idea.Source, idea.Status, idea.Suppressed,
		idea.CreatedAt.Format(time.RFC3339), idea.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create idea: %w", err)
	}

	return idea, nil
}

// UpdateIdea updates an idea's fields.
func UpdateIdea(ctx context.Context, db *DB, ideaID string, input UpdateIdeaInput) (*Idea, error) {
	// Build dynamic update
	setClauses := []string{"updated_at = datetime('now')"}
	args := []interface{}{}

	if input.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *input.Title)
	}
	if input.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *input.Description)
	}
	if input.Category != nil {
		setClauses = append(setClauses, "category = ?")
		args = append(args, *input.Category)
	}
	if input.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *input.Status)
	}
	if input.UserNotes != nil {
		setClauses = append(setClauses, "user_notes = ?")
		args = append(args, *input.UserNotes)
	}
	if input.ImpactScore != nil {
		setClauses = append(setClauses, "priority = ?")
		args = append(args, *input.ImpactScore)
	}
	if input.FeasibilityScore != nil {
		setClauses = append(setClauses, "feasibility_score = ?")
		args = append(args, *input.FeasibilityScore)
	}

	if len(setClauses) == 1 {
		return nil, fmt.Errorf("no fields to update")
	}

	query := "UPDATE ideas SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = ?"
	args = append(args, ideaID)

	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update idea: %w", err)
	}

	return GetIdeaByID(ctx, db, ideaID)
}