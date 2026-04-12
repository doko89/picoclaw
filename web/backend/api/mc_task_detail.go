package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// registerMCTaskDetailRoutes binds task detail endpoints to the ServeMux.
func (h *Handler) registerMCTaskDetailRoutes(mux *http.ServeMux) {
	// Activities
	mux.HandleFunc("GET /api/mc/tasks/{id}/activities", h.handleListMCTaskActivities)
	mux.HandleFunc("POST /api/mc/tasks/{id}/activities", h.handleCreateMCTaskActivity)

	// Deliverables
	mux.HandleFunc("GET /api/mc/tasks/{id}/deliverables", h.handleListMCTaskDeliverables)
	mux.HandleFunc("POST /api/mc/tasks/{id}/deliverables", h.handleCreateMCTaskDeliverable)

	// Roles
	mux.HandleFunc("GET /api/mc/tasks/{id}/roles", h.handleListMCTaskRoles)
	mux.HandleFunc("POST /api/mc/tasks/{id}/roles", h.handleCreateMCTaskRole)
	mux.HandleFunc("DELETE /api/mc/tasks/{id}/roles/{roleId}", h.handleDeleteMCTaskRole)

	// Unread
	mux.HandleFunc("GET /api/mc/tasks/unread", h.handleListMCUnreadTasks)
	mux.HandleFunc("POST /api/mc/tasks/{id}/read", h.handleMarkMCTaskRead)
}

// --- Activities ---

type mcActivity struct {
	ID           string  `json:"id"`
	TaskID       string  `json:"task_id"`
	AgentID      *string `json:"agent_id"`
	ActivityType string  `json:"activity_type"`
	Message      string  `json:"message"`
	Metadata      *string `json:"metadata"`
	CreatedAt    string  `json:"created_at"`
}

// handleListMCTaskActivities returns activities for a task.
//
//	GET /api/mc/tasks/{id}/activities
func (h *Handler) handleListMCTaskActivities(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, task_id, agent_id, activity_type, message, metadata, created_at FROM task_activities WHERE task_id = ? ORDER BY created_at DESC", taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list activities: %v", err))
		http.Error(w, "Failed to list activities", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var activities []mcActivity
	for rows.Next() {
		var a mcActivity
		if err = rows.Scan(&a.ID, &a.TaskID, &a.AgentID, &a.ActivityType, &a.Message, &a.Metadata, &a.CreatedAt); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan activity: %v", err))
			http.Error(w, "Failed to scan activity", http.StatusInternalServerError)
			return
		}
		activities = append(activities, a)
	}
	if activities == nil {
		activities = []mcActivity{}
	}
	writeJSON(w, activities)
}

// handleCreateMCTaskActivity logs a new activity for a task.
//
//	POST /api/mc/tasks/{id}/activities
func (h *Handler) handleCreateMCTaskActivity(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	var input struct {
		AgentID      string `json:"agent_id"`
		ActivityType string `json:"activity_type"`
		Message      string `json:"message"`
		Metadata     string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.ActivityType == "" {
		input.ActivityType = "updated"
	}
	if input.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	var agentID *string
	if input.AgentID != "" {
		agentID = &input.AgentID
	}
	var metadata *string
	if input.Metadata != "" {
		metadata = &input.Metadata
	}

	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO task_activities (id, task_id, agent_id, activity_type, message, metadata) VALUES (?, ?, ?, ?, ?, ?)",
		id, taskID, agentID, input.ActivityType, input.Message, metadata,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create activity: %v", err))
		http.Error(w, "Failed to create activity", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("activity_logged", input.Message, agentID, &taskID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	var a mcActivity
	_ = h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, task_id, agent_id, activity_type, message, metadata, created_at FROM task_activities WHERE id = ?", id,
	).Scan(&a.ID, &a.TaskID, &a.AgentID, &a.ActivityType, &a.Message, &a.Metadata, &a.CreatedAt)
	writeJSON(w, a)
}

// --- Deliverables ---

type mcDeliverable struct {
	ID              string `json:"id"`
	TaskID          string `json:"task_id"`
	DeliverableType string `json:"deliverable_type"`
	Title           string `json:"title"`
	Path            string `json:"path"`
	Description     string `json:"description"`
	CreatedAt       string `json:"created_at"`
}

// handleListMCTaskDeliverables returns deliverables for a task.
//
//	GET /api/mc/tasks/{id}/deliverables
func (h *Handler) handleListMCTaskDeliverables(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, task_id, deliverable_type, title, path, description, created_at FROM task_deliverables WHERE task_id = ? ORDER BY created_at DESC", taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list deliverables: %v", err))
		http.Error(w, "Failed to list deliverables", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var deliverables []mcDeliverable
	for rows.Next() {
		var d mcDeliverable
		if err = rows.Scan(&d.ID, &d.TaskID, &d.DeliverableType, &d.Title, &d.Path, &d.Description, &d.CreatedAt); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan deliverable: %v", err))
			http.Error(w, "Failed to scan deliverable", http.StatusInternalServerError)
			return
		}
		deliverables = append(deliverables, d)
	}
	if deliverables == nil {
		deliverables = []mcDeliverable{}
	}
	writeJSON(w, deliverables)
}

