package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCIdeationRoutes binds ideation endpoints to the ServeMux.
func (h *Handler) registerMCIdeationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mc/products/{id}/ideation/run", h.handleRunMCIdeation)
	mux.HandleFunc("GET /api/mc/products/{id}/ideation/cycles", h.handleListMCIdeationCycles)
	mux.HandleFunc("POST /api/mc/ideas/{id}/swipe", h.handleSwipeIdea)
	mux.HandleFunc("GET /api/mc/products/{id}/swipe-deck", h.handleGetMCSwipeDeck)
	// Swipe deck enhancements
	mux.HandleFunc("DELETE /api/mc/products/{productID}/swipe/{swipeID}/undo", h.handleUndoSwipe)
	mux.HandleFunc("POST /api/mc/products/{id}/swipe/batch", h.handleBatchSwipe)
	mux.HandleFunc("GET /api/mc/products/{id}/swipe/history", h.handleGetSwipeHistory)
	mux.HandleFunc("GET /api/mc/products/{id}/swipe/stats", h.handleGetSwipeStats)
	// Health score
	mux.HandleFunc("GET /api/mc/products/{id}/health", h.handleGetHealthScore)
	mux.HandleFunc("PUT /api/mc/products/{id}/health/weights", h.handleUpdateHealthWeights)
	mux.HandleFunc("GET /api/mc/health-scores", h.handleGetAllHealthScores)
	// Ideas CRUD
	mux.HandleFunc("POST /api/mc/products/{id}/ideas", h.handleCreateIdea)
	mux.HandleFunc("GET /api/mc/products/{id}/ideas", h.handleListIdeas)
	mux.HandleFunc("GET /api/mc/products/{id}/ideas/pending", h.handleGetPendingIdeas)
	mux.HandleFunc("GET /api/mc/ideas/{ideaId}", h.handleGetIdea)
	mux.HandleFunc("PATCH /api/mc/ideas/{ideaId}", h.handleUpdateIdea)
	// Maybe pool
	mux.HandleFunc("GET /api/mc/products/{id}/maybe", h.handleGetMaybePool)
	mux.HandleFunc("POST /api/mc/products/{id}/maybe/resurface", h.handleResurfaceIdea)
	// Activity log
	mux.HandleFunc("GET /api/mc/products/{id}/activity", h.handleGetActivityLog)
	// A/B testing
	mux.HandleFunc("GET /api/mc/products/{id}/ab-tests", h.handleListABTests)
	mux.HandleFunc("POST /api/mc/products/{id}/ab-tests", h.handleStartABTest)
	mux.HandleFunc("GET /api/mc/products/{id}/ab-tests/{testId}", h.handleGetABTest)
	mux.HandleFunc("PATCH /api/mc/products/{id}/ab-tests/{testId}/conclude", h.handleConcludeABTest)
	mux.HandleFunc("GET /api/mc/products/{id}/ab-tests/{testId}/comparison", h.handleGetABTestComparison)
	mux.HandleFunc("POST /api/mc/products/{id}/ab-tests/{testId}/promote", h.handlePromoteABWinner)
	mux.HandleFunc("GET /api/mc/products/{id}/variants", h.handleListVariants)
	mux.HandleFunc("POST /api/mc/products/{id}/variants", h.handleCreateVariant)
	mux.HandleFunc("GET /api/mc/products/{id}/variants/{variantId}", h.handleGetVariant)
	mux.HandleFunc("PATCH /api/mc/products/{id}/variants/{variantId}", h.handleUpdateVariant)
	mux.HandleFunc("DELETE /api/mc/products/{id}/variants/{variantId}", h.handleDeleteVariant)
	// Scheduling
	mux.HandleFunc("GET /api/mc/products/{id}/schedules", h.handleListSchedules)
	mux.HandleFunc("POST /api/mc/products/{id}/schedules", h.handleCreateSchedule)
	mux.HandleFunc("PATCH /api/mc/schedules/{scheduleId}", h.handleUpdateSchedule)
	mux.HandleFunc("DELETE /api/mc/schedules/{scheduleId}", h.handleDeleteSchedule)
	// Cost tracking
	mux.HandleFunc("GET /api/mc/products/{id}/costs", h.handleGetProductCosts)
	mux.HandleFunc("GET /api/mc/costs/caps", h.handleListCostCaps)
	mux.HandleFunc("POST /api/mc/costs/caps", h.handleCreateCostCap)
	mux.HandleFunc("PATCH /api/mc/costs/caps/{capId}", h.handleUpdateCostCap)
	mux.HandleFunc("DELETE /api/mc/costs/caps/{capId}", h.handleDeleteCostCap)
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

