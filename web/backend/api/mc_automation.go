package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCAutomationRoutes binds automation endpoints to the ServeMux.
func (h *Handler) registerMCAutomationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/products/{id}/automation-tier", h.handleGetMCAutomationTier)
	mux.HandleFunc("PUT /api/mc/products/{id}/automation-tier", h.handleSetMCAutomationTier)
	mux.HandleFunc("GET /api/mc/products/{id}/rollbacks", h.handleListMCRollbacks)
	mux.HandleFunc("POST /api/mc/rollbacks/{id}/acknowledge", h.handleAcknowledgeRollback)
	mux.HandleFunc("GET /api/mc/products/{id}/skills", h.handleListMCSkills)
	mux.HandleFunc("POST /api/mc/tasks/{id}/extract-skills", h.handleExtractTaskSkills)
}

// handleGetMCAutomationTier gets the automation tier for a product.
//
//	GET /api/mc/products/{id}/automation-tier
func (h *Handler) handleGetMCAutomationTier(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	tier, err := missioncontrol.GetAutomationTier(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get automation tier: %v", err))
		http.Error(w, "Failed to get automation tier", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"tier": string(tier)})
}

// handleSetMCAutomationTier sets the automation tier for a product.
//
//	PUT /api/mc/products/{id}/automation-tier
func (h *Handler) handleSetMCAutomationTier(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.Tier != "supervised" && input.Tier != "semi_auto" && input.Tier != "full_auto" {
		http.Error(w, "tier must be one of: supervised, semi_auto, full_auto", http.StatusBadRequest)
		return
	}

	err := missioncontrol.SetAutomationTier(r.Context(), h.mcDB, productID, missioncontrol.AutomationTier(input.Tier))
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("set automation tier: %v", err))
		http.Error(w, "Failed to set automation tier", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("automation_tier_changed", "Automation tier changed", nil, &productID)
	writeJSON(w, map[string]any{"success": true})
}

// handleListMCRollbacks lists rollbacks for a product.
//
//	GET /api/mc/products/{id}/rollbacks
func (h *Handler) handleListMCRollbacks(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	rollbacks, err := missioncontrol.GetRollbacks(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list rollbacks: %v", err))
		http.Error(w, "Failed to list rollbacks", http.StatusInternalServerError)
		return
	}

	var result []mcRollback
	for _, r := range rollbacks {
		var ackedAt, resolvedAt *string
		if r.AckedAt != nil {
			s := r.AckedAt.Format("2006-01-02T15:04:05Z07:00")
			ackedAt = &s
		}
		if r.ResolvedAt != nil {
			s := r.ResolvedAt.Format("2006-01-02T15:04:05Z07:00")
			resolvedAt = &s
		}

		result = append(result, mcRollback{
			ID:           r.ID,
			ProductID:    r.ProductID,
			TaskID:       r.TaskID,
			TriggerType:  string(r.TriggerType),
			Details:      r.Details,
			PRURL:        r.PRURL,
			RevertPRURL:  r.RevertPRURL,
			Status:       r.Status,
			CreatedAt:    r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			AckedAt:      ackedAt,
			ResolvedAt:   resolvedAt,
		})
	}

	if result == nil {
		result = []mcRollback{}
	}
	writeJSON(w, result)
}

// handleAcknowledgeRollback acknowledges a rollback and restores automation.
//
//	POST /api/mc/rollbacks/{id}/acknowledge
func (h *Handler) handleAcknowledgeRollback(w http.ResponseWriter, r *http.Request) {
	rollbackID := r.PathValue("id")

	var input struct {
		PreviousTier string `json:"previous_tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.PreviousTier == "" {
		input.PreviousTier = "semi_auto"
	}

	err := missioncontrol.AcknowledgeRollback(r.Context(), h.mcDB, rollbackID, missioncontrol.AutomationTier(input.PreviousTier))
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("acknowledge rollback: %v", err))
		http.Error(w, "Failed to acknowledge rollback", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("rollback_acknowledged", "Rollback acknowledged", nil, &rollbackID)
	writeJSON(w, map[string]any{"success": true})
}

// handleListMCSkills lists skills for a product.
//
//	GET /api/mc/products/{id}/skills
func (h *Handler) handleListMCSkills(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	skills, err := missioncontrol.GetProductSkills(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list skills: %v", err))
		http.Error(w, "Failed to list skills", http.StatusInternalServerError)
		return
	}

	var result []mcSkill
	for _, s := range skills {
		var supersedesSkill *string
		if s.SupersedesSkill != nil {
			supersedesSkill = s.SupersedesSkill
		}

		result = append(result, mcSkill{
			ID:              s.ID,
			ProductID:       s.ProductID,
			Name:            s.Name,
			Description:     s.Description,
			Category:        s.Category,
			Confidence:      s.Confidence,
			UsageCount:      s.UsageCount,
			SuccessCount:    s.SuccessCount,
			SuccessRate:     s.SuccessRate,
			Status:          s.Status,
			SupersedesSkill:  supersedesSkill,
			CreatedAt:       s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:       s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	if result == nil {
		result = []mcSkill{}
	}
	writeJSON(w, result)
}

// handleExtractTaskSkills extracts skills from a completed task.
//
//	POST /api/mc/tasks/{id}/extract-skills
func (h *Handler) handleExtractTaskSkills(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	err := missioncontrol.ExtractSkillsFromTask(r.Context(), h.mcDB, taskID, h.configPath)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("extract skills: %v", err))
		http.Error(w, "Failed to extract skills", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("skills_extracted", "Skills extracted", nil, &taskID)
	writeJSON(w, map[string]any{"success": true})
}

type mcRollback struct {
	ID           string  `json:"id"`
	ProductID    string  `json:"product_id"`
	TaskID       string  `json:"task_id"`
	TriggerType  string  `json:"trigger_type"`
	Details      string  `json:"details"`
	PRURL        string  `json:"pr_url"`
	RevertPRURL  string  `json:"revert_pr_url"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	AckedAt      *string `json:"acked_at"`
	ResolvedAt   *string `json:"resolved_at"`
}

type mcSkill struct {
	ID              string  `json:"id"`
	ProductID       string  `json:"product_id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Category        string  `json:"category"`
	Confidence      float64 `json:"confidence"`
	UsageCount      int     `json:"usage_count"`
	SuccessCount    int     `json:"success_count"`
	SuccessRate     float64 `json:"success_rate"`
	Status          string  `json:"status"`
	SupersedesSkill *string `json:"supersedes_skill"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}
