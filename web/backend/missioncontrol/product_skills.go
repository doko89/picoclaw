// Package missioncontrol provides product skills management.
//
// The skills package manages skill extraction and usage tracking.
package missioncontrol

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Skill represents a capability that can be learned.
type Skill struct {
	ID              string    `json:"id"`
	ProductID       string    `json:"product_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Category        string    `json:"category"`
	Confidence      float64   `json:"confidence"`       // Bayesian confidence score
	UsageCount      int       `json:"usage_count"`
	SuccessCount    int       `json:"success_count"`
	SuccessRate     float64   `json:"success_rate"`
	Status          string    `json:"status"`          // "learning", "active", "deprecated"
	SupersedesSkill *string   `json:"supersedes_skill"` // Skill this replaces
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

const (
	SkillStatusLearning   = "learning"
	SkillStatusActive     = "active"
	SkillStatusDeprecated = "deprecated"

	// Thresholds for skill promotion
	MinUsageForPromotion   = 5
	MinSuccessRateForPromotion = 0.8
)

// ExtractSkillsFromTask extracts skills from a completed task.
// If configPath is provided, uses LLM for extraction; otherwise falls back to pattern matching.
func ExtractSkillsFromTask(ctx context.Context, db *DB, taskID string, configPath string) error {
	// Get task info
	var productID, title, description SQLNullString
	err := db.QueryRowContext(ctx,
		"SELECT product_id, title, description FROM tasks WHERE id = ?",
		taskID,
	).Scan(&productID, &title, &description)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	if !productID.Valid {
		return nil // No product, skip
	}

	skills := extractSkillsWithLLM(ctx, configPath, title.String, description.String)

	// Record extracted skills
	for _, skillName := range skills {
		_, err := db.ExecContext(ctx,
			`INSERT INTO task_skills (id, task_id, skill_name, extracted_at)
			VALUES (?, ?, ?, datetime('now'))`,
			uuid.New().String(), taskID, skillName,
		)
		if err != nil {
			continue // Skip duplicates
		}

		// Update or create product skill
		_ = updateProductSkill(ctx, db, productID.String, skillName)
	}

	return nil
}

// extractSkillsWithLLM extracts skills using LLM or falls back to pattern matching.
func extractSkillsWithLLM(ctx context.Context, configPath, title, description string) []string {
	skills := []string{}

	if configPath != "" {
		// Try LLM extraction
		llmSkills := extractSkillsUsingLLM(ctx, configPath, title, description)
		if len(llmSkills) > 0 {
			return llmSkills
		}
	}

	// Fallback to pattern matching
	if title != "" || description != "" {
		text := title + " " + description
		textLower := strings.ToLower(text)

		// Pattern matching for common skill keywords
		skillPatterns := map[string][]string{
			"API Development":     {"api", "endpoint", "rest", "graphql", "grpc", "webhook"},
			"Database Management":  {"database", "sql", "query", "migration", "postgresql", "mysql", "sqlite"},
			"Frontend Development": {"frontend", "ui", "component", "react", "vue", "angular", "css", "html"},
			"Backend Development":  {"backend", "server", "microservice", "node", "go", "python", "java"},
			"Testing":             {"test", "testing", "unit", "integration", "e2e", "jest", "cypress"},
			"Authentication":     {"auth", "authentication", "authorization", "oauth", "jwt", "session", "password"},
			"DevOps":              {"docker", "kubernetes", "ci/cd", "pipeline", "deploy", "helm"},
			"Cloud Infrastructure": {"aws", "azure", "gcp", "cloud", "serverless", "lambda"},
			"Mobile Development":  {"mobile", "ios", "android", "react native", "flutter"},
			"Data Engineering":   {"data", "pipeline", "etl", "analytics", "warehouse", "spark"},
		}

		for skillName, keywords := range skillPatterns {
			for _, keyword := range keywords {
				if strings.Contains(textLower, keyword) {
					skills = append(skills, skillName)
					break
				}
			}
		}
	}

	return skills
}

// extractSkillsUsingLLM uses LLM to extract skills from task context.
func extractSkillsUsingLLM(ctx context.Context, configPath, title, description string) []string {
	provider, model, err := GetLLMProvider(configPath, "")
	if err != nil {
		return nil
	}

	systemPrompt := `You are a skill analyst. Given a task title and description, identify the key technical skills demonstrated.

Return a JSON array of skill names. Focus on:
- Programming languages used
- Frameworks and libraries
- Infrastructure and tools
- Methodologies

Keep skills generic and reusable (e.g., "API Development" not "Built REST API with Express").

Examples:
- "API Development"
- "Database Management"
- "Frontend Development"
- "Authentication"
- "Testing"
- "DevOps"`

	prompt := fmt.Sprintf("Task: %s\n\nDescription: %s\n\nWhat skills does this task demonstrate?", title, description)

	resp, err := Complete(ctx, LLMRequest{
		Prompt:      prompt,
		System:      systemPrompt,
		MaxTokens:  1024,
		Temperature: 0.3,
		Model:       model,
	}, provider)

	if err != nil {
		return nil
	}

	// Try to parse skills from LLM response
	return parseSkillsFromLLM(resp.Content)
}

// parseSkillsFromLLM extracts skill names from LLM response.
func parseSkillsFromLLM(content string) []string {
	var skills []string

	// Try JSON array first
	if strings.HasPrefix(strings.TrimSpace(content), "[") {
		var parsed []string
		if err := json.Unmarshal([]byte(content), &parsed); err == nil {
			return parsed
		}
	}

	// Fallback: parse line by line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Remove bullet points, numbers, etc.
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "• ")
		line = strings.Trim(strings.TrimSpace(line), "\"'")

		if len(line) > 2 && len(line) < 50 {
			skills = append(skills, line)
		}
	}

	return skills
}

// updateProductSkill updates a product's skill record.
func updateProductSkill(ctx context.Context, db *DB, productID, skillName string) error {
	// Check if skill exists
	var skillID string
	var confidence float64
	var usageCount, successCount int

	err := db.QueryRowContext(ctx,
		"SELECT id, confidence, usage_count, success_count FROM product_skills WHERE product_id = ? AND name = ?",
		productID, skillName,
	).Scan(&skillID, &confidence, &usageCount, &successCount)

	if err == sql.ErrNoRows {
		// Create new skill
		skillID = uuid.New().String()
		_, err = db.ExecContext(ctx,
			`INSERT INTO product_skills
			(id, product_id, name, category, confidence, usage_count, success_count, success_rate, status, created_at, updated_at)
			VALUES (?, ?, ?, 'general', 0.5, 0, 0, 0.0, 'learning', datetime('now'), datetime('now'))`,
			skillID, productID, skillName,
		)
		return err
	}
	if err != nil {
		return err
	}

	// Update Bayesian confidence
	newConfidence := calculateBayesianConfidence(confidence, usageCount, true)

	// Check for promotion
	status := SkillStatusLearning
	if usageCount+1 >= MinUsageForPromotion && (usageCount+1 > 0 && float64(successCount+1)/float64(usageCount+1) >= MinSuccessRateForPromotion) {
		status = SkillStatusActive
	}

	successRate := 0.0
	if usageCount+1 > 0 {
		successRate = float64(successCount+1) / float64(usageCount+1)
	}

	_, err = db.ExecContext(ctx,
		`UPDATE product_skills
		SET confidence = ?, usage_count = usage_count + 1, success_count = success_count + 1,
		    success_rate = ?, status = ?, updated_at = datetime('now')
		WHERE id = ?`,
		newConfidence, successRate, status, skillID,
	)

	return err
}

// calculateBayesianConfidence updates confidence using Bayesian approach.
func calculateBayesianConfidence(currentConfidence float64, usageCount int, success bool) float64 {
	// Prior: Beta(1, 1) = uniform distribution
	alpha := 1.0
	beta := 1.0

	// Add current evidence as pseudo-observations
	alpha += float64(usageCount) * currentConfidence
	beta += float64(usageCount) * (1 - currentConfidence)

	// Add new observation
	if success {
		alpha += 1
	} else {
		beta += 1
	}

	// Posterior mean
	return alpha / (alpha + beta)
}

// ReportSkillUsage reports skill usage and success/failure.
func ReportSkillUsage(ctx context.Context, db *DB, skillID, taskID string, used bool, succeeded bool) error {
	if !used {
		return nil // Not used, nothing to report
	}

	// Get skill info
	var productID, name string
	var confidence float64
	var usageCount, successCount int

	err := db.QueryRowContext(ctx,
		"SELECT product_id, name, confidence, usage_count, success_count FROM product_skills WHERE id = ?",
		skillID,
	).Scan(&productID, &name, &confidence, &usageCount, &successCount)
	if err != nil {
		return fmt.Errorf("get skill: %w", err)
	}

	// Update statistics
	newConfidence := calculateBayesianConfidence(confidence, usageCount, succeeded)
	newSuccessCount := successCount
	if succeeded {
		newSuccessCount++
	}

	successRate := 0.0
	if usageCount+1 > 0 {
		successRate = float64(newSuccessCount) / float64(usageCount+1)
	}

	// Check for promotion/demotion
	status := SkillStatusLearning
	if usageCount+1 >= MinUsageForPromotion && successRate >= MinSuccessRateForPromotion {
		status = SkillStatusActive
	} else if successRate < 0.5 {
		status = SkillStatusDeprecated
	}

	_, err = db.ExecContext(ctx,
		`UPDATE product_skills
		SET confidence = ?, usage_count = usage_count + 1, success_count = ?,
		    success_rate = ?, status = ?, updated_at = datetime('now')
		WHERE id = ?`,
		newConfidence, newSuccessCount, successRate, status, skillID,
	)

	return err
}

// GetProductSkills retrieves skills for a product.
func GetProductSkills(ctx context.Context, db *DB, productID string) ([]Skill, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, name, category, confidence, usage_count, success_count, success_rate, status, supersedes_skill_id, created_at, updated_at
		FROM product_skills WHERE product_id = ? ORDER BY confidence DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query product skills: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		var supersedesSkill SQLNullString
		err := rows.Scan(&s.ID, &s.ProductID, &s.Name, &s.Category, &s.Confidence,
			&s.UsageCount, &s.SuccessCount, &s.SuccessRate, &s.Status, &supersedesSkill, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			continue
		}
		if supersedesSkill.Valid {
			s.SupersedesSkill = &supersedesSkill.String
		}
		skills = append(skills, s)
	}

	return skills, nil
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (
		s == substr ||
		len(s) > len(substr) && (
			s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			// Case-insensitive compare
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}


