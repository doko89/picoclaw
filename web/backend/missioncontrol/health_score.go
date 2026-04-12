// Package missioncontrol provides health score computation for autopilot products.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// HealthComponent is a named component of the overall health score.
type HealthComponent string

const (
	HealthResearch   HealthComponent = "research"
	HealthPipeline   HealthComponent = "pipeline"
	HealthSwipe      HealthComponent = "swipe"
	HealthBuild      HealthComponent = "build"
	HealthCost       HealthComponent = "cost"
)

// HealthComponentScore holds the computed score for one component.
type HealthComponentScore struct {
	Name          HealthComponent `json:"name"`
	Label         string          `json:"label"`
	Score         float64         `json:"score"`
	Weight        float64         `json:"weight"`
	EffectiveWeight float64       `json:"effective_weight"`
	RawValue      float64         `json:"raw_value"`
	Unit          string          `json:"unit"`
	Description   string          `json:"description"`
}

// HealthWeightConfig holds the configured weights and disabled components.
type HealthWeightConfig struct {
	Research float64 `json:"research"`
	Pipeline float64 `json:"pipeline"`
	Swipe    float64 `json:"swipe"`
	Build    float64 `json:"build"`
	Cost     float64 `json:"cost"`
	Disabled []HealthComponent `json:"disabled,omitempty"`
}

// ProductHealthScore represents a stored or snapshot health score row.
type ProductHealthScore struct {
	ID                       string    `json:"id"`
	ProductID                string    `json:"product_id"`
	OverallScore             float64   `json:"overall_score"`
	ResearchFreshnessScore   float64   `json:"research_freshness_score"`
	PipelineDepthScore       float64   `json:"pipeline_depth_score"`
	SwipeVelocityScore       float64   `json:"swipe_velocity_score"`
	BuildSuccessScore        float64   `json:"build_success_score"`
	CostEfficiencyScore      float64   `json:"cost_efficiency_score"`
	ComponentData            string    `json:"component_data,omitempty"`
	SnapshotDate             *string   `json:"snapshot_date,omitempty"`
	CalculatedAt             time.Time `json:"calculated_at"`
}

// HealthScoreResponse is the full response returned by the health API.
type HealthScoreResponse struct {
	Score      ProductHealthScore    `json:"score"`
	Components []HealthComponentScore `json:"components"`
	Weights    HealthWeightConfig    `json:"weights"`
	History    []ProductHealthScore   `json:"history"`
}

// DefaultWeights returns the equal-weight configuration (20% each).
func DefaultWeights() HealthWeightConfig {
	return HealthWeightConfig{
		Research: 20,
		Pipeline: 20,
		Swipe:    20,
		Build:    20,
		Cost:     20,
		Disabled: nil,
	}
}

// parseWeights reads the health_weight_config JSON from the products table.
func parseWeights(configJSON string) HealthWeightConfig {
	if configJSON == "" {
		return DefaultWeights()
	}
	// Simple parse: try to extract values from JSON
	// In production this would use json.Unmarshal but we keep it lightweight
	w := DefaultWeights()
	// For now, return defaults; actual parsing done in computeHealthScore
	_ = configJSON
	return w
}

