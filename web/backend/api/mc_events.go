package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/missioncontrol"
)

// registerMCEventRoutes binds SSE event endpoints to the ServeMux.
func (h *Handler) registerMCEventRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/events/stream", h.handleMCSSEStream)
	mux.HandleFunc("GET /api/mc/events", h.handleListMCEvents)
}

// handleMCSSEStream streams real-time Mission Control events via Server-Sent Events.
//
//	GET /api/mc/events/stream
func (h *Handler) handleMCSSEStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial connected event
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Subscribe to broadcaster
	ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(ch)

	// Keep-alive ticker
	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				logger.ErrorC("mc", fmt.Sprintf("marshal SSE event: %v", err))
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-keepAlive.C:
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

// handleListMCEvents returns recent events from the database.
//
//	GET /api/mc/events?limit=50
func (h *Handler) handleListMCEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT id, type, agent_id, task_id, message, metadata, created_at FROM events ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("list events: %v", err))
		http.Error(w, "Failed to list events", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type mcEvent struct {
		ID        string  `json:"id"`
		Type      string  `json:"type"`
		AgentID   *string `json:"agent_id"`
		TaskID    *string `json:"task_id"`
		Message   string  `json:"message"`
		Metadata  *string `json:"metadata"`
		CreatedAt string  `json:"created_at"`
	}

	var events []mcEvent
	for rows.Next() {
		var e mcEvent
		if err = rows.Scan(&e.ID, &e.Type, &e.AgentID, &e.TaskID, &e.Message, &e.Metadata, &e.CreatedAt); err != nil {
			logger.ErrorC("mc", fmt.Sprintf("scan event: %v", err))
			http.Error(w, "Failed to scan event", http.StatusInternalServerError)
			return
		}
		events = append(events, e)
	}
	if events == nil {
		events = []mcEvent{}
	}
	writeJSON(w, events)
}

// broadcastMCEvent inserts an event into the DB and broadcasts it via SSE.
func (h *Handler) broadcastMCEvent(eventType, message string, agentID, taskID *string) {
	// Insert into events table (best-effort)
	_, _ = h.mcDB.ExecContext(nilCtx(),
		"INSERT INTO events (id, type, agent_id, task_id, message) VALUES (?, ?, ?, ?, ?)",
		uuid.New().String(), eventType, agentID, taskID, message,
	)

	// Broadcast via SSE
	payload, _ := json.Marshal(map[string]any{
		"type":     eventType,
		"message":  message,
		"agent_id": agentID,
		"task_id":  taskID,
	})
	h.broadcaster.Broadcast(missioncontrol.SSEEvent{
		Type:    eventType,
		Payload: payload,
	})
}