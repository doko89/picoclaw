package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// registerMCWorkflowRoutes binds workflow endpoints to the ServeMux.
func (h *Handler) registerMCWorkflowRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/workspaces/{id}/workflows", h.handleListMCWorkflows)
	mux.HandleFunc("POST /api/mc/workspaces/{id}/workflows", h.handleCreateMCWorkflow)
	mux.HandleFunc("PUT /api/mc/workflows/{id}", h.handleUpdateMCWorkflow)
	mux.HandleFunc("DELETE /api/mc/workflows/{id}", h.handleDeleteMCWorkflow)
}

type mcWorkflowTemplate struct {
	ID         string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name       string `json:"name"`
	Stages     string `json:"stages"`
	Category   string `json:"category"`
	CreatedAt  string `json:"created_at"`
}

// handleListMCWorkflows returns workflow templates for a workspace.
//
//	GET /api/mc/workspaces/{id}/workflows
func (h *Handler) handleListMCWorkflows(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")

	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, workspace_id, name, stages, category, created_at FROM workflow_templates WHERE workspace_id = ? ORDER BY created_at DESC",
		workspaceID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list workflows: %v", err))
		http.Error(w, "Failed to list workflows", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var workflows []mcWorkflowTemplate
	for rows.Next() {
		var wf mcWorkflowTemplate
		if err = rows.Scan(&wf.ID, &wf.WorkspaceID, &wf.Name, &wf.Stages, &wf.Category, &wf.CreatedAt); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan workflow: %v", err))
			http.Error(w, "Failed to scan workflow", http.StatusInternalServerError)
			return
		}
		workflows = append(workflows, wf)
	}
	if workflows == nil {
		workflows = []mcWorkflowTemplate{}
	}
	writeJSON(w, workflows)
}

// handleCreateMCWorkflow creates a workflow template.
//
//	POST /api/mc/workspaces/{id}/workflows
func (h *Handler) handleCreateMCWorkflow(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")

	var input struct {
		Name     string `json:"name"`
		Stages   string `json:"stages"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if input.Stages == "" {
		input.Stages = `["assigned","in_progress","done"]`
	}
	if input.Category == "" {
		input.Category = "custom"
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO workflow_templates (id, workspace_id, name, stages, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))",
		id, workspaceID, input.Name, input.Stages, input.Category,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create workflow: %v", err))
		http.Error(w, "Failed to create workflow", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	var wf mcWorkflowTemplate
	_ = h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, workspace_id, name, stages, category, created_at FROM workflow_templates WHERE id = ?",
		id,
	).Scan(&wf.ID, &wf.WorkspaceID, &wf.Name, &wf.Stages, &wf.Category, &wf.CreatedAt)
	writeJSON(w, wf)
}

// handleUpdateMCWorkflow updates a workflow template.
//
//	PUT /api/mc/workflows/{id}
func (h *Handler) handleUpdateMCWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var input struct {
		Name     string `json:"name"`
		Stages   string `json:"stages"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE workflow_templates SET name = COALESCE(?, name), stages = COALESCE(?, stages), category = COALESCE(?, category), updated_at = datetime('now') WHERE id = ?",
		input.Name, input.Stages, input.Category, id,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update workflow: %v", err))
		http.Error(w, "Failed to update workflow", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

// handleDeleteMCWorkflow deletes a workflow template.
//
//	DELETE /api/mc/workflows/{id}
func (h *Handler) handleDeleteMCWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.mcDB.ExecContext(r.Context(), "DELETE FROM workflow_templates WHERE id = ?", id)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete workflow: %v", err))
		http.Error(w, "Failed to delete workflow", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