// handleUndoSwipe undoes a swipe within the 10-second window.
//
//	DELETE /api/mc/products/{productID}/swipe/{swipeID}/undo
func (h *Handler) handleUndoSwipe(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("productID")
	swipeID := r.PathValue("swipeID")

	idea, err := missioncontrol.UndoSwipe(r.Context(), h.mcDB, productID, swipeID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Swipe not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "expired") {
			http.Error(w, "Undo window has expired", http.StatusConflict)
			return
		}
		logger.ErrorC("mc", fmt.Sprintf("undo swipe: %v", err))
		http.Error(w, "Failed to undo swipe", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"success": true, "idea": mcIdea{
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
	}})
}

// handleBatchSwipe processes multiple swipe actions in one transaction.
//
//	POST /api/mc/products/{id}/swipe/batch
func (h *Handler) handleBatchSwipe(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		Actions []missioncontrol.BatchSwipeInput `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if len(input.Actions) == 0 || len(input.Actions) > 200 {
		http.Error(w, "batch size must be 1-200", http.StatusBadRequest)
		return
	}

	err := missioncontrol.BatchSwipe(r.Context(), h.mcDB, productID, input.Actions)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("batch swipe: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true, "count": len(input.Actions)})
}

// handleGetSwipeHistory retrieves swipe history for a product.
//
//	GET /api/mc/products/{id}/swipe/history
func (h *Handler) handleGetSwipeHistory(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	history, err := missioncontrol.GetSwipeHistory(r.Context(), h.mcDB, productID, limit)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get swipe history: %v", err))
		http.Error(w, "Failed to get swipe history", http.StatusInternalServerError)
		return
	}

	type historyEntry struct {
		ID               string   `json:"id"`
		IdeaID           string   `json:"idea_id"`
		ProductID        string   `json:"product_id"`
		Action           string   `json:"action"`
		Category         string   `json:"category"`
		Tags             *string  `json:"tags,omitempty"`
		ImpactScore      *float64 `json:"impact_score,omitempty"`
		FeasibilityScore *float64 `json:"feasibility_score,omitempty"`
		Complexity       *string  `json:"complexity,omitempty"`
		UserNotes        *string  `json:"user_notes,omitempty"`
		CreatedAt        string   `json:"created_at"`
	}

	var result []historyEntry
	for _, h := range history {
		result = append(result, historyEntry{
			ID:               h.ID,
			IdeaID:           h.IdeaID,
			ProductID:        h.ProductID,
			Action:           h.Action,
			Category:         h.Category,
			Tags:             h.Tags,
			ImpactScore:      h.ImpactScore,
			FeasibilityScore: h.FeasibilityScore,
			Complexity:       h.Complexity,
			UserNotes:        h.UserNotes,
			CreatedAt:        h.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	if result == nil {
		result = []historyEntry{}
	}
	writeJSON(w, result)
}

// handleGetSwipeStats retrieves swipe statistics for a product.
//
//	GET /api/mc/products/{id}/swipe/stats
func (h *Handler) handleGetSwipeStats(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	stats, err := missioncontrol.GetSwipeStats(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get swipe stats: %v", err))
		http.Error(w, "Failed to get swipe stats", http.StatusInternalServerError)
		return
	}

	writeJSON(w, stats)
}

// handleGetHealthScore returns the full health score response for a product.
//
//	GET /api/mc/products/{id}/health
func (h *Handler) handleGetHealthScore(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	resp, err := missioncontrol.GetHealthResponse(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get health score: %v", err))
		http.Error(w, "Failed to get health score", http.StatusInternalServerError)
		return
	}

	writeJSON(w, resp)
}

// handleUpdateHealthWeights updates the health weight configuration for a product.
//
//	PUT /api/mc/products/{id}/health/weights
func (h *Handler) handleUpdateHealthWeights(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var weights missioncontrol.HealthWeightConfig
	if err := json.NewDecoder(r.Body).Decode(&weights); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	err := missioncontrol.UpdateHealthWeights(r.Context(), h.mcDB, productID, weights)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update health weights: %v", err))
		http.Error(w, "Failed to update health weights", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

// handleGetAllHealthScores returns overall scores for all active products.
//
//	GET /api/mc/health-scores
func (h *Handler) handleGetAllHealthScores(w http.ResponseWriter, r *http.Request) {
	scores, err := missioncontrol.GetAllProductScores(r.Context(), h.mcDB)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get all health scores: %v", err))
		http.Error(w, "Failed to get health scores", http.StatusInternalServerError)
		return
	}

	writeJSON(w, scores)
}

// handleCreateIdea creates a manual idea for a product.
//
//	POST /api/mc/products/{id}/ideas
func (h *Handler) handleCreateIdea(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
		ImpactScore float64 `json:"impact_score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	idea, err := missioncontrol.CreateManualIdea(r.Context(), h.mcDB, productID, missioncontrol.CreateManualIdeaInput{
		Title:       input.Title,
		Description: input.Description,
		Category:    input.Category,
		ImpactScore: input.ImpactScore,
	})
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create idea: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"id": idea.ID, "title": idea.Title, "status": idea.Status})
}

