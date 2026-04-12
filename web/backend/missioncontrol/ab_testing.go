// Package missioncontrol provides A/B testing for product program variants.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// ABTestStatus is the status of an A/B test.
type ABTestStatus string

const (
	ABTestActive    ABTestStatus = "active"
	ABTestConcluded ABTestStatus = "concluded"
	ABTestCancelled ABTestStatus = "cancelled"
)

// SplitMode is how variants are assigned.
type SplitMode string

const (
	SplitConcurrent  SplitMode = "concurrent"
	SplitAlternating SplitMode = "alternating"
)

// Variant represents a product program variant.
type Variant struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	IsControl bool      `json:"is_control"`
	CreatedAt time.Time `json:"created_at"`
}

// ABTest represents an A/B test record.
type ABTest struct {
	ID              string       `json:"id"`
	ProductID       string       `json:"product_id"`
	VariantAID      string       `json:"variant_a_id"`
	VariantBID      string       `json:"variant_b_id"`
	Status          ABTestStatus `json:"status"`
	SplitMode       SplitMode    `json:"split_mode"`
	MinSwipes       int          `json:"min_swipes"`
	LastVariantUsed string       `json:"last_variant_used,omitempty"`
	WinnerVariantID string       `json:"winner_variant_id,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	ConcludedAt     *time.Time   `json:"concluded_at,omitempty"`
}

// VariantStats holds per-variant statistics for comparison.
type VariantStats struct {
	VariantID      string  `json:"variant_id"`
	TotalSwipes    int     `json:"total_swipes"`
	Approved       int     `json:"approved"`
	Rejected       int     `json:"rejected"`
	Maybe          int     `json:"maybe"`
	ApprovalRate   float64 `json:"approval_rate"`
	BuiltCount     int     `json:"built_count"`
	CostPerBuilt    float64 `json:"cost_per_built"`
}

// TestComparison holds the full comparison result with chi-squared analysis.
type TestComparison struct {
	TestID           string                  `json:"test_id"`
	VariantA         VariantStats            `json:"variant_a"`
	VariantB         VariantStats            `json:"variant_b"`
	ChiSquared       float64                 `json:"chi_squared"`
	PValue           float64                 `json:"p_value"`
	Significance     string                  `json:"significance"` // "raw" | "ci" | "significance"
	Winner           string                  `json:"winner,omitempty"`
	RecommendedWinner string                  `json:"recommended_winner,omitempty"`
}

// CreateVariant creates a new program variant.
func CreateVariant(ctx context.Context, db *DB, productID, name, content string, isControl bool) (*Variant, error) {
	v := &Variant{
		ID:        uuid.New().String(),
		ProductID: productID,
		Name:      name,
		Content:   content,
		IsControl: isControl,
		CreatedAt: time.Now(),
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO product_program_variants (id, product_id, name, content, is_control, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		v.ID, v.ProductID, v.Name, v.Content, v.IsControl, v.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create variant: %w", err)
	}

	return v, nil
}

// GetVariant fetches a variant by ID.
func GetVariant(ctx context.Context, db *DB, variantID string) (*Variant, error) {
	var v Variant
	err := db.QueryRowContext(ctx,
		`SELECT id, product_id, name, content, is_control, created_at
		FROM product_program_variants WHERE id = ?`,
		variantID,
	).Scan(&v.ID, &v.ProductID, &v.Name, &v.Content, &v.IsControl, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get variant: %w", err)
	}
	return &v, nil
}

// ListVariants returns all variants for a product.
func ListVariants(ctx context.Context, db *DB, productID string) ([]Variant, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, name, content, is_control, created_at
		FROM product_program_variants WHERE product_id = ?
		ORDER BY is_control DESC, created_at DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()

	var variants []Variant
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Name, &v.Content, &v.IsControl, &v.CreatedAt); err != nil {
			continue
		}
		variants = append(variants, v)
	}
	if variants == nil {
		variants = []Variant{}
	}
	return variants, nil
}

// UpdateVariant updates a variant's name or content.
func UpdateVariant(ctx context.Context, db *DB, variantID, name, content string) (*Variant, error) {
	_, err := db.ExecContext(ctx,
		`UPDATE product_program_variants SET name = ?, content = ? WHERE id = ?`,
		name, content, variantID,
	)
	if err != nil {
		return nil, fmt.Errorf("update variant: %w", err)
	}
	return GetVariant(ctx, db, variantID)
}

// DeleteVariant deletes a variant (fails if used in active test).
func DeleteVariant(ctx context.Context, db *DB, variantID string) error {
	// Check if used in active test
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM product_ab_tests
		WHERE (variant_a_id = ? OR variant_b_id = ?) AND status = 'active'`,
		variantID, variantID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check variant usage: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("variant is used in an active test")
	}

	_, err = db.ExecContext(ctx, `DELETE FROM product_program_variants WHERE id = ?`, variantID)
	if err != nil {
		return fmt.Errorf("delete variant: %w", err)
	}
	return nil
}

