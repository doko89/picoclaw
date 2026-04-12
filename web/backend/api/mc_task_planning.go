package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// registerMCTaskPlanningRoutes binds planning endpoints to the ServeMux.
func (h *Handler) registerMCTaskPlanningRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mc/tasks/{id}/planning", h.handleGetMCTaskPlanning)
	mux.HandleFunc("POST /api/mc/tasks/{id}/planning", h.handleStartMCTaskPlanning)
	mux.HandleFunc("POST /api/mc/tasks/{id}/planning/answer", h.handleAnswerMCPlanningQuestion)
	mux.HandleFunc("POST /api/mc/tasks/{id}/planning/approve", h.handleApproveMCPlanning)
	mux.HandleFunc("POST /api/mc/tasks/{id}/planning/force-complete", h.handleForceCompleteMCPlanning)
	mux.HandleFunc("POST /api/mc/tasks/{id}/planning/retry-dispatch", h.handleRetryMCPlanningDispatch)
	mux.HandleFunc("GET /api/mc/tasks/{id}/planning/poll", h.handlePollMCPlanning)
	mux.HandleFunc("DELETE /api/mc/tasks/{id}/planning", h.handleCancelMCTaskPlanning)
}

// mcPlanningState represents the planning state for a task.
type mcPlanningState struct {
	TaskID      string                `json:"task_id"`
	SessionKey  *string               `json:"session_key"`
	Messages    []mcPlanningMessage   `json:"messages"`
	IsStarted  bool                  `json:"is_started"`
	IsComplete  bool                  `json:"is_complete"`
	CurrentQuestion *mcPlanningQuestion `json:"current_question"`
	Spec        *mcPlanningSpec       `json:"spec"`
	Agents      json.RawMessage       `json:"agents"`
}

type mcPlanningMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

type mcPlanningQuestion struct {
	Question     string              `json:"question"`
	Options      []mcQuestionOption  `json:"options"`
	QuestionType string              `json:"question_type"`
	Category     string              `json:"category"`
}

type mcQuestionOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type planningQuestionRow struct {
	Category string
	Question string
	Answer   string
}

type mcPlanningSpec struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	SpecMarkdown string `json:"spec_markdown"`
	LockedAt     string `json:"locked_at"`
	LockedBy     *string `json:"locked_by"`
	CreatedAt    string `json:"created_at"`
}

// handleGetMCTaskPlanning returns the current planning state for a task.
//
//	GET /api/mc/tasks/{id}/planning
func (h *Handler) handleGetMCTaskPlanning(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var sessionKey, planningMessages, planningSpec, planningAgents sql.NullString
	var planningComplete sql.NullBool
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT planning_session_key, planning_messages, planning_complete, planning_spec, planning_agents FROM tasks WHERE id = ?",
		taskID,
	).Scan(&sessionKey, &planningMessages, &planningComplete, &planningSpec, &planningAgents)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		logger.ErrorC("mc", fmt.Sprintf("get planning: %v", err))
		http.Error(w, "Failed to get planning state", http.StatusInternalServerError)
		return
	}

	state := mcPlanningState{
		TaskID:     taskID,
		IsStarted:  planningMessages.Valid && planningMessages.String != "",
		IsComplete: planningComplete.Valid && planningComplete.Bool,
	}

	if sessionKey.Valid {
		state.SessionKey = &sessionKey.String
	}

	if planningMessages.Valid && planningMessages.String != "" {
		_ = json.Unmarshal([]byte(planningMessages.String), &state.Messages)
	}

	// Find the latest assistant message with a question
	for i := len(state.Messages) - 1; i >= 0; i-- {
		if state.Messages[i].Role == "assistant" {
			var q mcPlanningQuestion
			if err := json.Unmarshal([]byte(state.Messages[i].Content), &q); err == nil && q.Question != "" {
				state.CurrentQuestion = &q
			}
			break
		}
	}

	if planningSpec.Valid && planningSpec.String != "" {
		var spec mcPlanningSpec
		if err := json.Unmarshal([]byte(planningSpec.String), &spec); err == nil {
			state.Spec = &spec
		}
	}

	if planningAgents.Valid && planningAgents.String != "" {
		state.Agents = json.RawMessage(planningAgents.String)
	}

	writeJSON(w, state)
}

