package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCWorkspaceRoutes binds workspace management endpoints to the ServeMux.
func (h *Handler) registerMCWorkspaceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/workspaces", h.handleListMCWorkspaces)
	mux.HandleFunc("POST /api/mc/workspaces", h.handleCreateMCWorkspace)
	mux.HandleFunc("GET /api/mc/workspaces/{id}", h.handleGetMCWorkspace)
	mux.HandleFunc("PUT /api/mc/workspaces/{id}", h.handleUpdateMCWorkspace)
	mux.HandleFunc("DELETE /api/mc/workspaces/{id}", h.handleDeleteMCWorkspace)
}

type mcWorkspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// handleListMCWorkspaces returns all workspaces.
//
//	GET /api/mc/workspaces
func (h *Handler) handleListMCWorkspaces(w http.ResponseWriter, r *http.Request) {
	rows, err := h.mcDB.QueryContext(r.Context(), "SELECT id, name, slug, description, icon, created_at, updated_at FROM workspaces ORDER BY created_at DESC")
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list workspaces: %v", err))
		http.Error(w, "Failed to list workspaces", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var workspaces []mcWorkspace
	for rows.Next() {
		var ws mcWorkspace
		if err = rows.Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.Description, &ws.Icon, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan workspace: %v", err))
			http.Error(w, "Failed to scan workspace", http.StatusInternalServerError)
			return
		}
		workspaces = append(workspaces, ws)
	}
	if workspaces == nil {
		workspaces = []mcWorkspace{}
	}
	writeJSON(w, workspaces)
}

// handleCreateMCWorkspace creates a new workspace.
//
//	POST /api/mc/workspaces
func (h *Handler) handleCreateMCWorkspace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if input.Slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	if input.Icon == "" {
		input.Icon = "📁"
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO workspaces (id, name, slug, description, icon) VALUES (?, ?, ?, ?, ?)",
		id, input.Name, input.Slug, input.Description, input.Icon,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create workspace: %v", err))
		http.Error(w, "Failed to create workspace", http.StatusInternalServerError)
		return
	}

	h.broadcaster.Broadcast(mcSSEEvent("workspace_created", map[string]string{"id": id}))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	var ws mcWorkspace
	_ = h.mcDB.QueryRowContext(r.Context(), "SELECT id, name, slug, description, icon, created_at, updated_at FROM workspaces WHERE id = ?", id).
		Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.Description, &ws.Icon, &ws.CreatedAt, &ws.UpdatedAt)
	writeJSON(w, ws)
}

// handleGetMCWorkspace returns a single workspace.
//
//	GET /api/mc/workspaces/{id}
func (h *Handler) handleGetMCWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var ws mcWorkspace
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, name, slug, description, icon, created_at, updated_at FROM workspaces WHERE id = ?", id,
	).Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.Description, &ws.Icon, &ws.CreatedAt, &ws.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get workspace: %v", err))
		http.Error(w, "Failed to get workspace", http.StatusInternalServerError)
		return
	}
	writeJSON(w, ws)
}

// handleUpdateMCWorkspace updates a workspace.
//
//	PUT /api/mc/workspaces/{id}
func (h *Handler) handleUpdateMCWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE workspaces SET name = ?, slug = ?, description = ?, icon = ?, updated_at = datetime('now') WHERE id = ?",
		input.Name, input.Slug, input.Description, input.Icon, id,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update workspace: %v", err))
		http.Error(w, "Failed to update workspace", http.StatusInternalServerError)
		return
	}

	h.broadcaster.Broadcast(mcSSEEvent("workspace_updated", map[string]string{"id": id}))
	var ws mcWorkspace
	_ = h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, name, slug, description, icon, created_at, updated_at FROM workspaces WHERE id = ?", id,
	).Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.Description, &ws.Icon, &ws.CreatedAt, &ws.UpdatedAt)
	writeJSON(w, ws)
}

// handleDeleteMCWorkspace deletes a workspace.
//
//	DELETE /api/mc/workspaces/{id}
func (h *Handler) handleDeleteMCWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := h.mcDB.ExecContext(r.Context(), "DELETE FROM workspaces WHERE id = ?", id)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete workspace: %v", err))
		http.Error(w, "Failed to delete workspace", http.StatusInternalServerError)
		return
	}
	h.broadcaster.Broadcast(mcSSEEvent("workspace_deleted", map[string]string{"id": id}))
	w.WriteHeader(http.StatusNoContent)
}

// mcSSEEvent is a helper to create an SSEEvent with a JSON payload.
func mcSSEEvent(eventType string, payload any) missioncontrol.SSEEvent {
	data, _ := json.Marshal(payload)
	return missioncontrol.SSEEvent{
		Type:    eventType,
		Payload: data,
	}
}

// writeJSON writes v as JSON to w.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}