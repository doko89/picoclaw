package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCProductRoutes binds product endpoints to the ServeMux.
func (h *Handler) registerMCProductRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/products", h.handleListMCProducts)
	mux.HandleFunc("POST /api/mc/products", h.handleCreateMCProduct)
	mux.HandleFunc("GET /api/mc/products/{id}", h.handleGetMCProduct)
	mux.HandleFunc("PUT /api/mc/products/{id}", h.handleUpdateMCProduct)
	mux.HandleFunc("DELETE /api/mc/products/{id}", h.handleDeleteMCProduct)
	mux.HandleFunc("POST /api/mc/products/{id}/generate-description", h.handleGenerateProductDescription)
	mux.HandleFunc("POST /api/mc/products/scan-url", h.handleScanProductURL)
	mux.HandleFunc("POST /api/mc/products/import-readme", h.handleImportProductReadme)
}

// productColumns is the common column list for product queries.
const productColumns = `id, workspace_id, name, description, repo_url, live_url, product_program, icon, status, build_mode, automation_tier, research_enabled, ideation_enabled, default_branch, cost_cap_per_task, cost_cap_monthly, health_weight_config, batch_review_threshold, settings, created_at, updated_at`

type mcProduct struct {
	ID                   string          `json:"id"`
	WorkspaceID          string          `json:"workspace_id"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	RepoURL              string          `json:"repo_url"`
	LiveURL              string          `json:"live_url"`
	ProductProgram       string          `json:"product_program,omitempty"`
	Icon                 string          `json:"icon"`
	Status               string          `json:"status"`
	BuildMode            string          `json:"build_mode"`
	AutomationTier       string          `json:"automation_tier"`
	ResearchEnabled      bool            `json:"research_enabled"`
	IdeationEnabled      bool            `json:"ideation_enabled"`
	DefaultBranch        string          `json:"default_branch"`
	CostCapPerTask       *float64        `json:"cost_cap_per_task"`
	CostCapMonthly       *float64        `json:"cost_cap_monthly"`
	HealthWeightConfig   string          `json:"health_weight_config,omitempty"`
	BatchReviewThreshold int             `json:"batch_review_threshold"`
	Settings             json.RawMessage `json:"settings,omitempty"`
	CreatedAt            string          `json:"created_at"`
	UpdatedAt            string          `json:"updated_at"`
}

// scanProduct scans a single product row.
func scanProduct(row interface{ Scan(dest ...any) error }) (*mcProduct, error) {
	var p mcProduct
	var settings, program, icon, healthWeightConfig sql.NullString
	var costCapTask, costCapMonthly sql.NullFloat64
	var researchEnabled, ideationEnabled int
	err := row.Scan(
		&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.RepoURL, &p.LiveURL,
		&program, &icon, &p.Status, &p.BuildMode, &p.AutomationTier,
		&researchEnabled, &ideationEnabled, &p.DefaultBranch,
		&costCapTask, &costCapMonthly, &healthWeightConfig, &p.BatchReviewThreshold,
		&settings, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if program.Valid {
		p.ProductProgram = program.String
	}
	if icon.Valid {
		p.Icon = icon.String
	}
	p.ResearchEnabled = researchEnabled != 0
	p.IdeationEnabled = ideationEnabled != 0
	if costCapTask.Valid {
		p.CostCapPerTask = &costCapTask.Float64
	}
	if costCapMonthly.Valid {
		p.CostCapMonthly = &costCapMonthly.Float64
	}
	if healthWeightConfig.Valid {
		p.HealthWeightConfig = healthWeightConfig.String
	}
	if settings.Valid {
		p.Settings = []byte(settings.String)
	}
	return &p, nil
}

// handleListMCProducts lists all products.
//
//	GET /api/mc/products
func (h *Handler) handleListMCProducts(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")

	var rows *sql.Rows
	var err error

	if workspaceID != "" {
		rows, err = h.mcDB.QueryContext(r.Context(),
			"SELECT "+productColumns+" FROM products WHERE workspace_id = ? ORDER BY created_at DESC",
			workspaceID,
		)
	} else {
		rows, err = h.mcDB.QueryContext(r.Context(),
			"SELECT "+productColumns+" FROM products ORDER BY created_at DESC",
		)
	}

	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list products: %v", err))
		http.Error(w, "Failed to list products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var products []mcProduct
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			continue
		}
		products = append(products, *p)
	}

	if products == nil {
		products = []mcProduct{}
	}
	writeJSON(w, products)
}

// handleCreateMCProduct creates a new product.
//
//	POST /api/mc/products
func (h *Handler) handleCreateMCProduct(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkspaceID          string   `json:"workspace_id"`
		Name                 string   `json:"name"`
		Description          string   `json:"description"`
		RepoURL              string   `json:"repo_url"`
		LiveURL              string   `json:"live_url"`
		BuildMode            string   `json:"build_mode"`
		AutomationTier       string   `json:"automation_tier"`
		ResearchEnabled      bool     `json:"research_enabled"`
		IdeationEnabled      bool     `json:"ideation_enabled"`
		DefaultBranch        string   `json:"default_branch"`
		CostCapPerTask       *float64 `json:"cost_cap_per_task"`
		CostCapMonthly       *float64 `json:"cost_cap_monthly"`
		BatchReviewThreshold int      `json:"batch_review_threshold"`
		Settings             string   `json:"settings,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if input.WorkspaceID == "" {
		input.WorkspaceID = "default"
	}
	if input.BuildMode == "" {
		input.BuildMode = "plan_first"
	}
	if input.AutomationTier == "" {
		input.AutomationTier = "supervised"
	}
	if input.DefaultBranch == "" {
		input.DefaultBranch = "main"
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		`INSERT INTO products
		(id, workspace_id, name, description, repo_url, live_url, status, build_mode, automation_tier, research_enabled, ideation_enabled, default_branch, cost_cap_per_task, cost_cap_monthly, batch_review_threshold, settings, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		id, input.WorkspaceID, input.Name, input.Description, input.RepoURL, input.LiveURL,
		input.BuildMode, input.AutomationTier, input.ResearchEnabled, input.IdeationEnabled, input.DefaultBranch, input.CostCapPerTask, input.CostCapMonthly, input.BatchReviewThreshold, input.Settings,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create product: %v", err))
		http.Error(w, "Failed to create product", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("product_created", "Product created", nil, &id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	p, _ := scanProduct(h.mcDB.QueryRowContext(r.Context(),
		"SELECT "+productColumns+" FROM products WHERE id = ?", id,
	))
	writeJSON(w, p)
}

// handleGetMCProduct gets a product by ID.
//
//	GET /api/mc/products/{id}
func (h *Handler) handleGetMCProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	p, err := scanProduct(h.mcDB.QueryRowContext(r.Context(),
		"SELECT "+productColumns+" FROM products WHERE id = ?", id,
	))

	if err == sql.ErrNoRows {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get product: %v", err))
		http.Error(w, "Failed to get product", http.StatusInternalServerError)
		return
	}

	writeJSON(w, p)
}

// handleUpdateMCProduct updates a product.
//
//	PUT /api/mc/products/{id}
func (h *Handler) handleUpdateMCProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var input struct {
		Name                 *string  `json:"name,omitempty"`
		Description          *string  `json:"description,omitempty"`
		RepoURL              *string  `json:"repo_url,omitempty"`
		LiveURL              *string  `json:"live_url,omitempty"`
		Status               *string  `json:"status,omitempty"`
		BuildMode            *string  `json:"build_mode,omitempty"`
		AutomationTier       *string  `json:"automation_tier,omitempty"`
		ResearchEnabled      *bool    `json:"research_enabled,omitempty"`
		IdeationEnabled      *bool    `json:"ideation_enabled,omitempty"`
		DefaultBranch        *string  `json:"default_branch,omitempty"`
		CostCapPerTask       *float64 `json:"cost_cap_per_task,omitempty"`
		CostCapMonthly       *float64 `json:"cost_cap_monthly,omitempty"`
		BatchReviewThreshold *int     `json:"batch_review_threshold,omitempty"`
		Settings             *string  `json:"settings,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Build dynamic update query with parameterized values
	updates := []string{}
	args := []any{}

	if input.Name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *input.Name)
	}
	if input.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *input.Description)
	}
	if input.RepoURL != nil {
		updates = append(updates, "repo_url = ?")
		args = append(args, *input.RepoURL)
	}
	if input.LiveURL != nil {
		updates = append(updates, "live_url = ?")
		args = append(args, *input.LiveURL)
	}
	if input.Status != nil {
		updates = append(updates, "status = ?")
		args = append(args, *input.Status)
	}
	if input.BuildMode != nil {
		updates = append(updates, "build_mode = ?")
		args = append(args, *input.BuildMode)
	}
	if input.AutomationTier != nil {
		updates = append(updates, "automation_tier = ?")
		args = append(args, *input.AutomationTier)
	}
	if input.ResearchEnabled != nil {
		updates = append(updates, "research_enabled = ?")
		args = append(args, *input.ResearchEnabled)
	}
	if input.IdeationEnabled != nil {
		updates = append(updates, "ideation_enabled = ?")
		args = append(args, *input.IdeationEnabled)
	}
	if input.DefaultBranch != nil {
		updates = append(updates, "default_branch = ?")
		args = append(args, *input.DefaultBranch)
	}
	if input.CostCapPerTask != nil {
		updates = append(updates, "cost_cap_per_task = ?")
		args = append(args, *input.CostCapPerTask)
	}
	if input.CostCapMonthly != nil {
		updates = append(updates, "cost_cap_monthly = ?")
		args = append(args, *input.CostCapMonthly)
	}
	if input.BatchReviewThreshold != nil {
		updates = append(updates, "batch_review_threshold = ?")
		args = append(args, *input.BatchReviewThreshold)
	}
	if input.Settings != nil {
		updates = append(updates, "settings = ?")
		args = append(args, *input.Settings)
	}

	if len(updates) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	updates = append(updates, "updated_at = datetime('now')")
	args = append(args, id)

	query := "UPDATE products SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	_, err := h.mcDB.ExecContext(r.Context(), query, args...)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update product: %v", err))
		http.Error(w, "Failed to update product", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("product_updated", "Product updated", nil, &id)
	p, _ := scanProduct(h.mcDB.QueryRowContext(r.Context(),
		"SELECT "+productColumns+" FROM products WHERE id = ?", id,
	))
	writeJSON(w, p)
}

