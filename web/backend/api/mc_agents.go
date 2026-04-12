package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// registerMCAgentRoutes binds agent management endpoints to the ServeMux.
func (h *Handler) registerMCAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/agents", h.handleListMCAgents)
	mux.HandleFunc("POST /api/mc/agents", h.handleCreateMCAgent)
	mux.HandleFunc("GET /api/mc/agents/{id}", h.handleGetMCAgent)
	mux.HandleFunc("PUT /api/mc/agents/{id}", h.handleUpdateMCAgent)
	mux.HandleFunc("DELETE /api/mc/agents/{id}", h.handleDeleteMCAgent)
	mux.HandleFunc("GET /api/mc/agents/discover", h.handleDiscoverMCAgents)
	mux.HandleFunc("POST /api/mc/agents/import", h.handleImportMCAgent)
}

type mcAgent struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Role             string  `json:"role"`
	Description      string  `json:"description"`
	AvatarEmoji      string  `json:"avatar_emoji"`
	Status           string  `json:"status"`
	IsMaster         bool    `json:"is_master"`
	WorkspaceID      string  `json:"workspace_id"`
	Model            string  `json:"model"`
	Source           string  `json:"source"`
	GatewayAgentID   string  `json:"gateway_agent_id"`
	SessionKeyPrefix string  `json:"session_key_prefix"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	TotalTokensUsed   int     `json:"total_tokens_used"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// scanAgent scans a single agent row into an mcAgent struct.
// It handles the INTEGER to bool conversion for is_master
// and nullable columns (model, source, gateway_agent_id, session_key_prefix, description).
func scanAgent(row interface{ Scan(dest ...any) error }) (*mcAgent, error) {
	var a mcAgent
	var isMaster int
	var model, source, gatewayAgentID, sessionKeyPrefix, description sql.NullString
	err := row.Scan(
		&a.ID, &a.Name, &a.Role, &description, &a.AvatarEmoji, &a.Status,
		&isMaster, &a.WorkspaceID, &model, &source, &gatewayAgentID,
		&sessionKeyPrefix, &a.TotalCostUSD, &a.TotalTokensUsed,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.IsMaster = isMaster != 0
	if model.Valid {
		a.Model = model.String
	}
	if source.Valid {
		a.Source = source.String
	}
	if gatewayAgentID.Valid {
		a.GatewayAgentID = gatewayAgentID.String
	}
	if sessionKeyPrefix.Valid {
		a.SessionKeyPrefix = sessionKeyPrefix.String
	}
	if description.Valid {
		a.Description = description.String
	}
	return &a, nil
}

// handleListMCAgents returns agents, optionally filtered by workspace_id.
//
//	GET /api/mc/agents?workspace_id=xxx
func (h *Handler) handleListMCAgents(w http.ResponseWriter, r *http.Request) {
	wsID := r.URL.Query().Get("workspace_id")

	var rows *sql.Rows
	var err error
	if wsID != "" {
		rows, err = h.mcDB.QueryContext(r.Context(),
			"SELECT id, name, role, description, avatar_emoji, status, is_master, workspace_id, model, source, gateway_agent_id, session_key_prefix, total_cost_usd, total_tokens_used, created_at, updated_at FROM agents WHERE workspace_id = ? ORDER BY created_at DESC", wsID)
	} else {
		rows, err = h.mcDB.QueryContext(r.Context(),
			"SELECT id, name, role, description, avatar_emoji, status, is_master, workspace_id, model, source, gateway_agent_id, session_key_prefix, total_cost_usd, total_tokens_used, created_at, updated_at FROM agents ORDER BY created_at DESC")
	}
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list agents: %v", err))
		http.Error(w, "Failed to list agents", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var agents []mcAgent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan agent: %v", err))
			http.Error(w, "Failed to scan agent", http.StatusInternalServerError)
			return
		}
		agents = append(agents, *a)
	}
	if agents == nil {
		agents = []mcAgent{}
	}
	writeJSON(w, agents)
}

