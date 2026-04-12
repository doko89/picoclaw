package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCWorkspaceIsolationRoutes binds workspace isolation endpoints to the ServeMux.
func (h *Handler) registerMCWorkspaceIsolationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/tasks/{id}/workspace", h.handleGetMCGitWorkspace)
	mux.HandleFunc("POST /api/mc/tasks/{id}/workspace", h.handleCreateMCGitWorkspace)
	mux.HandleFunc("POST /api/mc/tasks/{id}/workspace/merge", h.handleMergeMCGitWorkspace)
	mux.HandleFunc("DELETE /api/mc/tasks/{id}/workspace", h.handleAbandonMCGitWorkspace)
	mux.HandleFunc("GET /api/mc/products/{id}/workspaces", h.handleListMCGitWorkspaces)
	mux.HandleFunc("GET /api/mc/workspaces/ports/status", h.handleListMCWorkspacePorts)
}

type mcGitWorkspace struct {
	TaskID       string  `json:"task_id"`
	RepoURL      string  `json:"repo_url"`
	BaseBranch   string  `json:"base_branch"`
	WorktreePath string  `json:"worktree_path"`
	SandboxPath  string  `json:"sandbox_path"`
	Port         *int    `json:"port"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	ExpiresAt    *string `json:"expires_at"`
}

type mcWorkspacePort struct {
	TaskID      string  `json:"task_id"`
	ProductID   string  `json:"product_id"`
	Port        int     `json:"port"`
	AllocatedAt string  `json:"allocated_at"`
	ReleasedAt  *string `json:"released_at"`
}

// handleGetMCGitWorkspace gets workspace info for a task.
//
//	GET /api/mc/tasks/{id}/workspace
func (h *Handler) handleGetMCGitWorkspace(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	info, err := missioncontrol.GetWorkspaceStatus(r.Context(), h.mcDB, taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get workspace: %v", err))
		http.Error(w, "Failed to get workspace", http.StatusInternalServerError)
		return
	}

	if info == nil {
		writeJSON(w, nil)
		return
	}

	var expiresAt *string
	if info.ExpiresAt != nil {
		s := info.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		expiresAt = &s
	}

	writeJSON(w, mcGitWorkspace{
		TaskID:       info.TaskID,
		BaseBranch:   info.BaseBranch,
		WorktreePath: info.WorktreePath,
		SandboxPath:  info.SandboxPath,
		Port:         info.Port,
		Status:       info.Status,
		CreatedAt:    info.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ExpiresAt:    expiresAt,
	})
}

// handleCreateMCGitWorkspace creates a workspace for a task.
//
//	POST /api/mc/tasks/{id}/workspace
func (h *Handler) handleCreateMCGitWorkspace(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		RepoURL    string `json:"repo_url"`
		BaseBranch string `json:"base_branch"`
		ProductID  string `json:"product_id"`
		UseSandbox bool   `json:"use_sandbox"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Setup worktree
	worktreePath, err := missioncontrol.SetupWorktree(r.Context(), h.mcDB, taskID, input.RepoURL, input.BaseBranch)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("setup worktree: %v", err))
		http.Error(w, fmt.Sprintf("Failed to setup worktree: %v", err), http.StatusInternalServerError)
		return
	}

	sandboxPath := ""
	if input.UseSandbox {
		sandboxPath, err = missioncontrol.SetupSandbox(r.Context(), h.mcDB, taskID, worktreePath)
		if err != nil {
			logger.ErrorC("mc", fmt.Sprintf("setup sandbox: %v", err))
			// Continue without sandbox
		}
	}

	// Allocate port if product ID provided
	var port *int
	if input.ProductID != "" {
		allocatedPort, err := missioncontrol.AllocatePort(r.Context(), h.mcDB, taskID, input.ProductID)
		if err != nil {
			logger.ErrorC("mc", fmt.Sprintf("allocate port: %v", err))
		} else {
			port = &allocatedPort
		}
	}

	h.broadcastMCEvent("workspace_created", "Workspace created", nil, &taskID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, mcGitWorkspace{
		TaskID:       taskID,
		RepoURL:      input.RepoURL,
		BaseBranch:   input.BaseBranch,
		WorktreePath: worktreePath,
		SandboxPath:  sandboxPath,
		Port:         port,
		Status:       "active",
	})
}