// StartTest starts a new A/B test for a product.
func StartTest(ctx context.Context, db *DB, productID, variantAID, variantBID string, minSwipes int, splitMode SplitMode) (*ABTest, error) {
	// Enforce one active test per product
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM product_ab_tests WHERE product_id = ? AND status = 'active'`,
		productID,
	).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("check active tests: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("product already has an active test")
	}

	test := &ABTest{
		ID:        uuid.New().String(),
		ProductID: productID,
		VariantAID: variantAID,
		VariantBID: variantBID,
		Status:    ABTestActive,
		SplitMode: splitMode,
		MinSwipes: minSwipes,
		CreatedAt: time.Now(),
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO product_ab_tests
		(id, product_id, variant_a_id, variant_b_id, status, split_mode, min_swipes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		test.ID, test.ProductID, test.VariantAID, test.VariantBID,
		test.Status, test.SplitMode, test.MinSwipes, test.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("start test: %w", err)
	}

	return test, nil
}

// GetActiveTest returns the active test for a product.
func GetActiveTest(ctx context.Context, db *DB, productID string) (*ABTest, error) {
	var t ABTest
	var lastUsed sql.NullString
	var winnerID sql.NullString
	var concludedAt sql.NullString

	err := db.QueryRowContext(ctx,
		`SELECT id, product_id, variant_a_id, variant_b_id, status, split_mode, min_swipes,
		last_variant_used, winner_variant_id, created_at, concluded_at
		FROM product_ab_tests WHERE product_id = ? AND status = 'active'`,
		productID,
	).Scan(&t.ID, &t.ProductID, &t.VariantAID, &t.VariantBID, &t.Status,
		&t.SplitMode, &t.MinSwipes, &lastUsed, &winnerID, &t.CreatedAt, &concludedAt)
	if err != nil {
		return nil, fmt.Errorf("get active test: %w", err)
	}
	if lastUsed.Valid {
		t.LastVariantUsed = lastUsed.String
	}
	if winnerID.Valid {
		t.WinnerVariantID = winnerID.String
	}
	if concludedAt.Valid {
		if parsed, err := time.Parse(time.RFC3339, concludedAt.String); err == nil {
			t.ConcludedAt = &parsed
		}
	}
	return &t, nil
}

// ListTests returns all tests for a product.
func ListTests(ctx context.Context, db *DB, productID string) ([]ABTest, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, variant_a_id, variant_b_id, status, split_mode, min_swipes,
		last_variant_used, winner_variant_id, created_at, concluded_at
		FROM product_ab_tests WHERE product_id = ? ORDER BY created_at DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tests: %w", err)
	}
	defer rows.Close()

	var tests []ABTest
	for rows.Next() {
		var t ABTest
		var lastUsed, winnerID, concludedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.ProductID, &t.VariantAID, &t.VariantBID, &t.Status,
			&t.SplitMode, &t.MinSwipes, &lastUsed, &winnerID, &t.CreatedAt, &concludedAt); err != nil {
			continue
		}
		if lastUsed.Valid {
			t.LastVariantUsed = lastUsed.String
		}
		if winnerID.Valid {
			t.WinnerVariantID = winnerID.String
		}
		if concludedAt.Valid {
			if parsed, err := time.Parse(time.RFC3339, concludedAt.String); err == nil {
				t.ConcludedAt = &parsed
			}
		}
		tests = append(tests, t)
	}
	if tests == nil {
		tests = []ABTest{}
	}
	return tests, nil
}

// ConcludeTest concludes a test with a winner.
func ConcludeTest(ctx context.Context, db *DB, testID, winnerVariantID string) error {
	now := time.Now()
	_, err := db.ExecContext(ctx,
		`UPDATE product_ab_tests
		SET status = 'concluded', winner_variant_id = ?, concluded_at = ?
		WHERE id = ?`,
		winnerVariantID, now.Format(time.RFC3339), testID,
	)
	if err != nil {
		return fmt.Errorf("conclude test: %w", err)
	}
	return nil
}

// CancelTest cancels a test.
func CancelTest(ctx context.Context, db *DB, testID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE product_ab_tests SET status = 'cancelled' WHERE id = ?`,
		testID,
	)
	if err != nil {
		return fmt.Errorf("cancel test: %w", err)
	}
	return nil
}

