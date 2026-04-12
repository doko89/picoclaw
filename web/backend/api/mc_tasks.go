package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// registerMCTaskRoutes binds task management endpoints to the ServeMux.
func (h *Handler) registerMCTaskRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/tasks", h.handleListMCTasks)
	mux.HandleFunc("POST /api/mc/tasks", h.handleCreateMCTask)
	mux.HandleFunc("GET /api/mc/tasks/{id}", h.handleGetMCTask)
	mux.HandleFunc("PUT /api/mc/tasks/{id}", h.handleUpdateMCTask)
	mux.HandleFunc("DELETE /api/mc/tasks/{id}", h.handleDeleteMCTask)
	mux.HandleFunc("PATCH /api/mc/tasks/{id}/status", h.handleUpdateMCTaskStatus)
}

type mcTask struct {
	ID                    string   `json:"id"`
	Title                 string   `json:"title"`
	Description           string   `json:"description"`
	Status                string   `json:"status"`
	StatusReason          string   `json:"status_reason,omitempty"`
	Priority              string   `json:"priority"`
	AssignedAgentID       *string  `json:"assigned_agent_id"`
	WorkspaceID           string   `json:"workspace_id"`
	DueDate               *string  `json:"due_date"`
	PlanningComplete      bool     `json:"planning_complete"`
	PlanningSessionKey    string   `json:"planning_session_key,omitempty"`
	PlanningSpec          string   `json:"planning_spec,omitempty"`
	PlanningDispatchError string   `json:"planning_dispatch_error,omitempty"`
	ProductID             *string  `json:"product_id"`
	IdeaID                *string  `json:"idea_id"`
	EstimatedCostUSD      *float64 `json:"estimated_cost_usd"`
	ActualCostUSD         float64  `json:"actual_cost_usd"`
	IsSubtask             bool     `json:"is_subtask"`
	ConvoyID              string   `json:"convoy_id,omitempty"`
	Images                string   `json:"images,omitempty"`
	RepoURL               string   `json:"repo_url,omitempty"`
	RepoBranch            string   `json:"repo_branch,omitempty"`
	PRURL                 string   `json:"pr_url,omitempty"`
	PRStatus              string   `json:"pr_status,omitempty"`
	CreatedAt             string   `json:"created_at"`
	UpdatedAt             string   `json:"updated_at"`
}

// taskColumns is the common column list for task queries.
const taskColumns = `id, title, description, status, status_reason, priority, assigned_agent_id, workspace_id, due_date, planning_complete, planning_session_key, planning_spec, planning_dispatch_error, product_id, idea_id, estimated_cost_usd, actual_cost_usd, is_subtask, convoy_id, images, repo_url, repo_branch, pr_url, pr_status, created_at, updated_at`

// scanTask scans a single task row from the given query row.
func scanTask(row interface{ Scan(dest ...any) error }) (*mcTask, error) {
	var t mcTask
	var planningComplete, isSubtask int
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.StatusReason, &t.Priority, &t.AssignedAgentID, &t.WorkspaceID, &t.DueDate, &planningComplete, &t.PlanningSessionKey, &t.PlanningSpec, &t.PlanningDispatchError, &t.ProductID, &t.IdeaID, &t.EstimatedCostUSD, &t.ActualCostUSD, &isSubtask, &t.ConvoyID, &t.Images, &t.RepoURL, &t.RepoBranch, &t.PRURL, &t.PRStatus, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.PlanningComplete = planningComplete != 0
	t.IsSubtask = isSubtask != 0
	return &t, nil
}

