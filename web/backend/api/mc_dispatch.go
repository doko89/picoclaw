package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCConvoyRoutes binds convoy endpoints to the ServeMux.
func (h *Handler) registerMCConvoyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/tasks/{id}/convoy", h.handleGetMCConvoy)
	mux.HandleFunc("POST /api/mc/tasks/{id}/convoy", h.handleCreateMCConvoy)
	mux.HandleFunc("POST /api/mc/tasks/{id}/convoy/dispatch", h.handleDispatchMCConvoy)
	mux.HandleFunc("GET /api/mc/tasks/{id}/convoy/progress", h.handleGetMCConvoyProgress)
	mux.HandleFunc("GET /api/mc/tasks/{id}/convoy/subtasks", h.handleListMCConvoySubtasks)
	mux.HandleFunc("GET /api/mc/convoy/{convoyId}/mail", h.handleListMCConvoyMail)
}

type mcConvoy struct {
	ID           string `json:"id"`
	ParentTaskID string `json:"parent_task_id"`
	Name         string `json:"name"`
	Strategy     string `json:"strategy"`
	Spec         string `json:"spec"`
	Status       string `json:"status"`
}

type mcConvoySubtask struct {
	ID        string   `json:"id"`
	ConvoyID  string   `json:"convoy_id"`
	TaskID    string   `json:"task_id"`
	DependsOn []string `json:"depends_on"`
	Status    string   `json:"status"`
}

// handleGetMCConvoy gets convoy info for a task.
//
//	GET /api/mc/tasks/{id}/convoy
func (h *Handler) handleGetMCConvoy(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var convoyID sql.NullString
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT convoy_id FROM tasks WHERE id = ?",
		taskID,
	).Scan(&convoyID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get convoy: %v", err))
		http.Error(w, "Failed to get convoy", http.StatusInternalServerError)
		return
	}

	if !convoyID.Valid {
		writeJSON(w, nil)
		return
	}

	// Get full convoy details
	var convoy mcConvoy
	err = h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, parent_task_id, name, strategy, spec, status FROM convoys WHERE id = ?",
		convoyID.String,
	).Scan(&convoy.ID, &convoy.ParentTaskID, &convoy.Name, &convoy.Strategy, &convoy.Spec, &convoy.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, nil)
			return
		}
		logger.ErrorC("mc", fmt.Sprintf("get convoy details: %v", err))
		http.Error(w, "Failed to get convoy", http.StatusInternalServerError)
		return
	}

	writeJSON(w, convoy)
}

// handleCreateMCConvoy creates a new convoy for a task.
//
//	POST /api/mc/tasks/{id}/convoy
func (h *Handler) handleCreateMCConvoy(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		Name     string                           `json:"name"`
		Strategy string                           `json:"strategy"`
		Spec     string                           `json:"spec"`
		Subtasks []missioncontrol.ConvoySubtaskInput `json:"subtasks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	convoy, err := missioncontrol.CreateConvoy(r.Context(), h.mcDB, taskID, input.Name, input.Strategy, input.Spec, input.Subtasks)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create convoy: %v", err))
		http.Error(w, "Failed to create convoy", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("convoy_created", "Convoy created", nil, &taskID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, convoy)
}

// handleDispatchMCConvoy dispatches all subtasks in a convoy.
//
//	POST /api/mc/tasks/{id}/convoy/dispatch
func (h *Handler) handleDispatchMCConvoy(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var convoyID sql.NullString
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT convoy_id FROM tasks WHERE id = ?",
		taskID,
	).Scan(&convoyID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get convoy for dispatch: %v", err))
		http.Error(w, "Failed to get convoy", http.StatusInternalServerError)
		return
	}

	if !convoyID.Valid {
		http.Error(w, "No convoy found for this task", http.StatusNotFound)
		return
	}

	err = missioncontrol.DispatchConvoy(r.Context(), h.mcDB, convoyID.String)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("dispatch convoy: %v", err))
		http.Error(w, "Failed to dispatch convoy", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("convoy_dispatched", "Convoy dispatched", nil, &taskID)
	writeJSON(w, map[string]any{"success": true})
}

// handleGetMCConvoyProgress gets convoy progress.
//
//	GET /api/mc/tasks/{id}/convoy/progress
func (h *Handler) handleGetMCConvoyProgress(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var convoyID sql.NullString
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT convoy_id FROM tasks WHERE id = ?",
		taskID,
	).Scan(&convoyID)
	if err != nil || !convoyID.Valid {
		logger.ErrorC("mc", fmt.Sprintf("get convoy progress: %v", err))
		http.Error(w, "Failed to get convoy progress", http.StatusInternalServerError)
		return
	}

	progress, err := missioncontrol.GetConvoyProgress(r.Context(), h.mcDB, convoyID.String)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get convoy progress: %v", err))
		http.Error(w, "Failed to get convoy progress", http.StatusInternalServerError)
		return
	}

	writeJSON(w, progress)
}

// handleListMCConvoySubtasks lists subtasks in a convoy.
//
//	GET /api/mc/tasks/{id}/convoy/subtasks
func (h *Handler) handleListMCConvoySubtasks(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var convoyID sql.NullString
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT convoy_id FROM tasks WHERE id = ?",
		taskID,
	).Scan(&convoyID)
	if err != nil || !convoyID.Valid {
		writeJSON(w, []mcConvoySubtask{})
		return
	}

	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, convoy_id, task_id, depends_on, status FROM convoy_subtasks WHERE convoy_id = ?",
		convoyID.String,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list convoy subtasks: %v", err))
		http.Error(w, "Failed to list convoy subtasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var subtasks []mcConvoySubtask
	for rows.Next() {
		var st mcConvoySubtask
		var dependsOnJSON sql.NullString
		if err = rows.Scan(&st.ID, &st.ConvoyID, &st.TaskID, &dependsOnJSON, &st.Status); err != nil {
			continue
		}
		if dependsOnJSON.Valid && dependsOnJSON.String != "" {
			json.Unmarshal([]byte(dependsOnJSON.String), &st.DependsOn)
		}
		subtasks = append(subtasks, st)
	}
	if subtasks == nil {
		subtasks = []mcConvoySubtask{}
	}
	writeJSON(w, subtasks)
}

// handleListMCConvoyMail lists mail for convoy agents.
//
//	GET /api/mc/convoy/{convoyId}/mail
func (h *Handler) handleListMCConvoyMail(w http.ResponseWriter, r *http.Request) {
	convoyID := r.PathValue("convoyId")

	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, agent_id, message, created_at FROM agent_mailbox WHERE convoy_id = ? ORDER BY created_at DESC",
		convoyID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list convoy mail: %v", err))
		http.Error(w, "Failed to list convoy mail", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type mailItem struct {
		ID        string `json:"id"`
		AgentID  string `json:"agent_id"`
		Message   string `json:"message"`
		CreatedAt string `json:"created_at"`
	}

	var mail []mailItem
	for rows.Next() {
		var m mailItem
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Message, &m.CreatedAt); err != nil {
			continue
		}
		mail = append(mail, m)
	}
	if mail == nil {
		mail = []mailItem{}
	}
	writeJSON(w, mail)
}
