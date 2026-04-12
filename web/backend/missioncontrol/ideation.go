// Package missioncontrol provides ideation cycle management for autopilot.
//
// The ideation package manages automated idea generation based on research.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// IdeationPhase represents the phase of an ideation cycle.
type IdeationPhase string

const (
	IdeationPhaseInit         IdeationPhase = "init"
	IdeationPhaseGenerating   IdeationPhase = "generating"
	IdeationPhaseFiltering    IdeationPhase = "filtering"
	IdeationPhaseCompleted    IdeationPhase = "completed"
	IdeationPhaseFailed       IdeationPhase = "failed"
)

// IdeationCycle represents an ideation cycle for a product.
type IdeationCycle struct {
	ID              string          `json:"id"`
	ProductID       string          `json:"product_id"`
	ResearchCycleID *string         `json:"research_cycle_id"`
	Phase           IdeationPhase   `json:"phase"`
	IdeasCount      int             `json:"ideas_count"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

// Idea represents a generated product idea.
type Idea struct {
	ID               string    `json:"id"`
	ProductID        string    `json:"product_id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Category         string    `json:"category"`
	Priority         float64   `json:"priority"`      // Similarity score
	Source           string    `json:"source"`        // "ai" or "manual"
	Status           string    `json:"status"`        // "pending", "approved", "rejected", "maybe"
	Suppressed       bool      `json:"suppressed"`    // Auto-suppressed by similarity
	ResurfacedFrom   *string   `json:"resurfaced_from,omitempty"`
	ResurfacedReason *string   `json:"resurfaced_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RunIdeationCycle runs an ideation cycle asynchronously.
// If configPath is provided, it will use LLM for actual ideation; otherwise uses placeholder.
func RunIdeationCycle(ctx context.Context, db *DB, productID string, researchCycleID *string, configPath string, researchContext string) (string, error) {
	cycleID := uuid.New().String()

	// Create new cycle
	_, err := db.ExecContext(ctx,
		`INSERT INTO ideation_cycles
		(id, product_id, research_cycle_id, phase, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		cycleID, productID, researchCycleID, IdeationPhaseInit,
	)
	if err != nil {
		return "", fmt.Errorf("create ideation cycle: %w", err)
	}

	// Start async ideation
	go runIdeationAsync(ctx, db, cycleID, productID, researchCycleID, configPath, researchContext)

	return cycleID, nil
}

