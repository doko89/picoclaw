package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCAgentHealthRoutes binds agent health monitoring endpoints to the ServeMux.
func (h *Handler) registerMCAgentHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/agents/health", h.handleListMCAgentHealth)
	mux.HandleFunc("GET /api/mc/agents/{id}/health", h.handleGetMCAgentHealth)
	mux.HandleFunc("POST /api/mc/agents/{id}/health/nudge", h.handleNudgeMCAgent)
}

type mcAgentHealth struct {
	AgentID      string                       `json:"agent_id"`
	AgentName    string                       `json:"agent_name"`
	Status       string                       `json:"status"`
	HealthState  missioncontrol.AgentHealthState `json:"health_state"`
	LastActivity *string                      `json:"last_activity"`
	SessionKey   *string                      `json:"session_key"`
	CurrentTaskID *string                      `json:"current_task_id"`
	StallCount   int                          `json:"stall_count"`
}

// handleListMCAgentHealth returns health status for all agents.
//
//	GET /api/mc/agents/health
func (h *Handler) handleListMCAgentHealth(w http.ResponseWriter, r *http.Request) {
	results, err := missioncontrol.RunHealthCheckCycle(r.Context(), h.mcDB)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list agent health: %v", err))
		http.Error(w, "Failed to get agent health", http.StatusInternalServerError)
		return
	}

	var health []mcAgentHealth
	for _, info := range results {
		var lastActivity *string
		if info.LastActivity != nil {
			s := info.LastActivity.Format("2006-01-02T15:04:05Z07:00")
			lastActivity = &s
		}
		health = append(health, mcAgentHealth{
			AgentID:      info.AgentID,
			AgentName:    info.AgentName,
			Status:       info.Status,
			HealthState:  info.HealthState,
			LastActivity: lastActivity,
			SessionKey:   info.SessionKey,
			CurrentTaskID: info.CurrentTaskID,
			StallCount:   info.StallCount,
		})
	}
	if health == nil {
		health = []mcAgentHealth{}
	}
	writeJSON(w, health)
}

// handleGetMCAgentHealth returns detailed health info for a single agent.
//
//	GET /api/mc/agents/{id}/health
func (h *Handler) handleGetMCAgentHealth(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")

	info, err := missioncontrol.CheckAgentHealth(r.Context(), h.mcDB, agentID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get agent health: %v", err))
		http.Error(w, "Failed to get agent health", http.StatusInternalServerError)
		return
	}

	var lastActivity *string
	if info.LastActivity != nil {
		s := info.LastActivity.Format("2006-01-02T15:04:05Z07:00")
		lastActivity = &s
	}

	writeJSON(w, mcAgentHealth{
		AgentID:      info.AgentID,
		AgentName:    info.AgentName,
		Status:       info.Status,
		HealthState:  info.HealthState,
		LastActivity: lastActivity,
		SessionKey:   info.SessionKey,
		CurrentTaskID: info.CurrentTaskID,
		StallCount:   info.StallCount,
	})
}

// handleNudgeMCAgent nudges an agent (kills session and re-dispatches task).
//
//	POST /api/mc/agents/{id}/health/nudge
func (h *Handler) handleNudgeMCAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")

	var input struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	err := missioncontrol.NudgeAgent(r.Context(), h.mcDB, agentID, input.TaskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("nudge agent: %v", err))
		http.Error(w, "Failed to nudge agent", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}
