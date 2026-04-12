// Package missioncontrol provides LLM completion for autopilot features.
//
// The llm package provides in-process LLM completion using PicoClaw's provider system.
package missioncontrol

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sipeed/picoclaw/pkg/providers"
)

const (
	RoleSystem = "system"
	RoleUser   = "user"
	RoleAssistant = "assistant"
)

// LLMRequest represents a request to the LLM.
type LLMRequest struct {
	Prompt     string            `json:"prompt"`
	System     string            `json:"system,omitempty"`
	MaxTokens  int               `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Model      string            `json:"model,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// LLMResponse represents a response from the LLM.
type LLMResponse struct {
	Content      string          `json:"content"`
	Usage        *providers.UsageInfo `json:"usage,omitempty"`
	Model        string          `json:"model"`
	FinishReason string          `json:"finish_reason"`
	Duration     time.Duration   `json:"duration"`
}

// Complete sends a completion request to the LLM.
func Complete(ctx context.Context, req LLMRequest, provider providers.LLMProvider) (*LLMResponse, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	// Build messages
	messages := []providers.Message{
		{Role: RoleUser, Content: req.Prompt},
	}
	if req.System != "" {
		messages = append([]providers.Message{{Role: RoleSystem, Content: req.System}}, messages...)
	}

	// Build options
	options := make(map[string]any)
	options["max_tokens"] = req.MaxTokens
	options["temperature"] = req.Temperature
	if req.Model != "" {
		options["model"] = req.Model
	}

	start := time.Now()
	resp, err := provider.Chat(ctx, messages, nil, "", options)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	if resp.Content == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	llmResp := &LLMResponse{
		Content:      resp.Content,
		Usage:        resp.Usage,
		Model:        req.Model,
		FinishReason: resp.FinishReason,
		Duration:     time.Since(start),
	}

	return llmResp, nil
}

// CompleteJSON sends a completion request and parses JSON from the response.
// Uses multiple strategies to extract JSON from the response.
func CompleteJSON(ctx context.Context, req LLMRequest, provider providers.LLMProvider, target any) error {
	req.Temperature = 0.3 // Lower temperature for more deterministic JSON

	resp, err := Complete(ctx, req, provider)
	if err != nil {
		return err
	}

	// Strategy 1: Try parsing the entire response as JSON
	if err := json.Unmarshal([]byte(resp.Content), target); err == nil {
		return nil
	}

	// Strategy 2: Extract JSON from markdown code blocks
	content := resp.Content

	// Look for ```json...``` or ```...```
	startIdx := -1
	endIdx := -1

	// Find opening code block
	for _, marker := range []string{"```json", "```"} {
		idx := findString(content, marker)
		if idx != -1 && (startIdx == -1 || idx < startIdx) {
			startIdx = idx + len(marker)
			break
		}
	}

	if startIdx != -1 {
		// Find closing code block
		endIdx = findString(content[startIdx:], "```")
		if endIdx != -1 {
			endIdx += startIdx
		}
	}

	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		jsonStr := content[startIdx:endIdx]
		if err := json.Unmarshal([]byte(jsonStr), target); err == nil {
			return nil
		}
	}

	// Strategy 3: Look for JSON object boundaries
	if jsonStart := findString(content, "{"); jsonStart != -1 {
		// Find matching closing brace
		depth := 0
		inString := false
		escapeNext := false

		for i := jsonStart; i < len(content); i++ {
			c := content[i]

			if escapeNext {
				escapeNext = false
				continue
			}

			if c == '\\' {
				escapeNext = true
				continue
			}

			if c == '"' && !escapeNext {
				inString = !inString
				continue
			}

			if !inString {
				if c == '{' {
					depth++
				} else if c == '}' {
					depth--
					if depth == 0 {
						jsonStr := content[jsonStart : i+1]
						if err := json.Unmarshal([]byte(jsonStr), target); err == nil {
							return nil
						}
						break
					}
				}
			}
		}
	}

	// Strategy 4: Try JSON array if object failed
	if jsonStart := findString(content, "["); jsonStart != -1 {
		depth := 0
		inString := false
		escapeNext := false

		for i := jsonStart; i < len(content); i++ {
			c := content[i]

			if escapeNext {
				escapeNext = false
				continue
			}

			if c == '\\' {
				escapeNext = true
				continue
			}

			if c == '"' && !escapeNext {
				inString = !inString
				continue
			}

			if !inString {
				if c == '[' {
					depth++
				} else if c == ']' {
					depth--
					if depth == 0 {
						jsonStr := content[jsonStart : i+1]
						if err := json.Unmarshal([]byte(jsonStr), target); err == nil {
							return nil
						}
						break
					}
				}
			}
		}
	}

	return fmt.Errorf("could not extract valid JSON from LLM response: %s", truncateString(resp.Content, 200))
}

// CompleteWithRetry sends a completion request with exponential backoff retry.
func CompleteWithRetry(ctx context.Context, req LLMRequest, provider providers.LLMProvider, maxRetries int) (*LLMResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 5s, 10s, 20s
			delay := time.Duration(5*attempt) * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := Complete(ctx, req, provider)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Don't retry context errors
		if ctx.Err() != nil {
			return nil, lastErr
		}
	}

	return nil, fmt.Errorf("LLM completion failed after %d retries: %w", maxRetries, lastErr)
}

// findString finds the first occurrence of a substring in a string.
func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// truncateString truncates a string to a maximum length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