// handleCreateMCTaskDeliverable adds a deliverable to a task.
//
//	POST /api/mc/tasks/{id}/deliverables
func (h *Handler) handleCreateMCTaskDeliverable(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	var input struct {
		DeliverableType string `json:"deliverable_type"`
		Title           string `json:"title"`
		Path            string `json:"path"`
		Description     string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if input.DeliverableType == "" {
		input.DeliverableType = "file"
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO task_deliverables (id, task_id, deliverable_type, title, path, description) VALUES (?, ?, ?, ?, ?, ?)",
		id, taskID, input.DeliverableType, input.Title, input.Path, input.Description,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create deliverable: %v", err))
		http.Error(w, "Failed to create deliverable", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("deliverable_added", input.Title, nil, &taskID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	var d mcDeliverable
	_ = h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, task_id, deliverable_type, title, path, description, created_at FROM task_deliverables WHERE id = ?", id,
	).Scan(&d.ID, &d.TaskID, &d.DeliverableType, &d.Title, &d.Path, &d.Description, &d.CreatedAt)
	writeJSON(w, d)
}

// --- Roles ---

type mcTaskRole struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Role      string `json:"role"`
	AgentID   string `json:"agent_id"`
	CreatedAt string `json:"created_at"`
}

// handleListMCTaskRoles returns role assignments for a task.
//
//	GET /api/mc/tasks/{id}/roles
func (h *Handler) handleListMCTaskRoles(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, task_id, role, agent_id, created_at FROM task_roles WHERE task_id = ?", taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list roles: %v", err))
		http.Error(w, "Failed to list roles", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var roles []mcTaskRole
	for rows.Next() {
		var r mcTaskRole
		if err = rows.Scan(&r.ID, &r.TaskID, &r.Role, &r.AgentID, &r.CreatedAt); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan role: %v", err))
			http.Error(w, "Failed to scan role", http.StatusInternalServerError)
			return
		}
		roles = append(roles, r)
	}
	if roles == nil {
		roles = []mcTaskRole{}
	}
	writeJSON(w, roles)
}

// handleCreateMCTaskRole assigns a role to an agent for a task.
//
//	POST /api/mc/tasks/{id}/roles
func (h *Handler) handleCreateMCTaskRole(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	var input struct {
		Role    string `json:"role"`
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Role == "" || input.AgentID == "" {
		http.Error(w, "role and agent_id are required", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO task_roles (id, task_id, role, agent_id) VALUES (?, ?, ?, ?)",
		id, taskID, input.Role, input.AgentID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create role: %v", err))
		http.Error(w, "Failed to create role", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	var role mcTaskRole
	_ = h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, task_id, role, agent_id, created_at FROM task_roles WHERE id = ?", id,
	).Scan(&role.ID, &role.TaskID, &role.Role, &role.AgentID, &role.CreatedAt)
	writeJSON(w, role)
}

// handleDeleteMCTaskRole removes a role assignment.
//
//	DELETE /api/mc/tasks/{id}/roles/{roleId}
func (h *Handler) handleDeleteMCTaskRole(w http.ResponseWriter, r *http.Request) {
	roleID := r.PathValue("roleId")
	_, err := h.mcDB.ExecContext(r.Context(), "DELETE FROM task_roles WHERE id = ?", roleID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete role: %v", err))
		http.Error(w, "Failed to delete role", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Unread ---

// handleListMCUnreadTasks returns unread task counts per task.
//
//	GET /api/mc/tasks/unread
func (h *Handler) handleListMCUnreadTasks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT task_id, COUNT(*) as unread_count FROM task_notes WHERE status = 'pending' GROUP BY task_id")
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list unread: %v", err))
		http.Error(w, "Failed to list unread", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type unreadEntry struct {
		TaskID      string `json:"task_id"`
		UnreadCount int    `json:"unread_count"`
	}
	var entries []unreadEntry
	for rows.Next() {
		var e unreadEntry
		if err = rows.Scan(&e.TaskID, &e.UnreadCount); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan unread: %v", err))
			http.Error(w, "Failed to scan unread", http.StatusInternalServerError)
			return
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []unreadEntry{}
	}
	writeJSON(w, entries)
}

// handleMarkMCTaskRead marks all pending notes for a task as read.
//
//	POST /api/mc/tasks/{id}/read
func (h *Handler) handleMarkMCTaskRead(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE task_notes SET status = 'read' WHERE task_id = ? AND status = 'pending'", taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("mark read: %v", err))
		http.Error(w, "Failed to mark as read", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}