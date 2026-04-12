package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// registerMCTaskChatRoutes binds task chat/note endpoints to the ServeMux.
func (h *Handler) registerMCTaskChatRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/tasks/{id}/chat", h.handleListMCTaskNotes)
	mux.HandleFunc("POST /api/mc/tasks/{id}/chat", h.handleCreateMCTaskNote)
	mux.HandleFunc("GET /api/mc/tasks/{id}/chat/agents", h.handleListMCTaskChatAgents)
}

// --- Task Notes (Chat) ---

type mcTaskNote struct {
	ID          string  `json:"id"`
	TaskID      string  `json:"task_id"`
	Content     string  `json:"content"`
	Mode        string  `json:"mode"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
	DeliveredAt *string `json:"delivered_at"`
	CreatedAt   string  `json:"created_at"`
}

// handleListMCTaskNotes returns notes for a task.
//
//	GET /api/mc/tasks/{id}/chat
func (h *Handler) handleListMCTaskNotes(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, task_id, content, mode, role, status, delivered_at, created_at FROM task_notes WHERE task_id = ? ORDER BY created_at ASC",
		taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list notes: %v", err))
		http.Error(w, "Failed to list notes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var notes []mcTaskNote
	for rows.Next() {
		var n mcTaskNote
		if err = rows.Scan(&n.ID, &n.TaskID, &n.Content, &n.Mode, &n.Role, &n.Status, &n.DeliveredAt, &n.CreatedAt); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan note: %v", err))
			http.Error(w, "Failed to scan note", http.StatusInternalServerError)
			return
		}
		notes = append(notes, n)
	}
	if notes == nil {
		notes = []mcTaskNote{}
	}
	writeJSON(w, notes)
}

// handleCreateMCTaskNote creates a note (chat message) for a task.
//
//	POST /api/mc/tasks/{id}/chat
func (h *Handler) handleCreateMCTaskNote(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		Content string `json:"content"`
		Mode    string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}
	if input.Mode == "" {
		input.Mode = "note"
	}
	if input.Mode != "note" && input.Mode != "direct" {
		http.Error(w, "mode must be 'note' or 'direct'", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO task_notes (id, task_id, content, mode, role, status) VALUES (?, ?, ?, ?, 'user', 'pending')",
		id, taskID, input.Content, input.Mode,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create note: %v", err))
		http.Error(w, "Failed to create note", http.StatusInternalServerError)
		return
	}

	var n mcTaskNote
	err = h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, task_id, content, mode, role, status, delivered_at, created_at FROM task_notes WHERE id = ?", id,
	).Scan(&n.ID, &n.TaskID, &n.Content, &n.Mode, &n.Role, &n.Status, &n.DeliveredAt, &n.CreatedAt)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("read back note: %v", err))
		n = mcTaskNote{ID: id, TaskID: taskID, Content: input.Content, Mode: input.Mode, Role: "user", Status: "pending"}
	}

	h.broadcastMCEvent("note_queued", "New note added", nil, &taskID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, n)
}

// --- Chat Agents ---

type mcChatAgent struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	AvatarEmoji     string `json:"avatar_emoji"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	IsAssigned      bool   `json:"is_assigned"`
	IsConvoyMember  bool   `json:"is_convoy_member"`
}

// handleListMCTaskChatAgents returns agents available for chat with a task.
//
//	GET /api/mc/tasks/{id}/chat/agents
func (h *Handler) handleListMCTaskChatAgents(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var agents []mcChatAgent
	seen := make(map[string]bool)

	// 1. The assigned agent
	var assignedAgentID sql.NullString
	h.mcDB.QueryRowContext(r.Context(),
		"SELECT assigned_agent_id FROM tasks WHERE id = ?", taskID,
	).Scan(&assignedAgentID)

	if assignedAgentID.Valid {
		var a mcChatAgent
		err := h.mcDB.QueryRowContext(r.Context(),
			"SELECT id, name, COALESCE(avatar_emoji, '🤖'), role, COALESCE(status, 'standby') FROM agents WHERE id = ?",
			assignedAgentID.String,
		).Scan(&a.ID, &a.Name, &a.AvatarEmoji, &a.Role, &a.Status)
		if err == nil {
			a.IsAssigned = true
			agents = append(agents, a)
			seen[a.ID] = true
		}
	}

	// 2. Task role agents
	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT DISTINCT a.id, a.name, COALESCE(a.avatar_emoji, '🤖'), a.role, COALESCE(a.status, 'standby') FROM agents a JOIN task_roles tr ON tr.agent_id = a.id WHERE tr.task_id = ?",
		taskID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a mcChatAgent
			if rows.Scan(&a.ID, &a.Name, &a.AvatarEmoji, &a.Role, &a.Status) == nil && !seen[a.ID] {
				agents = append(agents, a)
				seen[a.ID] = true
			}
		}
	}

	// 3. All workspace agents (from task's workspace)
	var workspaceID sql.NullString
	h.mcDB.QueryRowContext(r.Context(),
		"SELECT workspace_id FROM tasks WHERE id = ?", taskID,
	).Scan(&workspaceID)

	wsID := "default"
	if workspaceID.Valid {
		wsID = workspaceID.String
	}

	rows, err = h.mcDB.QueryContext(r.Context(),
		"SELECT id, name, COALESCE(avatar_emoji, '🤖'), role, COALESCE(status, 'standby') FROM agents WHERE workspace_id = ? ORDER BY name",
		wsID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a mcChatAgent
			if rows.Scan(&a.ID, &a.Name, &a.AvatarEmoji, &a.Role, &a.Status) == nil && !seen[a.ID] {
				agents = append(agents, a)
				seen[a.ID] = true
			}
		}
	}

	if agents == nil {
		agents = []mcChatAgent{}
	}
	writeJSON(w, agents)
}