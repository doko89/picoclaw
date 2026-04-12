package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCCheckpointRoutes binds checkpoint endpoints to the ServeMux.
func (h *Handler) registerMCCheckpointRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/tasks/{id}/checkpoints", h.handleListMCCheckpoints)
	mux.HandleFunc("POST /api/mc/tasks/{id}/checkpoint", h.handleCreateMCCheckpoint)
	mux.HandleFunc("POST /api/mc/tasks/{id}/checkpoint/restore", h.handleRestoreMCCheckpoint)
}

type mcCheckpoint struct {
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	AgentID        string `json:"agent_id"`
	CheckpointType string `json:"checkpoint_type"`
	StateSummary   string `json:"state_summary"`
	CreatedAt      string `json:"created_at"`
}

// handleListMCCheckpoints returns checkpoints for a task.
//
//	GET /api/mc/tasks/{id}/checkpoints
func (h *Handler) handleListMCCheckpoints(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	checkpoints, err := missioncontrol.GetCheckpoints(r.Context(), h.mcDB, taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list checkpoints: %v", err))
		http.Error(w, "Failed to list checkpoints", http.StatusInternalServerError)
		return
	}

	writeJSON(w, checkpoints)
}

// handleCreateMCCheckpoint creates a manual checkpoint for a task.
//
//	POST /api/mc/tasks/{id}/checkpoint
func (h *Handler) handleCreateMCCheckpoint(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		AgentID        string `json:"agent_id"`
		CheckpointType string `json:"checkpoint_type"`
		StateSummary   string `json:"state_summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.CheckpointType == "" {
		input.CheckpointType = "manual"
	}

	err := missioncontrol.SaveCheckpoint(r.Context(), h.mcDB, taskID, input.AgentID, input.CheckpointType, map[string]any{
		"summary": input.StateSummary,
	})
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create checkpoint: %v", err))
		http.Error(w, "Failed to create checkpoint", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("checkpoint_saved", "Checkpoint saved", &input.AgentID, &taskID)
	writeJSON(w, map[string]any{"success": true})
}

// handleRestoreMCCheckpoint restores a task from a checkpoint.
//
//	POST /api/mc/tasks/{id}/checkpoint/restore
func (h *Handler) handleRestoreMCCheckpoint(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		CheckpointID string `json:"checkpoint_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.CheckpointID == "" {
		http.Error(w, "checkpoint_id is required", http.StatusBadRequest)
		return
	}

	err := missioncontrol.RestoreCheckpoint(r.Context(), h.mcDB, taskID, input.CheckpointID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("restore checkpoint: %v", err))
		http.Error(w, "Failed to restore checkpoint", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("checkpoint_restored", "Checkpoint restored", nil, &taskID)
	writeJSON(w, map[string]any{"success": true})
}