// runIdeationAsync runs the ideation cycle in the background.
func runIdeationAsync(ctx context.Context, db *DB, cycleID, productID string, researchCycleID *string, configPath string, researchContext string) {
	// Update phase to generating
	updateIdeationPhase(ctx, db, cycleID, IdeationPhaseGenerating)

	// Get research report if available and no context provided
	if researchContext == "" && researchCycleID != nil {
		err := db.QueryRowContext(ctx,
			"SELECT report FROM research_cycles WHERE id = ?",
			*researchCycleID,
		).Scan(&researchContext)
		if err != nil {
			researchContext = ""
		}
	}

	// Generate ideas using LLM if config available
	ideas := generateIdeas(ctx, configPath, productID, researchContext)

	// Check for similarity and suppress duplicates
	for i := range ideas {
		similar, _ := CheckSimilarity(ctx, db, productID, ideas[i].Title, ideas[i].Description)
		if similar {
			ideas[i].Suppressed = true
		}
	}

	// Update phase to filtering
	updateIdeationPhase(ctx, db, cycleID, IdeationPhaseFiltering)

	// Insert ideas (non-suppressed)
	ideasCount := 0
	for _, idea := range ideas {
		if !idea.Suppressed {
			_, err := db.ExecContext(ctx,
				`INSERT INTO ideas
				(id, product_id, title, description, category, priority, source, status, suppressed, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
				idea.ID, idea.ProductID, idea.Title, idea.Description, idea.Category,
				idea.Priority, idea.Source, idea.Status, idea.Suppressed,
			)
			if err == nil {
				ideasCount++
			}
		}
	}

	// Update phase to completed
	completedAt := time.Now()
	_, err := db.ExecContext(ctx,
		`UPDATE ideation_cycles
		SET ideas_count = ?, phase = ?, updated_at = datetime('now'), completed_at = ?
		WHERE id = ?`,
		ideasCount, IdeationPhaseCompleted, completedAt.Format(time.RFC3339), cycleID,
	)
	if err != nil {
		updateIdeationPhase(ctx, db, cycleID, IdeationPhaseFailed)
		return
	}

	// Broadcast SSE event
	// TODO: Broadcast ideation_completed event
}

// generateIdeas generates ideas using LLM or returns placeholder.
func generateIdeas(ctx context.Context, configPath, productID, researchContext string) []Idea {
	if configPath == "" {
		// No config - return placeholder ideas
		return []Idea{
			{
				ID:          uuid.New().String(),
				ProductID:   productID,
				Title:       "AI-Powered Feature Suggestion",
				Description: "Implement ML-based feature recommendations based on user behavior",
				Category:    "feature",
				Priority:    0.85,
				Source:      "ai",
				Status:      "pending",
			},
			{
				ID:          uuid.New().String(),
				ProductID:   productID,
				Title:       "Real-time Collaboration",
				Description: "Add WebSocket-based real-time collaboration features",
				Category:    "feature",
				Priority:    0.78,
				Source:      "ai",
				Status:      "pending",
			},
			{
				ID:          uuid.New().String(),
				ProductID:   productID,
				Title:       "Mobile App Optimization",
				Description: "Optimize the mobile experience with better touch interactions",
				Category:    "improvement",
				Priority:    0.72,
				Source:      "ai",
				Status:      "pending",
			},
		}
	}

	// Try to use LLM
	provider, model, err := GetLLMProvider(configPath, "")
	if err != nil {
		return placeholderIdeas(productID)
	}

	systemPrompt := `You are a product innovation specialist. Based on the research context provided, generate 5-10 specific, actionable product improvement ideas.

For each idea, provide:
1. Title (clear, concise name)
2. Description (what it does and why it matters)
3. Category: feature, improvement, ux, performance, integration, infrastructure, content, growth, monetization, operations, or security
4. Priority score (0.0-1.0, higher = more impactful)

Return ideas that are:
- Specific to the research context
- Feasible to implement
- Have clear user benefit
- Show innovation/not just incremental`

	prompt := "Generate product improvement ideas for this product."
	if researchContext != "" {
		prompt = fmt.Sprintf("Based on this research:\n\n%s\n\nGenerate product improvement ideas.", researchContext)
	}

	resp, err := Complete(ctx, LLMRequest{
		Prompt:      prompt,
		System:      systemPrompt,
		MaxTokens:  4096,
		Temperature: 0.8,
		Model:       model,
	}, provider)

	if err != nil {
		return placeholderIdeas(productID)
	}

	// Parse ideas from LLM response
	return parseIdeasFromLLM(ctx, productID, resp.Content)
}

// placeholderIdeas returns default ideas when no LLM is available.
func placeholderIdeas(productID string) []Idea {
	return []Idea{
		{
			ID:          uuid.New().String(),
			ProductID:   productID,
			Title:       "AI-Powered Feature Suggestion",
			Description: "Implement ML-based feature recommendations based on user behavior",
			Category:    "feature",
			Priority:    0.85,
			Source:      "ai",
			Status:      "pending",
		},
		{
			ID:          uuid.New().String(),
			ProductID:   productID,
			Title:       "Real-time Collaboration",
			Description: "Add WebSocket-based real-time collaboration features",
			Category:    "feature",
			Priority:    0.78,
			Source:      "ai",
			Status:      "pending",
		},
	}
}

// parseIdeasFromLLM parses ideas from LLM response text.
func parseIdeasFromLLM(ctx context.Context, productID, content string) []Idea {
	// Try to extract ideas from the LLM response
	// Look for structured ideas - could be markdown list, JSON, etc.
	var ideas []Idea

	// Simple parsing: look for lines that start with - or * (markdown list)
	lines := splitLines(content)
	var currentTitle, currentDesc, currentCategory string
	var currentPriority float64 = 0.5

	for _, line := range lines {
		line = trimSpace(line)

		// Detect title-like lines (markdown headers or list items)
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "**") {
			// Save previous idea if exists
			if currentTitle != "" {
				ideas = append(ideas, Idea{
					ID:          uuid.New().String(),
					ProductID:   productID,
					Title:       currentTitle,
					Description: currentDesc,
					Category:    currentCategory,
					Priority:    currentPriority,
					Source:      "ai",
					Status:      "pending",
				})
			}
			// Start new idea
			currentTitle = strings.TrimPrefix(strings.TrimPrefix(line, "## "), "**")
			currentTitle = strings.TrimSuffix(currentTitle, "**")
			currentDesc = ""
			currentCategory = "feature"
			currentPriority = 0.5
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			desc := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			if currentTitle != "" {
				currentDesc += desc + " "
			}
		} else if strings.Contains(strings.ToLower(line), "category:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				currentCategory = trimSpace(parts[1])
			}
		} else if strings.Contains(strings.ToLower(line), "priority:") || strings.Contains(strings.ToLower(line), "score:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				priorityStr := strings.TrimSpace(parts[1])
				if p, err := strconv.ParseFloat(priorityStr, 64); err == nil {
					currentPriority = p
				}
			}
		}
	}

	// Don't forget the last idea
	if currentTitle != "" {
		ideas = append(ideas, Idea{
			ID:          uuid.New().String(),
			ProductID:   productID,
			Title:       currentTitle,
			Description: currentDesc,
			Category:    currentCategory,
			Priority:    currentPriority,
			Source:      "ai",
			Status:      "pending",
		})
	}

	// If no ideas parsed, create one with the whole content
	if len(ideas) == 0 {
		ideas = append(ideas, Idea{
			ID:          uuid.New().String(),
			ProductID:   productID,
			Title:       "Product Improvement",
			Description: content,
			Category:    "feature",
			Priority:    0.5,
			Source:      "ai",
			Status:      "pending",
		})
	}

	return ideas
}

// Helper functions
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// updateIdeationPhase updates the phase of an ideation cycle.
func updateIdeationPhase(ctx context.Context, db *DB, cycleID string, phase IdeationPhase) error {
	_, err := db.ExecContext(ctx,
		"UPDATE ideation_cycles SET phase = ?, updated_at = datetime('now') WHERE id = ?",
		phase, cycleID,
	)
	return err
}

// CheckSimilarity checks if a new idea is similar to existing ones.
func CheckSimilarity(ctx context.Context, db *DB, productID, title, description string) (bool, float64) {
	// Get existing ideas for comparison
	rows, err := db.QueryContext(ctx,
		"SELECT id, title, description FROM ideas WHERE product_id = ? AND suppressed = false",
		productID,
	)
	if err != nil {
		return false, 0
	}
	defer rows.Close()

	maxSimilarity := 0.0
	const (
		AutoSuppressThreshold = 0.90
		FlagThreshold         = 0.75
	)

	for rows.Next() {
		var id, existingTitle, existingDesc string
		if err := rows.Scan(&id, &existingTitle, &existingDesc); err != nil {
			continue
		}

		// Compute similarity (simplified cosine similarity on hash vectors)
		similarity := computeCosineSimilarity(title+" "+description, existingTitle+" "+existingDesc)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
		}
	}

	return maxSimilarity >= AutoSuppressThreshold, maxSimilarity
}

// computeCosineSimilarity computes cosine similarity between two texts using feature hashing.
func computeCosineSimilarity(text1, text2 string) float64 {
	// Feature hashing (256 dimensions)
	const dim = 256

	vec1 := hashToVector(text1, dim)
	vec2 := hashToVector(text2, dim)

	// Compute cosine similarity
	dotProduct := 0.0
	norm1 := 0.0
	norm2 := 0.0

	for i := 0; i < dim; i++ {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (sqrt(norm1) * sqrt(norm2))
}

// hashToVector converts text to a hash vector using FNV-1a.
func hashToVector(text string, dim int) []float64 {
	vec := make([]float64, dim)

	// Simple word tokenization
	words := tokenize(text)
	for _, word := range words {
		// FNV-1a hash
		hash := fnv1a(word)
		idx := int(hash % uint32(dim))
		vec[idx] += 1.0
	}

	return vec
}

// fnv1a computes FNV-1a hash of a string.
func fnv1a(s string) uint32 {
	const prime = 0x01000193
	hash := uint32(0x811c9dc5)

	for _, c := range s {
		hash ^= uint32(c)
		hash *= prime
	}

	return hash
}

// sqrt computes square root using Newton's method.
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// tokenize splits text into words (lowercase).
func tokenize(text string) []string {
	// Simple tokenization - split on non-alphanumeric
	words := make([]string, 0)
	currentWord := ""

	for _, c := range text {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			currentWord += string(c)
		} else {
			if len(currentWord) > 0 {
				words = append(words, toLower(currentWord))
				currentWord = ""
			}
		}
	}
	if len(currentWord) > 0 {
		words = append(words, toLower(currentWord))
	}

	return words
}

// toLower converts a string to lowercase.
func toLower(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}

// GetSwipeDeck retrieves the swipe deck for a product.
func GetSwipeDeck(ctx context.Context, db *DB, productID string) ([]Idea, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, title, description, category, priority, source, status, suppressed, created_at, updated_at
		FROM ideas WHERE product_id = ? AND suppressed = false AND status = 'pending'
		ORDER BY priority DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query swipe deck: %w", err)
	}
	defer rows.Close()

	var ideas []Idea
	for rows.Next() {
		var idea Idea
		err := rows.Scan(&idea.ID, &idea.ProductID, &idea.Title, &idea.Description,
			&idea.Category, &idea.Priority, &idea.Source, &idea.Status, &idea.Suppressed,
			&idea.CreatedAt, &idea.UpdatedAt)
		if err != nil {
			continue
		}
		ideas = append(ideas, idea)
	}

	return ideas, nil
}

// RecordSwipe records a user's swipe action on an idea.
func RecordSwipe(ctx context.Context, db *DB, productID, ideaID, action, notes string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO swipe_history
		(id, product_id, idea_id, action, notes, created_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		uuid.New().String(), productID, ideaID, action, notes,
	)
	if err != nil {
		return fmt.Errorf("record swipe: %w", err)
	}

	// Update idea status based on action
	newStatus := "pending"
	switch action {
	case "approve":
		newStatus = "approved"
	case "reject":
		newStatus = "rejected"
	case "maybe":
		newStatus = "maybe"
	}

	_, err = db.ExecContext(ctx,
		"UPDATE ideas SET status = ?, updated_at = datetime('now') WHERE id = ?",
		newStatus, ideaID,
	)
	if err != nil {
		return fmt.Errorf("update idea status: %w", err)
	}

	// Rebuild preference model if approved/rejected
	if action == "approve" || action == "reject" {
		go rebuildPreferenceModel(context.Background(), db, productID)
	}

	return nil
}

// rebuildPreferenceModel rebuilds the preference model for a product.
func rebuildPreferenceModel(ctx context.Context, db *DB, productID string) {
	// Analyze swipe history to update preferences
	// TODO: Implement actual ML model
}

// SwipeHistory represents a swipe action recorded in the system.
type SwipeHistory struct {
	ID               string    `json:"id"`
	IdeaID           string    `json:"idea_id"`
	ProductID        string    `json:"product_id"`
	Action           string    `json:"action"`
	Category         string    `json:"category"`
	Tags             *string   `json:"tags,omitempty"`
	ImpactScore      *float64  `json:"impact_score,omitempty"`
	FeasibilityScore *float64  `json:"feasibility_score,omitempty"`
	Complexity       *string   `json:"complexity,omitempty"`
	UserNotes        *string   `json:"user_notes,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// SwipeStats holds swipe statistics for a product.
type SwipeStats struct {
	TotalSwipes   int                `json:"total_swipes"`
	ApprovalRate  float64            `json:"approval_rate"`
	PerCategory   map[string]CatStats `json:"per_category"`
}

// CatStats holds per-category swipe breakdown.
type CatStats struct {
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
	Maybe    int `json:"maybe"`
	Fire     int `json:"fire"`
}

// BatchSwipeInput represents one action in a batch swipe request.
type BatchSwipeInput struct {
	IdeaID string `json:"idea_id"`
	Action string `json:"action"` // approve, reject, maybe, fire
}

// UndoSwipe undoes a swipe action within the 10-second undo window.
// Returns the restored idea or an error if undo is not allowed.
func UndoSwipe(ctx context.Context, db *DB, productID, swipeID string) (*Idea, error) {
	const undoWindowSeconds = 10

	// Get the swipe record
	var createdAt time.Time
	var ideaID string
	err := db.QueryRowContext(ctx,
		`SELECT id, idea_id, created_at FROM swipe_history WHERE id = ? AND product_id = ?`,
		swipeID, productID,
	).Scan(&swipeID, &ideaID, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("swipe not found: %w", err)
	}

	// Check undo window
	if time.Since(createdAt) > undoWindowSeconds*time.Second {
		return nil, fmt.Errorf("undo window has expired")
	}

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Restore idea to pending
	_, err = tx.ExecContext(ctx,
		`UPDATE ideas SET status = 'pending', updated_at = datetime('now') WHERE id = ?`,
		ideaID,
	)
	if err != nil {
		return nil, fmt.Errorf("restore idea: %w", err)
	}

	// Delete from maybe_pool if present
	_, _ = tx.ExecContext(ctx, `DELETE FROM maybe_pool WHERE idea_id = ?`, ideaID)

	// Delete swipe record
	_, err = tx.ExecContext(ctx, `DELETE FROM swipe_history WHERE id = ?`, swipeID)
	if err != nil {
		return nil, fmt.Errorf("delete swipe: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	// Fetch restored idea
	var idea Idea
	err = db.QueryRowContext(ctx,
		`SELECT id, product_id, title, description, category, priority, source, status, suppressed, created_at, updated_at
		FROM ideas WHERE id = ?`, ideaID,
	).Scan(&idea.ID, &idea.ProductID, &idea.Title, &idea.Description,
		&idea.Category, &idea.Priority, &idea.Source, &idea.Status, &idea.Suppressed,
		&idea.CreatedAt, &idea.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetch restored idea: %w", err)
	}

	return &idea, nil
}

// BatchSwipe processes multiple swipe actions in a single transaction.
func BatchSwipe(ctx context.Context, db *DB, productID string, actions []BatchSwipeInput) error {
	if len(actions) == 0 || len(actions) > 200 {
		return fmt.Errorf("batch size must be 1-200")
	}

	return db.RunTx(ctx, func(tx *sql.Tx) error {
		for _, a := range actions {
			// Validate action
			if a.Action != "approve" && a.Action != "reject" && a.Action != "maybe" && a.Action != "fire" {
				return fmt.Errorf("invalid action: %s", a.Action)
			}

			// Verify idea exists and is pending
			var status string
			err := tx.QueryRowContext(ctx,
				`SELECT status FROM ideas WHERE id = ? AND product_id = ?`,
				a.IdeaID, productID,
			).Scan(&status)
			if err != nil {
				return fmt.Errorf("idea %s not found: %w", a.IdeaID, err)
			}
			if status != "pending" {
				return fmt.Errorf("idea %s is not pending (status: %s)", a.IdeaID, status)
			}

			// Record swipe
			_, err = tx.ExecContext(ctx,
				`INSERT INTO swipe_history (id, product_id, idea_id, action, notes, created_at)
				VALUES (?, ?, ?, ?, ?, datetime('now'))`,
				uuid.New().String(), productID, a.IdeaID, a.Action, "",
			)
			if err != nil {
				return fmt.Errorf("record swipe for %s: %w", a.IdeaID, err)
			}

			// Update idea status
			newStatus := "pending"
			switch a.Action {
			case "approve":
				newStatus = "approved"
			case "reject":
				newStatus = "rejected"
			case "maybe":
				newStatus = "maybe"
			case "fire":
				newStatus = "building"
			}
			_, err = tx.ExecContext(ctx,
				`UPDATE ideas SET status = ?, updated_at = datetime('now') WHERE id = ?`,
				newStatus, a.IdeaID,
			)
			if err != nil {
				return fmt.Errorf("update idea %s: %w", a.IdeaID, err)
			}

			// If maybe, add to maybe_pool
			if a.Action == "maybe" {
				_, _ = tx.ExecContext(ctx,
					`INSERT OR IGNORE INTO maybe_pool (id, idea_id, product_id, created_at)
					VALUES (?, ?, ?, datetime('now'))`,
					uuid.New().String(), a.IdeaID, productID,
				)
			}
		}
		return nil
	})
}

// GetSwipeHistory retrieves swipe history for a product, ordered newest first.
func GetSwipeHistory(ctx context.Context, db *DB, productID string, limit int) ([]SwipeHistory, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, idea_id, product_id, action, category, tags, impact_score, feasibility_score, complexity, user_notes, created_at
		FROM swipe_history WHERE product_id = ?
		ORDER BY created_at DESC LIMIT ?`,
		productID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query swipe history: %w", err)
	}
	defer rows.Close()

	var history []SwipeHistory
	for rows.Next() {
		var h SwipeHistory
		err := rows.Scan(&h.ID, &h.IdeaID, &h.ProductID, &h.Action, &h.Category,
			&h.Tags, &h.ImpactScore, &h.FeasibilityScore, &h.Complexity, &h.UserNotes, &h.CreatedAt)
		if err != nil {
			continue
		}
		history = append(history, h)
	}

	if history == nil {
		history = []SwipeHistory{}
	}
	return history, nil
}

// GetSwipeStats computes swipe statistics for a product.
func GetSwipeStats(ctx context.Context, db *DB, productID string) (*SwipeStats, error) {
	// Total counts
	var total, approved, rejected, maybe, fire int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN action='approve' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='reject' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='maybe' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='fire' THEN 1 ELSE 0 END), 0)
		FROM swipe_history WHERE product_id = ?`,
		productID,
	).Scan(&total, &approved, &rejected, &maybe, &fire)
	if err != nil {
		return nil, fmt.Errorf("query swipe totals: %w", err)
	}

	// Per-category breakdown
	rows, err := db.QueryContext(ctx,
		`SELECT COALESCE(category, 'unknown'),
			SUM(CASE WHEN action='approve' THEN 1 ELSE 0 END),
			SUM(CASE WHEN action='reject' THEN 1 ELSE 0 END),
			SUM(CASE WHEN action='maybe' THEN 1 ELSE 0 END),
			SUM(CASE WHEN action='fire' THEN 1 ELSE 0 END)
		FROM swipe_history WHERE product_id = ?
		GROUP BY category`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query per-category stats: %w", err)
	}
	defer rows.Close()

	perCategory := make(map[string]CatStats)
	for rows.Next() {
		var cat string
		var a, r, m, f int
		if err := rows.Scan(&cat, &a, &r, &m, &f); err != nil {
			continue
		}
		perCategory[cat] = CatStats{Approved: a, Rejected: r, Maybe: m, Fire: f}
	}

	approvalRate := 0.0
	if total > 0 {
		approvalRate = float64(approved+fire) / float64(total)
	}

	return &SwipeStats{
		TotalSwipes:  total,
		ApprovalRate: approvalRate,
		PerCategory:  perCategory,
	}, nil
}

// GetIdeationCycles retrieves ideation cycles for a product.
func GetIdeationCycles(ctx context.Context, db *DB, productID string) ([]IdeationCycle, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, research_cycle_id, phase, ideas_count, created_at, updated_at, completed_at
		FROM ideation_cycles WHERE product_id = ? ORDER BY created_at DESC`,
		productID,
		)
		if err != nil {
			return nil, fmt.Errorf("query ideation cycles: %w", err)
		}
		defer rows.Close()

		var cycles []IdeationCycle
		for rows.Next() {
			var c IdeationCycle
			var completedAt SQLNullTime
			var researchCycleID SQLNullString
			err := rows.Scan(&c.ID, &c.ProductID, &researchCycleID, &c.Phase, &c.IdeasCount,
				&c.CreatedAt, &c.UpdatedAt, &completedAt)
			if err != nil {
				continue
			}
			if researchCycleID.Valid {
				c.ResearchCycleID = &researchCycleID.String
			}
			if completedAt.Valid {
				c.CompletedAt = &completedAt.Time
			}
			cycles = append(cycles, c)
		}

		return cycles, nil
}
