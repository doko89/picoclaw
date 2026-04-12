package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCIdeationRoutes binds ideation endpoints to the ServeMux.
func (h *Handler) registerMCIdeationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mc/products/{id}/ideation/run", h.handleRunMCIdeation)
	mux.HandleFunc("GET /api/mc/products/{id}/ideation/cycles", h.handleListMCIdeationCycles)
	mux.HandleFunc("GET /api/mc/products/{id}/ideas", h.handleListMCIdeas)
	mux.HandleFunc("POST /api/mc/ideas/{id}/swipe", h.handleSwipeIdea)
	mux.HandleFunc("GET /api/mc/products/{id}/swipe-deck", h.handleGetMCSwipeDeck)
}

type mcIdeationCycle struct {
	ID             string  `json:"id"`
	ProductID      string  `json:"product_id"`
	ResearchCycleID *string `json:"research_cycle_id"`
	Phase          string  `json:"phase"`
	IdeasCount     int     `json:"ideas_count"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	CompletedAt    *string `json:"completed_at"`
}

type mcIdea struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"product_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Priority    float64 `json:"priority"`
	Source      string  `json:"source"`
	Status      string  `json:"status"`
	Suppressed  bool    `json:"suppressed"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// handleRunMCIdeation runs an ideation cycle for a product.
//
//	POST /api/mc/products/{id}/ideation/run
func (h *Handler) handleRunMCIdeation(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		ResearchCycleID *string `json:"research_cycle_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	cycleID, err := missioncontrol.RunIdeationCycle(r.Context(), h.mcDB, productID, input.ResearchCycleID, h.configPath, "")
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("run ideation: %v", err))
		http.Error(w, "Failed to run ideation", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("ideation_started", "Ideation started", nil, &cycleID)
	writeJSON(w, map[string]any{"cycle_id": cycleID})
}

// handleListMCIdeationCycles lists ideation cycles for a product.
//
//	GET /api/mc/products/{id}/ideation/cycles
func (h *Handler) handleListMCIdeationCycles(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	cycles, err := missioncontrol.GetIdeationCycles(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list ideation cycles: %v", err))
		http.Error(w, "Failed to list ideation cycles", http.StatusInternalServerError)
		return
	}

	var result []mcIdeationCycle
	for _, c := range cycles {
		var completedAt *string
		if c.CompletedAt != nil {
			s := c.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
			completedAt = &s
		}

		result = append(result, mcIdeationCycle{
			ID:             c.ID,
			ProductID:      c.ProductID,
			ResearchCycleID: c.ResearchCycleID,
			Phase:          string(c.Phase),
			IdeasCount:     c.IdeasCount,
			CreatedAt:      c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:      c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			CompletedAt:    completedAt,
		})
	}

	if result == nil {
		result = []mcIdeationCycle{}
	}
	writeJSON(w, result)
}

// handleListMCIdeas lists ideas for a product.
//
//	GET /api/mc/products/{id}/ideas
func (h *Handler) handleListMCIdeas(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	ideas, err := missioncontrol.GetSwipeDeck(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list ideas: %v", err))
		http.Error(w, "Failed to list ideas", http.StatusInternalServerError)
		return
	}

	var result []mcIdea
	for _, idea := range ideas {
		result = append(result, mcIdea{
			ID:          idea.ID,
			ProductID:   idea.ProductID,
			Title:       idea.Title,
			Description: idea.Description,
			Category:    idea.Category,
			Priority:    idea.Priority,
			Source:      idea.Source,
			Status:      idea.Status,
			Suppressed:  idea.Suppressed,
			CreatedAt:   idea.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   idea.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	if result == nil {
		result = []mcIdea{}
	}
	writeJSON(w, result)
}

// handleSwipeIdea records a swipe action on an idea.
//
//	POST /api/mc/ideas/{id}/swipe
func (h *Handler) handleSwipeIdea(w http.ResponseWriter, r *http.Request) {
	ideaID := r.PathValue("id")

	var input struct {
		ProductID string `json:"product_id"`
		Action    string `json:"action"` // approve, reject, maybe
		Notes     string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.Action != "approve" && input.Action != "reject" && input.Action != "maybe" {
		http.Error(w, "action must be one of: approve, reject, maybe", http.StatusBadRequest)
		return
	}

	err := missioncontrol.RecordSwipe(r.Context(), h.mcDB, input.ProductID, ideaID, input.Action, input.Notes)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("record swipe: %v", err))
		http.Error(w, "Failed to record swipe", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("idea_swiped", "Idea swiped", nil, &ideaID)
	writeJSON(w, map[string]any{"success": true})
}

// handleGetMCSwipeDeck gets the swipe deck for a product.
//
//	GET /api/mc/products/{id}/swipe-deck
func (h *Handler) handleGetMCSwipeDeck(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	ideas, err := missioncontrol.GetSwipeDeck(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get swipe deck: %v", err))
		http.Error(w, "Failed to get swipe deck", http.StatusInternalServerError)
		return
	}

	var result []mcIdea
	for _, idea := range ideas {
		result = append(result, mcIdea{
			ID:          idea.ID,
			ProductID:   idea.ProductID,
			Title:       idea.Title,
			Description: idea.Description,
			Category:    idea.Category,
			Priority:    idea.Priority,
			Source:      idea.Source,
			Status:      idea.Status,
			Suppressed:  idea.Suppressed,
			CreatedAt:   idea.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   idea.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	if result == nil {
		result = []mcIdea{}
	}
	writeJSON(w, result)
}
