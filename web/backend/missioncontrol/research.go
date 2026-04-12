// Package missioncontrol provides research cycle management for autopilot.
//
// The research package manages automated research cycles for products.
package missioncontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ResearchPhase represents the phase of a research cycle.
type ResearchPhase string

const (
	ResearchPhaseInit       ResearchPhase = "init"
	ResearchPhaseLLMSubmitted ResearchPhase = "llm_submitted"
	ResearchPhaseLLMPolling  ResearchPhase = "llm_polling"
	ResearchPhaseReportReceived ResearchPhase = "report_received"
	ResearchPhaseCompleted   ResearchPhase = "completed"
	ResearchPhaseFailed      ResearchPhase = "failed"
)

// ResearchCycle represents a research cycle for a product.
type ResearchCycle struct {
	ID              string        `json:"id"`
	ProductID       string        `json:"product_id"`
	Phase           ResearchPhase `json:"phase"`
	Query           string        `json:"query"`
	Report          string        `json:"report"`
	Sources         []string      `json:"sources"`
	Variant         string        `json:"variant"`           // For A/B testing
	ParentCycleID   *string       `json:"parent_cycle_id"`   // For linking cycles
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
}

// RunResearchCycle runs a research cycle asynchronously.
// If configPath is provided, it will use LLM for actual research; otherwise uses placeholder.
func RunResearchCycle(ctx context.Context, db *DB, productID string, existingCycleID *string, chainIdeation bool, configPath string) (string, error) {
	cycleID := uuid.New().String()

	// Check if there's an active cycle
	if existingCycleID != nil {
		var phase string
		err := db.QueryRowContext(ctx,
			"SELECT phase FROM research_cycles WHERE id = ?",
			*existingCycleID,
		).Scan(&phase)

		if err == nil && phase != string(ResearchPhaseCompleted) && phase != string(ResearchPhaseFailed) {
			return *existingCycleID, nil // Cycle still running
		}
	}

	// Create new cycle
	_, err := db.ExecContext(ctx,
		`INSERT INTO research_cycles
		(id, product_id, phase, created_at, updated_at)
		VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		cycleID, productID, ResearchPhaseInit,
	)
	if err != nil {
		return "", fmt.Errorf("create research cycle: %w", err)
	}

	// Start async research
	go runResearchAsync(ctx, db, cycleID, productID, chainIdeation, configPath)

	return cycleID, nil
}

// runResearchAsync runs the research cycle in the background.
func runResearchAsync(ctx context.Context, db *DB, cycleID, productID string, chainIdeation bool, configPath string) {
	// Update phase to LLM submitted
	updateResearchPhase(ctx, db, cycleID, ResearchPhaseLLMSubmitted)

	// Get product context
	var name, description string
	err := db.QueryRowContext(ctx,
		"SELECT name, description FROM products WHERE id = ?",
		productID,
	).Scan(&name, &description)
	if err != nil {
		updateResearchPhase(ctx, db, cycleID, ResearchPhaseFailed)
		return
	}

	// Build research query
	query := buildResearchQuery(name, description)

	// Execute research using LLM if config available
	report, _ := executeResearch(ctx, configPath, query, name)

	// Update with report
	_, err = db.ExecContext(ctx,
		`UPDATE research_cycles
		SET query = ?, report = ?, phase = ?, updated_at = datetime('now'), completed_at = datetime('now')
		WHERE id = ?`,
		query, report, ResearchPhaseCompleted, cycleID,
	)
	if err != nil {
		updateResearchPhase(ctx, db, cycleID, ResearchPhaseFailed)
		return
	}

	// Chain to ideation if requested
	if chainIdeation {
		_, _ = RunIdeationCycle(context.Background(), db, productID, &cycleID, configPath, report)
	}
}

// executeResearch performs research using LLM or returns placeholder.
func executeResearch(ctx context.Context, configPath, query, productName string) (string, []string) {
	if configPath == "" {
		// No config - return placeholder
		return fmt.Sprintf("Research report for %s\n\nMarket analysis, competitor analysis, and opportunity assessment...", productName), nil
	}

	// Try to use LLM
	provider, model, err := GetLLMProvider(configPath, "")
	if err != nil {
		return fmt.Sprintf("Research report for %s\n\nMarket analysis, competitor analysis, and opportunity assessment...", productName), nil
	}

	systemPrompt := `You are a market research analyst. Conduct comprehensive research and provide a detailed report with:
1. Market landscape and size
2. Key competitors and their positioning
3. User needs and pain points
4. Technology trends and innovations
5. Opportunities and threats
6. Strategic recommendations

Format your response as a well-structured markdown report.`

	resp, err := Complete(ctx, LLMRequest{
		Prompt:      query,
		System:      systemPrompt,
		MaxTokens:  4096,
		Temperature: 0.7,
		Model:       model,
	}, provider)

	if err != nil {
		return fmt.Sprintf("Research report for %s\n\nMarket analysis, competitor analysis, and opportunity assessment...", productName), nil
	}

	return resp.Content, nil
}

// updateResearchPhase updates the phase of a research cycle.
func updateResearchPhase(ctx context.Context, db *DB, cycleID string, phase ResearchPhase) error {
	_, err := db.ExecContext(ctx,
		"UPDATE research_cycles SET phase = ?, updated_at = datetime('now') WHERE id = ?",
		phase, cycleID,
	)
	return err
}

// buildResearchQuery builds a research query from product context.
func buildResearchQuery(name, description string) string {
	return fmt.Sprintf("Conduct comprehensive research for: %s\n\nDescription: %s\n\nFocus on:\n1. Market landscape\n2. Competitor analysis\n3. User needs\n4. Technology trends\n5. Opportunities",
		name, description)
}

// GetResearchCycles retrieves research cycles for a product.
func GetResearchCycles(ctx context.Context, db *DB, productID string) ([]ResearchCycle, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, phase, query, report, variant, parent_cycle_id, created_at, updated_at, completed_at
		FROM research_cycles WHERE product_id = ? ORDER BY created_at DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query research cycles: %w", err)
	}
	defer rows.Close()

	var cycles []ResearchCycle
	for rows.Next() {
		var c ResearchCycle
		var parentCycleID SQLNullString
		var completedAt SQLNullTime
		err := rows.Scan(&c.ID, &c.ProductID, &c.Phase, &c.Query, &c.Report,
			&c.Variant, &parentCycleID, &c.CreatedAt, &c.UpdatedAt, &completedAt)
		if err != nil {
			continue
		}
		if parentCycleID.Valid {
			c.ParentCycleID = &parentCycleID.String
		}
		if completedAt.Valid {
			t, err := time.Parse(time.RFC3339, completedAt.Time.Format(time.RFC3339))
			if err == nil {
				c.CompletedAt = &t
			}
		}
		cycles = append(cycles, c)
	}

	return cycles, nil
}