// handleMergeMCGitWorkspace merges a workspace back to main branch.
//
//	POST /api/mc/tasks/{id}/workspace/merge
func (h *Handler) handleMergeMCGitWorkspace(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	prURL, err := missioncontrol.MergeWorkspace(r.Context(), h.mcDB, taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("merge workspace: %v", err))
		http.Error(w, fmt.Sprintf("Failed to merge workspace: %v", err), http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("workspace_merged", "Workspace merged", nil, &taskID)
	writeJSON(w, map[string]any{
		"success": true,
		"pr_url":  prURL,
	})
}

// handleAbandonMCGitWorkspace abandons and cleans up a workspace.
//
//	DELETE /api/mc/tasks/{id}/workspace
func (h *Handler) handleAbandonMCGitWorkspace(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	err := missioncontrol.AbandonWorkspace(r.Context(), h.mcDB, taskID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("abandon workspace: %v", err))
		http.Error(w, "Failed to abandon workspace", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("workspace_abandoned", "Workspace abandoned", nil, &taskID)
	w.WriteHeader(http.StatusNoContent)
}

// handleListMCGitWorkspaces lists pending workspaces for a product.
//
//	GET /api/mc/products/{id}/workspaces
func (h *Handler) handleListMCGitWorkspaces(w http.ResponseWriter, r *http.Request) {
	productID := r.PathValue("id")

	taskIDs, err := missioncontrol.GetPendingWorkspacesForProduct(r.Context(), h.mcDB, productID)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list product workspaces: %v", err))
		http.Error(w, "Failed to list workspaces", http.StatusInternalServerError)
		return
	}

	var workspaces []mcGitWorkspace
	for _, taskID := range taskIDs {
		info, err := missioncontrol.GetWorkspaceStatus(r.Context(), h.mcDB, taskID)
		if err != nil {
			continue
		}
		if info == nil {
			continue
		}

		var expiresAt *string
		if info.ExpiresAt != nil {
			s := info.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
			expiresAt = &s
		}

		workspaces = append(workspaces, mcGitWorkspace{
			TaskID:       info.TaskID,
			BaseBranch:   info.BaseBranch,
			WorktreePath: info.WorktreePath,
			SandboxPath:  info.SandboxPath,
			Port:         info.Port,
			Status:       info.Status,
			CreatedAt:    info.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt:    expiresAt,
		})
	}

	if workspaces == nil {
		workspaces = []mcGitWorkspace{}
	}
	writeJSON(w, workspaces)
}

// handleListMCWorkspacePorts lists all port allocations.
//
//	GET /api/mc/workspaces/ports/status
func (h *Handler) handleListMCWorkspacePorts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	productID := query.Get("product_id")
	showReleased := query.Get("released") == "true"

	var rows *sql.Rows
	var err error

	if productID != "" {
		if showReleased {
			rows, err = h.mcDB.QueryContext(r.Context(),
				"SELECT task_id, product_id, port, allocated_at, released_at FROM workspace_ports WHERE product_id = ? ORDER BY port ASC",
				productID,
			)
		} else {
			rows, err = h.mcDB.QueryContext(r.Context(),
				"SELECT task_id, product_id, port, allocated_at, released_at FROM workspace_ports WHERE product_id = ? AND released_at IS NULL ORDER BY port ASC",
				productID,
			)
		}
	} else {
		if showReleased {
			rows, err = h.mcDB.QueryContext(r.Context(),
				"SELECT task_id, product_id, port, allocated_at, released_at FROM workspace_ports ORDER BY port ASC",
			)
		} else {
			rows, err = h.mcDB.QueryContext(r.Context(),
				"SELECT task_id, product_id, port, allocated_at, released_at FROM workspace_ports WHERE released_at IS NULL ORDER BY port ASC",
			)
		}
	}

	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list workspace ports: %v", err))
		http.Error(w, "Failed to list ports", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ports []mcWorkspacePort
	for rows.Next() {
		var p mcWorkspacePort
		var releasedAt sql.NullString
		if err := rows.Scan(&p.TaskID, &p.ProductID, &p.Port, &p.AllocatedAt, &releasedAt); err != nil {
			continue
		}
		if releasedAt.Valid {
			p.ReleasedAt = &releasedAt.String
		}
		ports = append(ports, p)
	}

	if ports == nil {
		ports = []mcWorkspacePort{}
	}
	writeJSON(w, ports)
}
