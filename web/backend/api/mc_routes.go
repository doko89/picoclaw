package api

import (
	"context"
	"net/http"
)

// registerMCRoutes registers all Mission Control API routes.
func (h *Handler) registerMCRoutes(mux *http.ServeMux) {
	h.registerMCWorkspaceRoutes(mux)
	h.registerMCAgentRoutes(mux)
	h.registerMCTaskRoutes(mux)
	h.registerMCEventRoutes(mux)
	h.registerMCTaskDetailRoutes(mux)
	h.registerMCTaskPlanningRoutes(mux)
	h.registerMCTaskImageRoutes(mux)
	h.registerMCTaskChatRoutes(mux)
	// Phase 3: Agent Orchestration
	h.registerMCWorkflowRoutes(mux)
	h.registerMCAgentHealthRoutes(mux)
	h.registerMCCheckpointRoutes(mux)
	h.registerMCConvoyRoutes(mux)
	// Phase 4: Workspace Isolation
	h.registerMCWorkspaceIsolationRoutes(mux)
	// Phase 5: Autopilot Pipeline
	h.registerMCProductRoutes(mux)
	h.registerMCResearchRoutes(mux)
	h.registerMCIdeationRoutes(mux)
	// Phase 6: Automation (Simplified)
	h.registerMCAutomationRoutes(mux)
}

// nilCtx returns a background context for fire-and-forget DB inserts.
func nilCtx() context.Context { return context.Background() }