// handleListMCTasks returns tasks, optionally filtered.
//
//	GET /api/mc/tasks?workspace_id=xxx&status=xxx&assigned_agent_id=xxx
func (h *Handler) handleListMCTasks(w http.ResponseWriter, r *http.Request) {
	query := "SELECT " + taskColumns + " FROM tasks WHERE 1=1"
	var args []any

	if wsID := r.URL.Query().Get("workspace_id"); wsID != "" {
		query += " AND workspace_id = ?"
		args = append(args, wsID)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if agentID := r.URL.Query().Get("assigned_agent_id"); agentID != "" {
		query += " AND assigned_agent_id = ?"
		args = append(args, agentID)
	}
	if productID := r.URL.Query().Get("product_id"); productID != "" {
		query += " AND product_id = ?"
		args = append(args, productID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.mcDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list tasks: %v", err))
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []mcTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan task: %v", err))
			http.Error(w, "Failed to scan task", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, *t)
	}
	if tasks == nil {
		tasks = []mcTask{}
	}
	writeJSON(w, tasks)
}

// handleCreateMCTask creates a new task.
//
//	POST /api/mc/tasks
func (h *Handler) handleCreateMCTask(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Priority        string  `json:"priority"`
		WorkspaceID     string  `json:"workspace_id"`
		AssignedAgentID *string `json:"assigned_agent_id"`
		ProductID       *string `json:"product_id"`
		DueDate         *string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	if input.WorkspaceID == "" {
		input.WorkspaceID = "default"
	}

	id := uuid.New().String()
	status := "inbox"
	if input.AssignedAgentID != nil && *input.AssignedAgentID != "" {
		status = "assigned"
	}

	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO tasks (id, title, description, status, priority, assigned_agent_id, workspace_id, due_date, product_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, input.Title, input.Description, status, input.Priority, input.AssignedAgentID, input.WorkspaceID, input.DueDate, input.ProductID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create task: %v", err))
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	h.broadcaster.Broadcast(mcSSEEvent("task_created", map[string]string{"id": id}))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	t, _ := scanTask(h.mcDB.QueryRowContext(r.Context(),
		"SELECT "+taskColumns+" FROM tasks WHERE id = ?", id,
	))
	writeJSON(w, t)
}

// handleGetMCTask returns a single task.
//
//	GET /api/mc/tasks/{id}
func (h *Handler) handleGetMCTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := scanTask(h.mcDB.QueryRowContext(r.Context(),
		"SELECT "+taskColumns+" FROM tasks WHERE id = ?", id,
	))
	if err == sql.ErrNoRows {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get task: %v", err))
		http.Error(w, "Failed to get task", http.StatusInternalServerError)
		return
	}
	writeJSON(w, t)
}

// handleUpdateMCTask updates a task.
//
//	PUT /api/mc/tasks/{id}
func (h *Handler) handleUpdateMCTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Priority        string  `json:"priority"`
		AssignedAgentID *string `json:"assigned_agent_id"`
		DueDate         *string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET title = ?, description = ?, priority = ?, assigned_agent_id = ?, due_date = ?, updated_at = datetime('now') WHERE id = ?",
		input.Title, input.Description, input.Priority, input.AssignedAgentID, input.DueDate, id,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update task: %v", err))
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}
	h.broadcaster.Broadcast(mcSSEEvent("task_updated", map[string]string{"id": id}))
	t, _ := scanTask(h.mcDB.QueryRowContext(r.Context(),
		"SELECT "+taskColumns+" FROM tasks WHERE id = ?", id,
	))
	writeJSON(w, t)
}

// handleDeleteMCTask deletes a task.
//
//	DELETE /api/mc/tasks/{id}
func (h *Handler) handleDeleteMCTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := h.mcDB.ExecContext(r.Context(), "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete task: %v", err))
		http.Error(w, "Failed to delete task", http.StatusInternalServerError)
		return
	}
	h.broadcaster.Broadcast(mcSSEEvent("task_deleted", map[string]string{"id": id}))
	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateMCTaskStatus updates a task's status and broadcasts the change.
//
//	PATCH /api/mc/tasks/{id}/status
func (h *Handler) handleUpdateMCTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Status       string `json:"status"`
		StatusReason string `json:"status_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Status == "" {
		http.Error(w, "status is required", http.StatusBadRequest)
		return
	}

	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET status = ?, status_reason = ?, updated_at = datetime('now') WHERE id = ?",
		input.Status, input.StatusReason, id,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update task status: %v", err))
		http.Error(w, "Failed to update task status", http.StatusInternalServerError)
		return
	}
	h.broadcaster.Broadcast(mcSSEEvent("task_status_changed", map[string]string{"id": id, "status": input.Status}))
	t, _ := scanTask(h.mcDB.QueryRowContext(r.Context(),
		"SELECT "+taskColumns+" FROM tasks WHERE id = ?", id,
	))
	writeJSON(w, t)
}