// PromoteWinner copies the winner variant content into the product program.
func PromoteWinner(ctx context.Context, db *DB, productID, winnerVariantID string) error {
	var content string
	err := db.QueryRowContext(ctx,
		`SELECT content FROM product_program_variants WHERE id = ?`,
		winnerVariantID,
	).Scan(&content)
	if err != nil {
		return fmt.Errorf("get winner content: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`UPDATE products SET product_program = ?, updated_at = datetime('now') WHERE id = ?`,
		content, productID,
	)
	if err != nil {
		return fmt.Errorf("promote winner: %w", err)
	}
	return nil
}

// getVariantSwipeStats returns swipe statistics for a variant.
// A variant's swipes are tied to ideas that were tagged with the variant at creation.
func getVariantSwipeStats(ctx context.Context, db *DB, variantID string) (VariantStats, error) {
	stats := VariantStats{VariantID: variantID}

	// Get all ideas associated with this variant
	rows, err := db.QueryContext(ctx,
		`SELECT id FROM ideas WHERE variant_id = ?`,
		variantID,
	)
	if err != nil {
		return stats, fmt.Errorf("get variant ideas: %w", err)
	}
	defer rows.Close()

	var ideaIDs []string
	for rows.Next() {
		var ideaID string
		if err := rows.Scan(&ideaID); err != nil {
			continue
		}
		ideaIDs = append(ideaIDs, ideaID)
	}

	if len(ideaIDs) == 0 {
		return stats, nil
	}

	// Build placeholders for IN clause
	placeholders := ""
	for i := range ideaIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}

	// Get swipe counts per action
	err = db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN action='approve' OR action='fire' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='reject' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action='maybe' THEN 1 ELSE 0 END), 0)
		FROM swipe_history WHERE idea_id IN (%s)`, placeholders),
		ideaIDs,
	).Scan(&stats.TotalSwipes, &stats.Approved, &stats.Rejected, &stats.Maybe)
	if err != nil {
		return stats, fmt.Errorf("get swipe stats: %w", err)
	}

	if stats.TotalSwipes > 0 {
		stats.ApprovalRate = float64(stats.Approved) / float64(stats.TotalSwipes)
	}

	// Get built count from tasks for approved ideas
	err = db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM tasks
		WHERE idea_id IN (%s) AND status IN ('merged','built','shipped')`, placeholders),
		ideaIDs,
	).Scan(&stats.BuiltCount)
	if err != nil {
		stats.BuiltCount = 0
	}

	return stats, nil
}

