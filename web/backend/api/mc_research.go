package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCResearchRoutes binds research endpoints to the ServeMux.
func (h *Handler) registerMCResearchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mc/products/{id}/research/run", h.handleRunMCResearch)
	mux.HandleFunc("GET /api/mc/products/{id}/research/cycles", h.handleListMCResearchCycles)
}

type mcResearchCycle struct {
	ID            string       `json:"id"`
	ProductID     string       `json:"product_id"`
	Phase         string       `json:"phase"`
	Query         string       `json:"query"`
	Report        string       `json:"report"`
	Sources       []string     `json:"sources"`
	Variant       string       `json:"variant"`
	ParentCycleID *string      `json:"parent_cycle_id"`
	CreatedAt     string       `json:"created_at"`
	UpdatedAt     string       `json:"updated_at"`
	CompletedAt   *string      `json:"completed_at"`
}

// handleRunMCResearch runs a research cycle for a product.
//
//	POST /api/mc/products/{id}/research/run
func (h *Handler) handleRunMCResearch(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		ExistingCycleID *string `json:"existing_cycle_id,omitempty"`
		ChainIdeation   bool    `json:"chain_ideation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	cycleID, err := missioncontrol.RunResearchCycle(r.Context(), h.mcDB, productID, input.ExistingCycleID, input.ChainIdeation, h.configPath)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("run research: %v", err))
		http.Error(w, "Failed to run research", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("research_started", "Research started", nil, &cycleID)
	writeJSON(w, map[string]any{"cycle_id": cycleID})
}

// handleListMCResearchCycles lists research cycles for a product.
//
//	GET /api/mc/products/{id}/research/cycles
func (h *Handler) handleListMCResearchCycles(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	cycles, err := missioncontrol.GetResearchCycles(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list research cycles: %v", err))
		http.Error(w, "Failed to list research cycles", http.StatusInternalServerError)
		return
	}

	var result []mcResearchCycle
	for _, c := range cycles {
		var completedAt *string
		if c.CompletedAt != nil {
			s := c.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
			completedAt = &s
		}

		result = append(result, mcResearchCycle{
			ID:            c.ID,
			ProductID:     c.ProductID,
			Phase:         string(c.Phase),
			Query:         c.Query,
			Report:        c.Report,
			Sources:       c.Sources,
			Variant:       c.Variant,
			ParentCycleID: c.ParentCycleID,
			CreatedAt:     c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:     c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			CompletedAt:   completedAt,
		})
	}

	if result == nil {
		result = []mcResearchCycle{}
	}
	writeJSON(w, result)
}