// handleStartMCTaskPlanning starts a planning session for a task.
//
//	POST /api/mc/tasks/{id}/planning
func (h *Handler) handleStartMCTaskPlanning(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Check if planning already started
	var existingKey sql.NullString
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT planning_session_key FROM tasks WHERE id = ?", taskID,
	).Scan(&existingKey)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		logger.ErrorC("mc", fmt.Sprintf("start planning: %v", err))
		http.Error(w, "Failed to start planning", http.StatusInternalServerError)
		return
	}
	if existingKey.Valid && existingKey.String != "" {
		http.Error(w, "Planning already started", http.StatusBadRequest)
		return
	}

	var input struct {
		SessionKeyPrefix string `json:"session_key_prefix"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)

	// Build session key
	prefix := input.SessionKeyPrefix
	if prefix == "" {
		prefix = "agent:main:"
	}
	sessionKey := prefix + "planning:" + taskID

	// Get task info for the planning prompt
	var title, description string
	err = h.mcDB.QueryRowContext(r.Context(),
		"SELECT title, COALESCE(description, '') FROM tasks WHERE id = ?", taskID,
	).Scan(&title, &description)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("start planning - get task: %v", err))
		http.Error(w, "Failed to start planning", http.StatusInternalServerError)
		return
	}

	// Build initial planning messages
	planningPrompt := fmt.Sprintf(`PLANNING REQUEST

Task Title: %s
Task Description: %s

You are starting a planning session for this task. Generate your FIRST question to understand what the user needs. Remember:
- Questions must be multiple choice
- Include an "Other" option
- Be specific to THIS task, not generic

Respond with ONLY valid JSON in this format:
{
  "question": "Your question here?",
  "options": [
    {"id": "A", "label": "First option"},
    {"id": "B", "label": "Second option"},
    {"id": "C", "label": "Third option"},
    {"id": "other", "label": "Other"}
  ]}`, title, description)

	messages := []mcPlanningMessage{
		{Role: "user", Content: planningPrompt, Timestamp: 0},
	}
	messagesJSON, _ := json.Marshal(messages)

	// Update task with session key, messages, and status
	_, err = h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET planning_session_key = ?, planning_messages = ?, status = 'planning', updated_at = datetime('now') WHERE id = ?",
		sessionKey, string(messagesJSON), taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("start planning - update: %v", err))
		http.Error(w, "Failed to start planning", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("planning_started", "Planning session started", nil, &taskID)
	writeJSON(w, map[string]any{
		"success":     true,
		"session_key": sessionKey,
		"messages":    messages,
		"note":        "Planning started. Poll GET endpoint for updates.",
	})
}

// handleAnswerMCPlanningQuestion submits an answer and progresses planning.
//
//	POST /api/mc/tasks/{id}/planning/answer
func (h *Handler) handleAnswerMCPlanningQuestion(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		QuestionID string `json:"question_id"`
		Answer     string `json:"answer"`
		OtherText  string `json:"other_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if input.Answer == "" {
		http.Error(w, "answer is required", http.StatusBadRequest)
		return
	}

	// Save answer to planning_questions if question_id provided
	if input.QuestionID != "" {
		answerText := input.Answer
		if input.Answer == "other" && input.OtherText != "" {
			answerText = "Other: " + input.OtherText
		}
		_, _ = h.mcDB.ExecContext(r.Context(),
			"UPDATE planning_questions SET answer = ?, answered_at = datetime('now') WHERE id = ? AND task_id = ?",
			answerText, input.QuestionID, taskID,
		)
	}

	// Update planning messages with user answer
	var planningMessages sql.NullString
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT planning_messages FROM tasks WHERE id = ?", taskID,
	).Scan(&planningMessages)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("answer planning: %v", err))
		http.Error(w, "Failed to submit answer", http.StatusInternalServerError)
		return
	}

	var messages []mcPlanningMessage
	if planningMessages.Valid && planningMessages.String != "" {
		_ = json.Unmarshal([]byte(planningMessages.String), &messages)
	}

	answerText := input.Answer
	if input.Answer == "other" && input.OtherText != "" {
		answerText = "Other: " + input.OtherText
	}
	messages = append(messages, mcPlanningMessage{
		Role:      "user",
		Content:   answerText,
		Timestamp: 0,
	})

	messagesJSON, _ := json.Marshal(messages)
	_, err = h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET planning_messages = ? WHERE id = ?",
		string(messagesJSON), taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("answer planning - update: %v", err))
		http.Error(w, "Failed to submit answer", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("planning_answer_submitted", "Planning answer submitted", nil, &taskID)
	writeJSON(w, map[string]any{
		"success":  true,
		"messages": messages,
		"note":     "Answer submitted. Poll GET endpoint for updates.",
	})
}