// handleListIdeas lists ideas for a product with optional filters.
//
//	GET /api/mc/products/{id}/ideas
func (h *Handler) handleListIdeas(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	opts := missioncontrol.ListIdeasOptions{
		Status:   r.URL.Query().Get("status"),
		Category: r.URL.Query().Get("category"),
		Source:   r.URL.Query().Get("source"),
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			opts.Limit = parsed
		}
	}

	ideas, err := missioncontrol.ListIdeas(r.Context(), h.mcDB, productID, opts)
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

// handleGetPendingIdeas returns pending ideas for a product.
//
//	GET /api/mc/products/{id}/ideas/pending
func (h *Handler) handleGetPendingIdeas(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	ideas, err := missioncontrol.GetPendingIdeas(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get pending ideas: %v", err))
		http.Error(w, "Failed to get pending ideas", http.StatusInternalServerError)
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

// handleGetIdea returns a single idea by ID.
//
//	GET /api/mc/ideas/{ideaId}
func (h *Handler) handleGetIdea(w http.ResponseWriter, r *http.Request) {
	ideaID := r.PathValue("ideaId")

	idea, err := missioncontrol.GetIdeaByID(r.Context(), h.mcDB, ideaID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get idea: %v", err))
		http.Error(w, "Idea not found", http.StatusNotFound)
		return
	}

	writeJSON(w, mcIdea{
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

// handleUpdateIdea updates an idea's fields.
//
//	PATCH /api/mc/ideas/{ideaId}
func (h *Handler) handleUpdateIdea(w http.ResponseWriter, r *http.Request) {
	ideaID := r.PathValue("ideaId")

	var input missioncontrol.UpdateIdeaInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	idea, err := missioncontrol.UpdateIdea(r.Context(), h.mcDB, ideaID, input)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update idea: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, mcIdea{
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

// handleGetMaybePool returns the maybe pool for a product.
//
//	GET /api/mc/products/{id}/maybe
func (h *Handler) handleGetMaybePool(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	pool, err := missioncontrol.GetMaybePool(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get maybe pool: %v", err))
		http.Error(w, "Failed to get maybe pool", http.StatusInternalServerError)
		return
	}

	writeJSON(w, pool)
}

// handleResurfaceIdea resurfaces an idea from the maybe pool.
//
//	POST /api/mc/products/{id}/maybe/resurface
func (h *Handler) handleResurfaceIdea(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		MaybePoolID string `json:"maybe_pool_id"`
		Reason      string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	idea, err := missioncontrol.ResurfaceIdea(r.Context(), h.mcDB, productID, input.MaybePoolID, input.Reason)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("resurface idea: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true, "idea_id": idea.ID})
}

// handleGetActivityLog retrieves activity entries for a product.
//
//	GET /api/mc/products/{id}/activity
func (h *Handler) handleGetActivityLog(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	entries, err := missioncontrol.GetActivityLog(r.Context(), h.mcDB, productID, limit, 0)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get activity log: %v", err))
		http.Error(w, "Failed to get activity log", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"entries": entries})
}

// handleListABTests lists A/B tests for a product.
//
//	GET /api/mc/products/{id}/ab-tests
func (h *Handler) handleListABTests(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	tests, err := missioncontrol.ListTests(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list ab tests: %v", err))
		http.Error(w, "Failed to list A/B tests", http.StatusInternalServerError)
		return
	}

	writeJSON(w, tests)
}

// handleStartABTest starts a new A/B test.
//
//	POST /api/mc/products/{id}/ab-tests
func (h *Handler) handleStartABTest(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		VariantAID string `json:"variant_a_id"`
		VariantBID string `json:"variant_b_id"`
		MinSwipes  int    `json:"min_swipes"`
		SplitMode  string `json:"split_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.MinSwipes == 0 {
		input.MinSwipes = 50
	}
	if input.SplitMode == "" {
		input.SplitMode = "concurrent"
	}

	test, err := missioncontrol.StartTest(r.Context(), h.mcDB, productID, input.VariantAID, input.VariantBID, input.MinSwipes, missioncontrol.SplitMode(input.SplitMode))
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("start ab test: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"test_id": test.ID, "status": test.Status})
}

// handleGetABTest returns a single A/B test.
//
//	GET /api/mc/products/{id}/ab-tests/{testId}
func (h *Handler) handleGetABTest(w http.ResponseWriter, r *http.Request) {
	testID := r.PathValue("testId")

	// We need to get test by ID - use ListTests and filter
	productID := r.PathValue("id")
	tests, err := missioncontrol.ListTests(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get ab test: %v", err))
		http.Error(w, "Failed to get A/B test", http.StatusInternalServerError)
		return
	}

	for _, t := range tests {
		if t.ID == testID {
			writeJSON(w, t)
			return
		}
	}
	http.Error(w, "Test not found", http.StatusNotFound)
}

// handleConcludeABTest concludes an A/B test with a winner.
//
//	PATCH /api/mc/products/{id}/ab-tests/{testId}/conclude
func (h *Handler) handleConcludeABTest(w http.ResponseWriter, r *http.Request) {
	testID := r.PathValue("testId")

	var input struct {
		WinnerVariantID string `json:"winner_variant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	err := missioncontrol.ConcludeTest(r.Context(), h.mcDB, testID, input.WinnerVariantID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("conclude ab test: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

// handleGetABTestComparison returns comparison metrics for an A/B test.
//
//	GET /api/mc/products/{id}/ab-tests/{testId}/comparison
func (h *Handler) handleGetABTestComparison(w http.ResponseWriter, r *http.Request) {
	testID := r.PathValue("testId")

	comparison, err := missioncontrol.GetTestComparison(r.Context(), h.mcDB, testID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get ab test comparison: %v", err))
		http.Error(w, "Failed to get comparison", http.StatusInternalServerError)
		return
	}

	writeJSON(w, comparison)
}

// handlePromoteABWinner promotes the winner variant to the product program.
//
//	POST /api/mc/products/{id}/ab-tests/{testId}/promote
func (h *Handler) handlePromoteABWinner(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")
	testID := r.PathValue("testId")

	// Get the test to find winner
	tests, err := missioncontrol.ListTests(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get test for promote: %v", err))
		http.Error(w, "Failed to get test", http.StatusInternalServerError)
		return
	}

	var winnerID string
	for _, t := range tests {
		if t.ID == testID && t.WinnerVariantID != "" {
			winnerID = t.WinnerVariantID
			break
		}
	}
	if winnerID == "" {
		http.Error(w, "No winner set for this test", http.StatusBadRequest)
		return
	}

	err = missioncontrol.PromoteWinner(r.Context(), h.mcDB, productID, winnerID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("promote winner: %v", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"success": true, "winner_variant_id": winnerID})
}

// handleListVariants lists program variants for a product.
//
//	GET /api/mc/products/{id}/variants
func (h *Handler) handleListVariants(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	variants, err := missioncontrol.ListVariants(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list variants: %v", err))
		http.Error(w, "Failed to list variants", http.StatusInternalServerError)
		return
	}

	writeJSON(w, variants)
}

// handleCreateVariant creates a new program variant.
//
//	POST /api/mc/products/{id}/variants
func (h *Handler) handleCreateVariant(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		Name      string `json:"name"`
		Content   string `json:"content"`
		IsControl bool   `json:"is_control"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	variant, err := missioncontrol.CreateVariant(r.Context(), h.mcDB, productID, input.Name, input.Content, input.IsControl)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create variant: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"id": variant.ID, "name": variant.Name})
}

// handleGetVariant returns a single variant.
//
//	GET /api/mc/products/{id}/variants/{variantId}
func (h *Handler) handleGetVariant(w http.ResponseWriter, r *http.Request) {
	variantID := r.PathValue("variantId")

	variant, err := missioncontrol.GetVariant(r.Context(), h.mcDB, variantID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get variant: %v", err))
		http.Error(w, "Variant not found", http.StatusNotFound)
		return
	}

	writeJSON(w, variant)
}

// handleUpdateVariant updates a variant.
//
//	PATCH /api/mc/products/{id}/variants/{variantId}
func (h *Handler) handleUpdateVariant(w http.ResponseWriter, r *http.Request) {
	variantID := r.PathValue("variantId")

	var input struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	variant, err := missioncontrol.UpdateVariant(r.Context(), h.mcDB, variantID, input.Name, input.Content)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update variant: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, variant)
}

// handleDeleteVariant deletes a variant.
//
//	DELETE /api/mc/products/{id}/variants/{variantId}
func (h *Handler) handleDeleteVariant(w http.ResponseWriter, r *http.Request) {
	variantID := r.PathValue("variantId")

	err := missioncontrol.DeleteVariant(r.Context(), h.mcDB, variantID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete variant: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

// handleListSchedules lists schedules for a product.
//
//	GET /api/mc/products/{id}/schedules
func (h *Handler) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	schedules, err := missioncontrol.ListSchedules(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list schedules: %v", err))
		http.Error(w, "Failed to list schedules", http.StatusInternalServerError)
		return
	}

	writeJSON(w, schedules)
}

// handleCreateSchedule creates a new schedule.
//
//	POST /api/mc/products/{id}/schedules
func (h *Handler) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	var input struct {
		ScheduleType  string `json:"schedule_type"`
		CronExpression string `json:"cron_expression"`
		Timezone      string `json:"timezone"`
		Config        string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.Timezone == "" {
		input.Timezone = "America/Denver"
	}

	schedule, err := missioncontrol.CreateSchedule(r.Context(), h.mcDB, productID, missioncontrol.ScheduleType(input.ScheduleType), input.CronExpression, input.Timezone, input.Config)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create schedule: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"id": schedule.ID, "schedule_type": schedule.ScheduleType})
}

// handleUpdateSchedule updates a schedule.
//
//	PATCH /api/mc/schedules/{scheduleId}
func (h *Handler) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.PathValue("scheduleId")

	var input struct {
		CronExpression *string `json:"cron_expression,omitempty"`
		Timezone      *string `json:"timezone,omitempty"`
		Enabled       *bool   `json:"enabled,omitempty"`
		Config        *string `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	err := missioncontrol.UpdateSchedule(r.Context(), h.mcDB, scheduleID, input.CronExpression, input.Timezone, input.Enabled, input.Config)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update schedule: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

// handleDeleteSchedule deletes a schedule.
//
//	DELETE /api/mc/schedules/{scheduleId}
func (h *Handler) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleID := r.PathValue("scheduleId")

	err := missioncontrol.DeleteSchedule(r.Context(), h.mcDB, scheduleID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete schedule: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

// handleGetProductCosts returns cost events for a product.
//
//	GET /api/mc/products/{id}/costs
func (h *Handler) handleGetProductCosts(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	events, err := missioncontrol.GetProductCosts(r.Context(), h.mcDB, productID, nil, nil)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get product costs: %v", err))
		http.Error(w, "Failed to get costs", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"events": events})
}

// handleListCostCaps lists cost caps for a workspace.
//
//	GET /api/mc/costs/caps
func (h *Handler) handleListCostCaps(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		http.Error(w, "workspace_id is required", http.StatusBadRequest)
		return
	}

	var productID *string
	if pid := r.URL.Query().Get("product_id"); pid != "" {
		productID = &pid
	}

	caps, err := missioncontrol.ListCostCaps(r.Context(), h.mcDB, workspaceID, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list cost caps: %v", err))
		http.Error(w, "Failed to list cost caps", http.StatusInternalServerError)
		return
	}

	writeJSON(w, caps)
}

// handleCreateCostCap creates a new cost cap.
//
//	POST /api/mc/costs/caps
func (h *Handler) handleCreateCostCap(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkspaceID string  `json:"workspace_id"`
		ProductID   *string `json:"product_id,omitempty"`
		CapType     string  `json:"cap_type"`
		LimitUSD    float64 `json:"limit_usd"`
		PeriodStart *string `json:"period_start,omitempty"`
		PeriodEnd   *string `json:"period_end,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	cap, err := missioncontrol.CreateCostCap(r.Context(), h.mcDB, input.WorkspaceID, input.CapType, input.LimitUSD, input.ProductID, input.PeriodStart, input.PeriodEnd)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create cost cap: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"id": cap.ID, "cap_type": cap.CapType})
}

// handleUpdateCostCap updates a cost cap.
//
//	PATCH /api/mc/costs/caps/{capId}
func (h *Handler) handleUpdateCostCap(w http.ResponseWriter, r *http.Request) {
	capID := r.PathValue("capId")

	var input struct {
		LimitUSD *float64 `json:"limit_usd,omitempty"`
		Status   *string  `json:"status,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	err := missioncontrol.UpdateCostCap(r.Context(), h.mcDB, capID, input.LimitUSD, input.Status)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update cost cap: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

// handleDeleteCostCap deletes a cost cap.
//
//	DELETE /api/mc/costs/caps/{capId}
func (h *Handler) handleDeleteCostCap(w http.ResponseWriter, r *http.Request) {
	capID := r.PathValue("capId")

	err := missioncontrol.DeleteCostCap(r.Context(), h.mcDB, capID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete cost cap: %v", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}
