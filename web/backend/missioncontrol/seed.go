// Package missioncontrol provides the database layer and business logic for the
// Mission Control product-autopilot engine embedded in the PicoClaw launcher.
//
// The seed package initializes default data when the database is first created.
package missioncontrol

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SeedDefaultAgents creates the default Mission Control agents.
// This should be called after database initialization if no agents exist.
func SeedDefaultAgents(ctx context.Context, db *DB) error {
	// Check if agents already exist
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agents").Scan(&count)
	if err != nil {
		return fmt.Errorf("check agents: %w", err)
	}
	if count > 0 {
		return nil // Already seeded
	}

	// Create default workspace if not exists
	_, err = db.ExecContext(ctx,
		"INSERT OR IGNORE INTO workspaces (id, name, slug, description, icon, created_at, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))",
		"default", "Default", "default", "Default workspace", "📁",
	)
	if err != nil {
		return fmt.Errorf("create default workspace: %w", err)
	}

	// Orchestrator agent (master)
	orchestratorID := uuid.New().String()
	orchestratorSoulMD := `# Mission Control Orchestrator

You are the master orchestrator of Mission Control. You lead a team of AI agents working together to complete tasks.

## Core Identity

- **Role**: Team Lead & Orchestrator
- **Personality**: Calm, strategic, supportive, decisive
- **Communication Style**: Clear, encouraging, direct when needed

## Responsibilities

1. **Task Coordination**: Receive tasks, analyze requirements, delegate to appropriate team members
2. **Team Support**: Check on agents, help when stuck, celebrate wins
3. **Quality Control**: Review work before marking complete
4. **Communication Hub**: Facilitate agent-to-agent collaboration

## Decision Framework

When a new task arrives:
1. Assess complexity and required skills
2. Check agent availability and expertise
3. Assign to best-fit agent(s)
4. Set clear expectations and deadlines
5. Monitor progress and offer support
`

	_, err = db.ExecContext(ctx,
		"INSERT INTO agents (id, name, role, description, avatar_emoji, status, is_master, soul_md, user_md, agents_md, workspace_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))",
		orchestratorID, "Orchestrator", "Team Lead & Orchestrator", "The master orchestrator who coordinates all agents and manages the mission queue",
		"🦞", "standby", 1,
		orchestratorSoulMD, "", "", "default",
	)
	if err != nil {
		return fmt.Errorf("create orchestrator: %w", err)
	}

	// Create team agents
	agents := []struct {
		name   string
		role   string
		desc   string
		emoji  string
	}{
		{"Developer", "Code & Automation", "Writes code, creates automations, handles technical tasks", "💻"},
		{"Researcher", "Research & Analysis", "Gathers information, analyzes data, provides insights", "🔍"},
		{"Writer", "Content & Documentation", "Creates content, writes documentation, handles communications", "✍️"},
		{"Designer", "Creative & Design", "Handles visual design, UX decisions, creative work", "🎨"},
	}

	for _, agent := range agents {
		id := uuid.New().String()
		_, err = db.ExecContext(ctx,
			"INSERT INTO agents (id, name, role, description, avatar_emoji, status, is_master, workspace_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))",
			id, agent.name, agent.role, agent.desc, agent.emoji, "standby", 0, "default",
		)
		if err != nil {
			return fmt.Errorf("create agent %s: %w", agent.name, err)
		}
	}

	// Create default workflow templates
	workflowTemplates := []struct {
		name     string
		stages   string
		category string
	}{
		{
			name:   "Simple",
			stages: `["assigned", "in_progress", "done"]`,
			category: "simple",
		},
		{
			name:   "Standard",
			stages: `["assigned", "in_progress", "testing", "review", "done"]`,
			category: "standard",
		},
		{
			name:   "Strict",
			stages: `["assigned", "in_progress", "testing", "review", "verification", "done"]`,
			category: "strict",
		},
	}

	for _, wt := range workflowTemplates {
		id := uuid.New().String()
		_, err = db.ExecContext(ctx,
			"INSERT INTO workflow_templates (id, workspace_id, name, stages, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))",
			id, "default", wt.name, wt.stages, wt.category,
		)
		if err != nil {
			return fmt.Errorf("create workflow template %s: %w", wt.name, err)
		}
	}

	return nil
}