// handleApproveMCPlanning locks the planning spec and moves task to inbox.
//
//	POST /api/mc/tasks/{id}/planning/approve
func (h *Handler) handleApproveMCPlanning(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Check if spec already locked
	var existingCount int
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT COUNT(*) FROM planning_specs WHERE task_id = ?", taskID,
	).Scan(&existingCount)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("approve planning - check: %v", err))
		http.Error(w, "Failed to approve planning", http.StatusInternalServerError)
		return
	}
	if existingCount > 0 {
		http.Error(w, "Spec already locked", http.StatusBadRequest)
		return
	}

	// Get all answered questions
	rows, err := h.mcDB.QueryContext(r.Context(),
		"SELECT category, question, answer FROM planning_questions WHERE task_id = ? AND answer IS NOT NULL ORDER BY sort_order",
		taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("approve planning - questions: %v", err))
		http.Error(w, "Failed to approve planning", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var questions []planningQuestionRow
	for rows.Next() {
		var q planningQuestionRow
		if err = rows.Scan(&q.Category, &q.Question, &q.Answer); err != nil {
			continue
		}
		questions = append(questions, q)
	}

	// Get task info for spec generation
	var title, description string
	h.mcDB.QueryRowContext(r.Context(),
		"SELECT title, COALESCE(description, '') FROM tasks WHERE id = ?", taskID,
	).Scan(&title, &description)

	// Generate spec markdown
	specMarkdown := generateSpecMarkdown(title, description, questions)

	// Create spec record
	specID := uuid.New().String()
	_, err = h.mcDB.ExecContext(r.Context(),
		"INSERT INTO planning_specs (id, task_id, spec_markdown, locked_at) VALUES (?, ?, ?, datetime('now'))",
		specID, taskID, specMarkdown,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("approve planning - insert spec: %v", err))
		http.Error(w, "Failed to approve planning", http.StatusInternalServerError)
		return
	}

	// Update task description with spec and move to inbox
	_, err = h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET description = ?, status = 'inbox', updated_at = datetime('now') WHERE id = ?",
		specMarkdown, taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("approve planning - update task: %v", err))
		http.Error(w, "Failed to approve planning", http.StatusInternalServerError)
		return
	}

	// Log activity
	activityID := uuid.New().String()
	_, _ = h.mcDB.ExecContext(nilCtx(),
		"INSERT INTO task_activities (id, task_id, activity_type, message) VALUES (?, ?, 'status_changed', 'Planning complete - spec locked and moved to inbox')",
		activityID, taskID,
	)

	h.broadcastMCEvent("planning_approved", "Planning approved and spec locked", nil, &taskID)

	writeJSON(w, map[string]any{
		"success":       true,
		"spec_id":       specID,
		"spec_markdown": specMarkdown,
	})
}