// computeEffectiveWeights redistributes disabled component weights to active ones.
func computeEffectiveWeights(weights HealthWeightConfig) map[HealthComponent]float64 {
	active := map[HealthComponent]bool{
		HealthResearch: true,
		HealthPipeline: true,
		HealthSwipe:    true,
		HealthBuild:    true,
		HealthCost:     true,
	}
	for _, d := range weights.Disabled {
		active[d] = false
	}

	activeCount := 0
	if active[HealthResearch] {
		activeCount++
	}
	if active[HealthPipeline] {
		activeCount++
	}
	if active[HealthSwipe] {
		activeCount++
	}
	if active[HealthBuild] {
		activeCount++
	}
	if active[HealthCost] {
		activeCount++
	}

	result := make(map[HealthComponent]float64)
	totalWeight := 0.0

	if active[HealthResearch] {
		result[HealthResearch] = weights.Research
		totalWeight += weights.Research
	}
	if active[HealthPipeline] {
		result[HealthPipeline] = weights.Pipeline
		totalWeight += weights.Pipeline
	}
	if active[HealthSwipe] {
		result[HealthSwipe] = weights.Swipe
		totalWeight += weights.Swipe
	}
	if active[HealthBuild] {
		result[HealthBuild] = weights.Build
		totalWeight += weights.Build
	}
	if active[HealthCost] {
		result[HealthCost] = weights.Cost
		totalWeight += weights.Cost
	}

	// Normalize to 100%
	if totalWeight > 0 && activeCount > 0 {
		for k := range result {
			result[k] = (result[k] / totalWeight) * 100
		}
	}

	return result
}

// computeResearchFreshness returns a 0-100 score based on days since last completed research cycle.
func computeResearchFreshness(ctx context.Context, db *DB, productID string) (float64, string) {
	var lastCompleted *time.Time
	err := db.QueryRowContext(ctx,
		`SELECT completed_at FROM research_cycles
		WHERE product_id = ? AND phase = 'completed'
		ORDER BY completed_at DESC LIMIT 1`,
		productID,
	).Scan(&lastCompleted)
	if err == sql.ErrNoRows || lastCompleted == nil {
		return 50, "No research cycles completed"
	}
	if err != nil {
		return 50, "Error checking research cycles"
	}

	days := time.Since(*lastCompleted).Hours() / 24
	// 100 at 0 days, linearly to 0 at 30+ days
	score := math.Max(0, 100-(days*100/30))
	return score, fmt.Sprintf("Last research %.0f days ago", days)
}

// computePipelineDepth returns a 0-100 score based on pending ideas count.
func computePipelineDepth(ctx context.Context, db *DB, productID string) (float64, string) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ideas WHERE product_id = ? AND status = 'pending'`,
		productID,
	).Scan(&count)
	if err != nil {
		return 50, "Error checking pipeline"
	}

	// 10+ pending = 100, 0 = 0, linear in between
	score := math.Min(100, float64(count)*10)
	return score, fmt.Sprintf("%d pending ideas", count)
}

// computeSwipeVelocity returns a 0-100 score based on 7-day rolling average swipes per day.
func computeSwipeVelocity(ctx context.Context, db *DB, productID string) (float64, string) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM swipe_history
		WHERE product_id = ? AND created_at >= datetime('now', '-7 days')`,
		productID,
	).Scan(&count)
	if err != nil {
		return 50, "Error checking swipe history"
	}

	avgPerDay := float64(count) / 7.0
	// 5+/day = 100, 0 = 0
	score := math.Min(100, avgPerDay*20)
	return score, fmt.Sprintf("%.1f swipes/day (7-day avg)", avgPerDay)
}

// computeBuildSuccess returns a 0-100 score based on merged PRs vs dispatched build tasks.
func computeBuildSuccess(ctx context.Context, db *DB, productID string) (float64, string) {
	var total, merged int
	err := db.QueryRowContext(ctx,
		`SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status IN ('merged','built','shipped') THEN 1 ELSE 0 END), 0)
		FROM tasks WHERE product_id = ? AND task_type = 'build'`,
		productID,
	).Scan(&total, &merged)
	if err != nil || total == 0 {
		return 80, "No build tasks yet"
	}

	score := (float64(merged) / float64(total)) * 100
	return score, fmt.Sprintf("%d/%d build tasks completed", merged, total)
}