// handleCreateMCAgent creates a new agent.
//
//	POST /api/mc/agents
func (h *Handler) handleCreateMCAgent(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name             string `json:"name"`
		Role             string `json:"role"`
		Description      string `json:"description"`
		AvatarEmoji      string `json:"avatar_emoji"`
		WorkspaceID      string `json:"workspace_id"`
		Model            string `json:"model"`
		Source           string `json:"source"`
		GatewayAgentID   string `json:"gateway_agent_id"`
		SessionKeyPrefix string `json:"session_key_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if input.Role == "" {
		input.Role = "builder"
	}
	if input.Source == "" {
		input.Source = "local"
	}
	if input.WorkspaceID == "" {
		input.WorkspaceID = "default"
	}
	if input.AvatarEmoji == "" {
		input.AvatarEmoji = "🤖"
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO agents (id, name, role, description, avatar_emoji, workspace_id, model, source, gateway_agent_id, session_key_prefix) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, input.Name, input.Role, input.Description, input.AvatarEmoji, input.WorkspaceID, input.Model, input.Source, input.GatewayAgentID, input.SessionKeyPrefix,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("create agent: %v", err))
		http.Error(w, "Failed to create agent", http.StatusInternalServerError)
		return
	}

	h.broadcaster.Broadcast(mcSSEEvent("agent_created", map[string]string{"id": id}))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	a, _ := scanAgent(h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, name, role, description, avatar_emoji, status, is_master, workspace_id, model, source, gateway_agent_id, session_key_prefix, total_cost_usd, total_tokens_used, created_at, updated_at FROM agents WHERE id = ?", id,
	))
	writeJSON(w, a)
}

// handleGetMCAgent returns a single agent.
//
//	GET /api/mc/agents/{id}
func (h *Handler) handleGetMCAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := scanAgent(h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, name, role, description, avatar_emoji, status, is_master, workspace_id, model, source, gateway_agent_id, session_key_prefix, total_cost_usd, total_tokens_used, created_at, updated_at FROM agents WHERE id = ?", id,
	))
	if err == sql.ErrNoRows {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("get agent: %v", err))
		http.Error(w, "Failed to get agent", http.StatusInternalServerError)
		return
	}
	writeJSON(w, a)
}

// handleUpdateMCAgent updates an agent.
//
//	PUT /api/mc/agents/{id}
func (h *Handler) handleUpdateMCAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Name        string `json:"name"`
		Role        string `json:"role"`
		Description string `json:"description"`
		Model       string `json:"model"`
		Status      string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE agents SET name = ?, role = ?, description = ?, model = ?, status = ?, updated_at = datetime('now') WHERE id = ?",
		input.Name, input.Role, input.Description, input.Model, input.Status, id,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("update agent: %v", err))
		http.Error(w, "Failed to update agent", http.StatusInternalServerError)
		return
	}
	h.broadcaster.Broadcast(mcSSEEvent("agent_updated", map[string]string{"id": id}))
	a, _ := scanAgent(h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, name, role, description, avatar_emoji, status, is_master, workspace_id, model, source, gateway_agent_id, session_key_prefix, total_cost_usd, total_tokens_used, created_at, updated_at FROM agents WHERE id = ?", id,
	))
	writeJSON(w, a)
}

// handleDeleteMCAgent deletes an agent.
//
//	DELETE /api/mc/agents/{id}
func (h *Handler) handleDeleteMCAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := h.mcDB.ExecContext(r.Context(), "DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("delete agent: %v", err))
		http.Error(w, "Failed to delete agent", http.StatusInternalServerError)
		return
	}
	h.broadcaster.Broadcast(mcSSEEvent("agent_deleted", map[string]string{"id": id}))
	w.WriteHeader(http.StatusNoContent)
}

// handleDiscoverMCAgents discovers agents from the PicoClaw configuration.
// Returns a list of available agent configurations that are not yet imported into MC.
//
//	GET /api/mc/agents/discover
func (h *Handler) handleDiscoverMCAgents(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("discover agents - load config: %v", err))
		http.Error(w, "Failed to load configuration", http.StatusInternalServerError)
		return
	}

	// Get agents already imported into MC
	rows, err := h.mcDB.QueryContext(r.Context(), "SELECT gateway_agent_id FROM agents WHERE gateway_agent_id IS NOT NULL AND gateway_agent_id != ''")
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("discover agents - query existing: %v", err))
		http.Error(w, "Failed to query existing agents", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	importedIDs := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			importedIDs[id] = true
		}
	}

	// Build discovery list from config agents
	type discoveredAgent struct {
		Name            string `json:"name"`
		Role            string `json:"role"`
		GatewayAgentID  string `json:"gateway_agent_id"`
		Model           string `json:"model"`
		AlreadyImported bool   `json:"already_imported"`
	}

	var discovered []discoveredAgent

	// Enumerate agents from config
	for _, agentCfg := range cfg.Agents.List {
		agentID := agentCfg.ID
		if agentID == "" {
			continue
		}
		name := agentCfg.Name
		if name == "" {
			name = agentID
		}
		modelName := ""
		if agentCfg.Model != nil {
			modelName = agentCfg.Model.Primary
		}
		if modelName == "" {
			modelName = cfg.Agents.Defaults.GetModelName()
		}
		discovered = append(discovered, discoveredAgent{
			Name:            name,
			Role:            "agent",
			GatewayAgentID:  agentID,
			Model:           modelName,
			AlreadyImported: importedIDs[agentID],
		})
	}

	// Also add default model as a generic agent if not already listed
	defaultModel := cfg.Agents.Defaults.GetModelName()
	if defaultModel != "" {
		defaultID := "default:" + defaultModel
		if !importedIDs[defaultID] {
			found := false
			for _, d := range discovered {
				if d.GatewayAgentID == defaultID {
					found = true
					break
				}
			}
			if !found {
				discovered = append(discovered, discoveredAgent{
					Name:            "Default Agent",
					Role:            "assistant",
					GatewayAgentID:  defaultID,
					Model:           defaultModel,
					AlreadyImported: importedIDs[defaultID],
				})
			}
		}
	}

	writeJSON(w, discovered)
}

// handleImportMCAgent imports a discovered agent into MC.
//
//	POST /api/mc/agents/import
func (h *Handler) handleImportMCAgent(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name           string `json:"name"`
		Role           string `json:"role"`
		GatewayAgentID string `json:"gateway_agent_id"`
		WorkspaceID    string `json:"workspace_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if input.WorkspaceID == "" {
		input.WorkspaceID = "default"
	}
	if input.Role == "" {
		input.Role = "builder"
	}

	id := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO agents (id, name, role, avatar_emoji, workspace_id, source, gateway_agent_id) VALUES (?, ?, ?, '🤖', ?, 'gateway', ?)",
		id, input.Name, input.Role, input.WorkspaceID, input.GatewayAgentID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("import agent: %v", err))
		http.Error(w, "Failed to import agent", http.StatusInternalServerError)
		return
	}

	h.broadcaster.Broadcast(mcSSEEvent("agent_imported", map[string]string{"id": id}))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	a, _ := scanAgent(h.mcDB.QueryRowContext(r.Context(),
		"SELECT id, name, role, description, avatar_emoji, status, is_master, workspace_id, model, source, gateway_agent_id, session_key_prefix, total_cost_usd, total_tokens_used, created_at, updated_at FROM agents WHERE id = ?", id,
	))
	writeJSON(w, a)
}