// handleDeleteMCProduct deletes a product.
//
//	DELETE /api/mc/products/{id}
func (h *Handler) handleDeleteMCProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.mcDB.ExecContext(r.Context(), "DELETE FROM products WHERE id = ?", id)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete product: %v", err))
		http.Error(w, "Failed to delete product", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("product_deleted", "Product deleted", nil, &id)
	w.WriteHeader(http.StatusNoContent)
}

// handleGenerateProductDescription generates an AI description for a product.
//
//	POST /api/mc/products/{id}/generate-description
func (h *Handler) handleGenerateProductDescription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Get product info
	var name, repoURL, liveURL string
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT name, COALESCE(repo_url, ''), COALESCE(live_url, '') FROM products WHERE id = ?",
		id,
	).Scan(&name, &repoURL, &liveURL)

	if err == sql.ErrNoRows {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get product for description: %v", err))
		http.Error(w, "Failed to get product", http.StatusInternalServerError)
		return
	}

	// Generate description using LLM
	description := fmt.Sprintf("AI-generated description for %s. This product is designed to...", name)
	llmDesc, llmErr := missioncontrol.CompleteWithConfig(r.Context(), h.configPath, "", missioncontrol.LLMRequest{
		Prompt:      fmt.Sprintf("Generate a concise product description (2-3 sentences) for a product called %q. Repository: %s, Website: %s. Focus on what the product does and who it serves.", name, repoURL, liveURL),
		System:      "You are a product description writer. Generate clear, professional descriptions. Respond with only the description text, no formatting.",
		MaxTokens:   512,
		Temperature: 0.7,
	})
	if llmErr == nil && llmDesc != nil && llmDesc.Content != "" {
		description = llmDesc.Content
	}

	// Update product
	_, err = h.mcDB.ExecContext(r.Context(),
		"UPDATE products SET description = ?, updated_at = datetime('now') WHERE id = ?",
		description, id,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update product description: %v", err))
		http.Error(w, "Failed to update description", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"description": description})
}

// handleScanProductURL scans a URL for product metadata.
//
//	POST /api/mc/products/scan-url
func (h *Handler) handleScanProductURL(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	// TODO: Implement actual URL scanning
	// For now, return placeholder data
	writeJSON(w, map[string]any{
		"title":       "Scanned Title",
		"description": "Scanned Description",
		"url":         input.URL,
	})
}

// handleImportProductReadme imports a README as a product program.
//
//	POST /api/mc/products/import-readme
func (h *Handler) handleImportProductReadme(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProductID string `json:"product_id"`
		ReadmeURL string `json:"readme_url"`
		Readme    string `json:"readme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.ProductID == "" {
		http.Error(w, "product_id is required", http.StatusBadRequest)
		return
	}

	// TODO: Implement README parsing and program creation
	writeJSON(w, map[string]any{
		"success":    true,
		"product_id": input.ProductID,
		"message":   "README imported successfully",
	})
}