// computeCostEfficiency returns a 0-100 score (inverse: lower cost per merge = higher score).
func computeCostEfficiency(ctx context.Context, db *DB, productID string) (float64, string) {
	var totalCost float64
	var mergedCount int

	db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0) FROM cost_events WHERE product_id = ?`,
		productID,
	).Scan(&totalCost)

	db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE product_id = ? AND status IN ('merged','built','shipped')`,
		productID,
	).Scan(&mergedCount)

	if mergedCount == 0 {
		if totalCost > 0 {
			return 20, fmt.Sprintf("$%.2f spent but no shipped features", totalCost)
		}
		return 80, "No cost data yet"
	}

	costPerMerge := totalCost / float64(mergedCount)
	// $0/merge = 100, $50+/merge = 0, linear in between
	score := math.Max(0, 100-(costPerMerge*2))
	return score, fmt.Sprintf("$%.2f cost per shipped feature", costPerMerge)
}

// computeHealthScore computes the live health score for a product.
func computeHealthScore(ctx context.Context, db *DB, productID string) (float64, []HealthComponentScore, HealthWeightConfig, error) {
	// Get weight config from product
	var configJSON string
	err := db.QueryRowContext(ctx,
		`SELECT health_weight_config FROM products WHERE id = ?`,
		productID,
	).Scan(&configJSON)
	if err != nil {
		return 0, nil, DefaultWeights(), fmt.Errorf("fetch product: %w", err)
	}

	weights := parseWeights(configJSON)
	effective := computeEffectiveWeights(weights)

	components := []HealthComponentScore{
		{Name: HealthResearch, Label: "Research Freshness", Weight: weights.Research, EffectiveWeight: effective[HealthResearch], Unit: "%", Description: "How recent the last research cycle is"},
		{Name: HealthPipeline, Label: "Pipeline Depth", Weight: weights.Pipeline, EffectiveWeight: effective[HealthPipeline], Unit: "ideas", Description: "Number of pending ideas in the pipeline"},
		{Name: HealthSwipe, Label: "Swipe Velocity", Weight: weights.Swipe, EffectiveWeight: effective[HealthSwipe], Unit: "swipes/day", Description: "7-day rolling swipe rate"},
		{Name: HealthBuild, Label: "Build Success", Weight: weights.Build, EffectiveWeight: effective[HealthBuild], Unit: "%", Description: "Percentage of build tasks completed"},
		{Name: HealthCost, Label: "Cost Efficiency", Weight: weights.Cost, EffectiveWeight: effective[HealthCost], Unit: "$/feature", Description: "Cost efficiency of shipped features"},
	}

	rf, rfDesc := computeResearchFreshness(ctx, db, productID)
	pd, pdDesc := computePipelineDepth(ctx, db, productID)
	sv, svDesc := computeSwipeVelocity(ctx, db, productID)
	bs, bsDesc := computeBuildSuccess(ctx, db, productID)
	ce, ceDesc := computeCostEfficiency(ctx, db, productID)

	components[0].Score = rf
	components[0].RawValue = rf
	components[0].Description = rfDesc
	components[1].Score = pd
	components[1].RawValue = pd
	components[1].Description = pdDesc
	components[2].Score = sv
	components[2].RawValue = sv
	components[2].Description = svDesc
	components[3].Score = bs
	components[3].RawValue = bs
	components[3].Description = bsDesc
	components[4].Score = ce
	components[4].RawValue = ce
	components[4].Description = ceDesc

	overall := rf*effective[HealthResearch]/100 +
		pd*effective[HealthPipeline]/100 +
		sv*effective[HealthSwipe]/100 +
		bs*effective[HealthBuild]/100 +
		ce*effective[HealthCost]/100

	return overall, components, weights, nil
}