// chiSquaredTest performs a 2x2 chi-squared test.
// aApproved, aRejected: variant A outcomes; bApproved, bRejected: variant B outcomes.
func chiSquaredTest(aApproved, aRejected, bApproved, bRejected int) (chiSq float64) {
	// Contingency table: [aApproved, aRejected], [bApproved, bRejected]
	// Chi-squared = sum((O-E)^2/E) for each cell
	totalA := aApproved + aRejected
	totalB := bApproved + bRejected
	totalApproved := aApproved + bApproved
	totalRejected := aRejected + bRejected
	total := totalA + totalB

	if total == 0 {
		return 0
	}

	cells := [][2]int{
		{aApproved, aRejected},
		{bApproved, bRejected},
	}
	totals := [2]int{totalA, totalB}
	outcomes := [2]int{totalApproved, totalRejected}

	for i, row := range cells {
		for j, observed := range row {
			expected := float64(totals[i] * outcomes[j]) / float64(total)
			if expected > 0 {
				diff := float64(observed) - expected
				chiSq += (diff * diff) / expected
			}
		}
	}

	return chiSq
}

// normalCDFApprox approximates the standard normal CDF using Abramowitz & Stegun formula.
func normalCDFApprox(z float64) float64 {
	if z < 0 {
		return 1 - normalCDFApprox(-z)
	}
	t := 1.0 / (1.0 + 0.2316419*z)
	poly := (((((1.330274429 * t) - 1.821255978) * t + 1.781477873) * t - 0.356563782) * t + 0.319381530) * t
	return 1.0 - (1.0 / math.Sqrt(2*math.Pi)) * math.Exp(-z*z/2) * poly
}

// chiSquaredSurvival computes P(chi2 > x) for df=1 using normal approximation.
func chiSquaredSurvival(x float64) float64 {
	if x <= 0 {
		return 1.0
	}
	// chi2(1) distribution: P(X > x) = 2 * (1 - Phi(sqrt(x)))
	z := math.Sqrt(x)
	return 2 * (1 - normalCDFApprox(z))
}

// GetTestComparison computes full comparison metrics for a test.
func GetTestComparison(ctx context.Context, db *DB, testID string) (*TestComparison, error) {
	var t ABTest
	err := db.QueryRowContext(ctx,
		`SELECT id, product_id, variant_a_id, variant_b_id, status, split_mode, min_swipes
		FROM product_ab_tests WHERE id = ?`,
		testID,
	).Scan(&t.ID, &t.ProductID, &t.VariantAID, &t.VariantBID, &t.Status,
		&t.SplitMode, &t.MinSwipes)
	if err != nil {
		return nil, fmt.Errorf("get test: %w", err)
	}

	statsA, err := getVariantSwipeStats(ctx, db, t.VariantAID)
	if err != nil {
		return nil, err
	}
	statsB, err := getVariantSwipeStats(ctx, db, t.VariantBID)
	if err != nil {
		return nil, err
	}

	chiSq := chiSquaredTest(statsA.Approved, statsA.Rejected, statsB.Approved, statsB.Rejected)
	pValue := chiSquaredSurvival(chiSq)

	significance := "raw"
	if pValue < 0.05 {
		significance = "significance"
	} else if pValue < 0.10 {
		significance = "ci"
	}

	result := &TestComparison{
		TestID:     testID,
		VariantA:   statsA,
		VariantB:   statsB,
		ChiSquared: chiSq,
		PValue:     pValue,
		Significance: significance,
	}

	// Determine winner if enough swipes
	if statsA.TotalSwipes >= t.MinSwipes && statsB.TotalSwipes >= t.MinSwipes && pValue < 0.05 {
		if statsA.ApprovalRate > statsB.ApprovalRate {
			result.Winner = t.VariantAID
			result.RecommendedWinner = t.VariantAID
		} else {
			result.Winner = t.VariantBID
			result.RecommendedWinner = t.VariantBID
		}
	}

	return result, nil
}