// handleForceCompleteMCPlanning force-completes planning without full Q&A.
//
//	POST /api/mc/tasks/{id}/planning/force-complete
func (h *Handler) handleForceCompleteMCPlanning(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var input struct {
		SpecMarkdown string `json:"spec_markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if input.SpecMarkdown == "" {
		input.SpecMarkdown = "# Planning Force-Completed\n\nPlanning was force-completed without full Q&A."
	}

	// Create spec record
	specID := uuid.New().String()
	_, err := h.mcDB.ExecContext(r.Context(),
		"INSERT INTO planning_specs (id, task_id, spec_markdown, locked_at) VALUES (?, ?, ?, datetime('now'))",
		specID, taskID, input.SpecMarkdown,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("force-complete planning: %v", err))
		http.Error(w, "Failed to force-complete planning", http.StatusInternalServerError)
		return
	}

	// Update task
	_, err = h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET planning_complete = 1, status = 'inbox', updated_at = datetime('now') WHERE id = ?",
		taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("force-complete planning - update: %v", err))
		http.Error(w, "Failed to force-complete planning", http.StatusInternalServerError)
		return
	}

	// Log activity
	activityID := uuid.New().String()
	_, _ = h.mcDB.ExecContext(nilCtx(),
		"INSERT INTO task_activities (id, task_id, activity_type, message) VALUES (?, ?, 'status_changed', 'Planning force-completed')",
		activityID, taskID,
	)

	h.broadcastMCEvent("planning_force_completed", "Planning force-completed", nil, &taskID)
	writeJSON(w, map[string]any{"success": true, "spec_id": specID})
}

// handleRetryMCPlanningDispatch retries dispatching after planning.
//
//	POST /api/mc/tasks/{id}/planning/retry-dispatch
func (h *Handler) handleRetryMCPlanningDispatch(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Reset task to inbox for re-dispatch
	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET status = 'inbox', updated_at = datetime('now') WHERE id = ?",
		taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("retry dispatch: %v", err))
		http.Error(w, "Failed to retry dispatch", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("planning_retry_dispatch", "Planning retry dispatch", nil, &taskID)
	writeJSON(w, map[string]any{"success": true})
}

// handlePollMCPlanning polls the planning status for updates.
//
//	GET /api/mc/tasks/{id}/planning/poll
func (h *Handler) handlePollMCPlanning(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var sessionKey sql.NullString
	var planningMessages sql.NullString
	var planningComplete sql.NullBool
	err := h.mcDB.QueryRowContext(r.Context(),
		"SELECT planning_session_key, planning_messages, planning_complete FROM tasks WHERE id = ?",
		taskID,
	).Scan(&sessionKey, &planningMessages, &planningComplete)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		logger.ErrorC("mc", fmt.Sprintf("poll planning: %v", err))
		http.Error(w, "Failed to poll planning", http.StatusInternalServerError)
		return
	}

	var messages []mcPlanningMessage
	if planningMessages.Valid && planningMessages.String != "" {
		_ = json.Unmarshal([]byte(planningMessages.String), &messages)
	}

	writeJSON(w, map[string]any{
		"task_id":       taskID,
		"session_key":   sessionKey,
		"is_complete":   planningComplete.Valid && planningComplete.Bool,
		"message_count": len(messages),
		"messages":      messages,
	})
}

// handleCancelMCTaskPlanning cancels a planning session.
//
//	DELETE /api/mc/tasks/{id}/planning
func (h *Handler) handleCancelMCTaskPlanning(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	_, err := h.mcDB.ExecContext(r.Context(),
		"UPDATE tasks SET planning_session_key = NULL, planning_messages = NULL, planning_complete = 0, planning_spec = NULL, planning_agents = NULL, status = 'inbox', updated_at = datetime('now') WHERE id = ?",
		taskID,
	)
	if err != nil {
		logger.ErrorC("mc", fmt.Sprintf("cancel planning: %v", err))
		http.Error(w, "Failed to cancel planning", http.StatusInternalServerError)
		return
	}

	h.broadcastMCEvent("planning_cancelled", "Planning session cancelled", nil, &taskID)
	writeJSON(w, map[string]any{"success": true})
}

// generateSpecMarkdown creates a markdown spec from answered planning questions.
func generateSpecMarkdown(title, description string, questions []planningQuestionRow) string {
	lines := []string{
		"# " + title,
		"",
		"**Status:** SPEC LOCKED",
		"",
	}

	if description != "" {
		lines = append(lines, "## Original Request", description, "")
	}

	// Group by category
	categories := map[string][]struct {
		Question string
		Answer   string
	}{}
	var catOrder []string
	for _, q := range questions {
		if _, ok := categories[q.Category]; !ok {
			catOrder = append(catOrder, q.Category)
		}
		categories[q.Category] = append(categories[q.Category], struct {
			Question string
			Answer   string
		}{q.Question, q.Answer})
	}

	catLabels := map[string]string{
		"goal":        "Goal & Success Criteria",
		"audience":    "Target Audience",
		"scope":       "Scope",
		"design":      "Design & Visual",
		"content":     "Content",
		"technical":   "Technical Requirements",
		"timeline":    "Timeline",
		"constraints": "Constraints",
	}

	for _, cat := range catOrder {
		label := cat
		if l, ok := catLabels[cat]; ok {
			label = l
		}
		lines = append(lines, "## "+label, "")
		for _, q := range categories[cat] {
			lines = append(lines, "**"+q.Question+"**")
			lines = append(lines, "> "+q.Answer, "")
		}
	}

	lines = append(lines, "---", "*Spec locked at "+`{{timestamp}}`+"*")
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}