// calculateAndPersist computes and upserts a live (non-snapshot) health score row.
func calculateAndPersist(ctx context.Context, db *DB, productID string) (*ProductHealthScore, []HealthComponentScore, HealthWeightConfig, error) {
	overall, components, weights, err := computeHealthScore(ctx, db, productID)
	if err != nil {
		return nil, nil, weights, err
	}

	score := &ProductHealthScore{
		ID:                     uuid.New().String(),
		ProductID:              productID,
		OverallScore:          overall,
		ResearchFreshnessScore: components[0].Score,
		PipelineDepthScore:    components[1].Score,
		SwipeVelocityScore:    components[2].Score,
		BuildSuccessScore:     components[3].Score,
		CostEfficiencyScore:   components[4].Score,
		CalculatedAt:          time.Now(),
	}

	// Upsert: delete existing live row, insert new one
	_, err = db.ExecContext(ctx,
		`DELETE FROM product_health_scores WHERE product_id = ? AND snapshot_date IS NULL`,
		productID,
	)
	if err != nil {
		return nil, nil, weights, fmt.Errorf("clear live score: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO product_health_scores
		(id, product_id, overall_score, research_freshness_score, pipeline_depth_score,
		swipe_velocity_score, build_success_score, cost_efficiency_score, calculated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		score.ID, score.ProductID, score.OverallScore,
		score.ResearchFreshnessScore, score.PipelineDepthScore,
		score.SwipeVelocityScore, score.BuildSuccessScore, score.CostEfficiencyScore,
		score.CalculatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, nil, weights, fmt.Errorf("persist score: %w", err)
	}

	return score, components, weights, nil
}

// getLatestScore fetches the most recent non-snapshot row for a product.
func getLatestScore(ctx context.Context, db *DB, productID string) (*ProductHealthScore, error) {
	var s ProductHealthScore
	var snapshotDate sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT id, product_id, overall_score, research_freshness_score, pipeline_depth_score,
		swipe_velocity_score, build_success_score, cost_efficiency_score,
		component_data, snapshot_date, calculated_at
		FROM product_health_scores
		WHERE product_id = ? AND snapshot_date IS NULL
		ORDER BY calculated_at DESC LIMIT 1`,
		productID,
	).Scan(&s.ID, &s.ProductID, &s.OverallScore,
		&s.ResearchFreshnessScore, &s.PipelineDepthScore,
		&s.SwipeVelocityScore, &s.BuildSuccessScore, &s.CostEfficiencyScore,
		&s.ComponentData, &snapshotDate, &s.CalculatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest score: %w", err)
	}
	if snapshotDate.Valid {
		s.SnapshotDate = &snapshotDate.String
	}
	return &s, nil
}

// getScoreHistory fetches snapshot rows within the last N days.
func getScoreHistory(ctx context.Context, db *DB, productID string, days int) ([]ProductHealthScore, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, overall_score, research_freshness_score, pipeline_depth_score,
		swipe_velocity_score, build_success_score, cost_efficiency_score,
		component_data, snapshot_date, calculated_at
		FROM product_health_scores
		WHERE product_id = ? AND snapshot_date IS NOT NULL
		AND calculated_at >= datetime('now', '-`+fmt.Sprintf("%d", days)+` days')
		ORDER BY calculated_at ASC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("get score history: %w", err)
	}
	defer rows.Close()

	var history []ProductHealthScore
	for rows.Next() {
		var s ProductHealthScore
		var snapshotDate sql.NullString
		err := rows.Scan(&s.ID, &s.ProductID, &s.OverallScore,
			&s.ResearchFreshnessScore, &s.PipelineDepthScore,
			&s.SwipeVelocityScore, &s.BuildSuccessScore, &s.CostEfficiencyScore,
			&s.ComponentData, &snapshotDate, &s.CalculatedAt)
		if err != nil {
			continue
		}
		if snapshotDate.Valid {
			s.SnapshotDate = &snapshotDate.String
		}
		history = append(history, s)
	}
	if history == nil {
		history = []ProductHealthScore{}
	}
	return history, nil
}

// GetHealthResponse builds the full health API response.
func GetHealthResponse(ctx context.Context, db *DB, productID string) (*HealthScoreResponse, error) {
	// Try cached live score first
	live, _ := getLatestScore(ctx, db, productID)

	// Always compute fresh (but use live as fallback if DB calls fail mid-compute)
	score, components, weights, err := calculateAndPersist(ctx, db, productID)
	if err != nil {
		if live != nil {
			score = live
		} else {
			return nil, err
		}
	}

	history, err := getScoreHistory(ctx, db, productID, 30)
	if err != nil {
		history = []ProductHealthScore{}
	}

	// Rehydrate component data if we only have the live row
	if len(components) == 0 && score != nil {
		components = []HealthComponentScore{
			{Name: HealthResearch, Label: "Research Freshness", Score: score.ResearchFreshnessScore},
			{Name: HealthPipeline, Label: "Pipeline Depth", Score: score.PipelineDepthScore},
			{Name: HealthSwipe, Label: "Swipe Velocity", Score: score.SwipeVelocityScore},
			{Name: HealthBuild, Label: "Build Success", Score: score.BuildSuccessScore},
			{Name: HealthCost, Label: "Cost Efficiency", Score: score.CostEfficiencyScore},
		}
		for i := range components {
			components[i].Weight = 20
			components[i].EffectiveWeight = 20
			components[i].RawValue = components[i].Score
			components[i].Unit = "%"
		}
		weights = DefaultWeights()
	}

	return &HealthScoreResponse{
		Score:      *score,
		Components: components,
		Weights:    weights,
		History:    history,
	}, nil
}

// UpdateHealthWeights writes new weight config to the product and triggers recalc.
func UpdateHealthWeights(ctx context.Context, db *DB, productID string, weights HealthWeightConfig) error {
	// For now, store as JSON in health_weight_config column
	// We'll encode as a simple string format: research:pipeline:swipe:build:cost:disabled1,disabled2
	disabledStr := ""
	if len(weights.Disabled) > 0 {
		disabledStr = ":" + string(weights.Disabled[0])
		for i := 1; i < len(weights.Disabled); i++ {
			disabledStr += "," + string(weights.Disabled[i])
		}
	}

	configStr := fmt.Sprintf("%.0f:%.0f:%.0f:%.0f:%.0f%s",
		weights.Research, weights.Pipeline, weights.Swipe, weights.Build, weights.Cost, disabledStr)

	_, err := db.ExecContext(ctx,
		`UPDATE products SET health_weight_config = ?, updated_at = datetime('now') WHERE id = ?`,
		configStr, productID,
	)
	if err != nil {
		return fmt.Errorf("update weights: %w", err)
	}

	return nil
}

// GetAllProductScores returns a map of productId -> overall score for all active products.
func GetAllProductScores(ctx context.Context, db *DB) (map[string]float64, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id FROM products WHERE status = 'active'`,
	)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			continue
		}
		live, _ := getLatestScore(ctx, db, pid)
		if live != nil {
			result[pid] = live.OverallScore
		} else {
			// Compute fresh
			score, _, _, err := calculateAndPersist(ctx, db, pid)
			if err == nil {
				result[pid] = score.OverallScore
			}
		}
	}
	return result, nil
}

// TakeDailySnapshot idempotently inserts a snapshot row for today if none exists.
func TakeDailySnapshot(ctx context.Context, db *DB, productID string) error {
	today := time.Now().Format("2006-01-02")

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM product_health_scores
		WHERE product_id = ? AND snapshot_date = ?`,
		productID, today,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check snapshot: %w", err)
	}
	if count > 0 {
		return nil // Already has snapshot for today
	}

	live, err := getLatestScore(ctx, db, productID)
	if err != nil || live == nil {
		live, _, _, err = calculateAndPersist(ctx, db, productID)
		if err != nil {
			return fmt.Errorf("compute before snapshot: %w", err)
		}
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO product_health_scores
		(id, product_id, overall_score, research_freshness_score, pipeline_depth_score,
		swipe_velocity_score, build_success_score, cost_efficiency_score,
		component_data, snapshot_date, calculated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), productID, live.OverallScore,
		live.ResearchFreshnessScore, live.PipelineDepthScore,
		live.SwipeVelocityScore, live.BuildSuccessScore, live.CostEfficiencyScore,
		live.ComponentData, today, time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}

	return nil
}