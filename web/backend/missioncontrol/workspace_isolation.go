// Package missioncontrol provides workspace isolation for parallel builds.
//
// The workspace isolation package manages git worktrees, sandboxes, port allocation,
// and merge queues for parallel task execution.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Port range for workspace dev servers
const (
	PortMin        = 4200
	PortMax        = 4299
	WorktreeBase   = ".worktrees"
	SandboxBase    = ".sandboxes"
	WorktreeExpiry = 24 * time.Hour
)

var (
	portMutex    sync.RWMutex
	allocatedPorts = make(map[string]int) // taskID -> port
	portReverseMap = make(map[int]string)  // port -> taskID
)

// WorkspaceInfo represents information about a workspace.
type WorkspaceInfo struct {
	TaskID       string    `json:"task_id"`
	RepoURL      string    `json:"repo_url"`
	BaseBranch   string    `json:"base_branch"`
	WorktreePath string    `json:"worktree_path"`
	SandboxPath  string    `json:"sandbox_path"`
	Port         *int      `json:"port"`
	Status       string    `json:"status"` // pending, setup, active, abandoned, merged
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

// AllocatePort allocates a port for a workspace.
func AllocatePort(ctx context.Context, db *DB, taskID, productID string) (int, error) {
	portMutex.Lock()
	defer portMutex.Unlock()

	// Check if already allocated
	if port, ok := allocatedPorts[taskID]; ok {
		return port, nil
	}

	// Check database for existing allocation
	var existingPort sql.NullInt64
	err := db.QueryRowContext(ctx,
		"SELECT port FROM workspace_ports WHERE task_id = ?",
		taskID,
	).Scan(&existingPort)
	if err == nil && existingPort.Valid {
		port := int(existingPort.Int64)
		allocatedPorts[taskID] = port
		portReverseMap[port] = taskID
		return port, nil
	}

	// Find an available port
	for port := PortMin; port <= PortMax; port++ {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM workspace_ports WHERE port = ? AND released_at IS NULL",
			port,
		).Scan(&count)

		if err == nil && count == 0 {
			// Also check in-memory map
			if _, occupied := portReverseMap[port]; !occupied {
				// Allocate the port
				portID := uuid.New().String()
				_, err = db.ExecContext(ctx,
					"INSERT INTO workspace_ports (id, task_id, product_id, port, allocated_at) VALUES (?, ?, ?, ?, datetime('now'))",
					portID, taskID, productID, port,
				)
				if err != nil {
					return 0, fmt.Errorf("insert port allocation: %w", err)
				}

				allocatedPorts[taskID] = port
				portReverseMap[port] = taskID
				return port, nil
			}
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", PortMin, PortMax)
}

// ReleasePort releases a previously allocated port.
func ReleasePort(ctx context.Context, db *DB, taskID string) error {
	portMutex.Lock()
	defer portMutex.Unlock()

	port, ok := allocatedPorts[taskID]
	if !ok {
		return nil // Already released or never allocated
	}

	// Update database
	_, err := db.ExecContext(ctx,
		"UPDATE workspace_ports SET released_at = datetime('now') WHERE task_id = ?",
		taskID,
	)
	if err != nil {
		return fmt.Errorf("update port release: %w", err)
	}

	delete(allocatedPorts, taskID)
	delete(portReverseMap, port)
	return nil
}

// SetupWorktree creates a git worktree for a task.
func SetupWorktree(ctx context.Context, db *DB, taskID, repoURL, baseBranch string) (string, error) {
	if repoURL == "" {
		return "", fmt.Errorf("repo URL is required")
	}

	// Get pico home directory for worktrees base
	picoHome, err := GetPicoHome()
	if err != nil {
		return "", fmt.Errorf("get pico home: %w", err)
	}

	worktreesDir := filepath.Join(picoHome, WorktreeBase)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("create worktrees directory: %w", err)
	}

	// Clone repo if not already exists
	repoName := filepath.Base(repoURL)
	if ext := filepath.Ext(repoName); ext == ".git" {
		repoName = repoName[:len(repoName)-4]
	}
	bareRepoPath := filepath.Join(worktreesDir, repoName+".bare")

	// Initialize bare repo if needed
	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		cmd := exec.Command("git", "clone", "--mirror", repoURL, bareRepoPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("clone mirror repo: %w, output: %s", err, string(output))
		}
	}

	// Fetch latest from origin
	cmd := exec.Command("git", "--git-dir", bareRepoPath, "fetch", "origin")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("fetch updates: %w, output: %s", err, string(output))
	}

	// Create worktree
	worktreeName := fmt.Sprintf("%s-%s", taskID[:8], uuid.New().String()[:8])
	worktreePath := filepath.Join(worktreesDir, worktreeName)

	branch := baseBranch
	if branch == "" {
		branch = "main"
	}

	cmd = exec.Command("git", "--git-dir", bareRepoPath, "worktree", "add",
		"-b", fmt.Sprintf("task/%s", worktreeName),
		worktreePath,
		"origin/"+branch,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create worktree: %w, output: %s", err, string(output))
	}

	// Record in database
	expiresAt := time.Now().Add(WorktreeExpiry)
	_, err = db.ExecContext(ctx,
		`INSERT INTO workspace_merges
		(id, task_id, worktree_path, base_branch, status, created_at, expires_at)
		VALUES (?, ?, ?, ?, 'setup', datetime('now'), ?)`,
		uuid.New().String(), taskID, worktreePath, baseBranch, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		// Clean up worktree on failure
		_ = os.RemoveAll(worktreePath)
		return "", fmt.Errorf("record worktree: %w", err)
	}

	return worktreePath, nil
}

// SetupSandbox creates a sandbox copy of a workspace.
func SetupSandbox(ctx context.Context, db *DB, taskID, sourcePath string) (string, error) {
	if sourcePath == "" {
		return "", fmt.Errorf("source path is required")
	}

	picoHome, err := GetPicoHome()
	if err != nil {
		return "", fmt.Errorf("get pico home: %w", err)
	}

	sandboxesDir := filepath.Join(picoHome, SandboxBase)
	if err := os.MkdirAll(sandboxesDir, 0755); err != nil {
		return "", fmt.Errorf("create sandboxes directory: %w", err)
	}

	sandboxName := fmt.Sprintf("%s-%s", taskID[:8], uuid.New().String()[:8])
	sandboxPath := filepath.Join(sandboxesDir, sandboxName)

	// Use rsync for efficient copy
	cmd := exec.Command("rsync", "-a", "--delete", sourcePath+"/", sandboxPath+"/")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("rsync sandbox: %w, output: %s", err, string(output))
	}

	// Update database
	_, err = db.ExecContext(ctx,
		"UPDATE workspace_merges SET sandbox_path = ?, status = 'active' WHERE task_id = ?",
		sandboxPath, taskID,
	)
	if err != nil {
		// Clean up sandbox on failure
		_ = os.RemoveAll(sandboxPath)
		return "", fmt.Errorf("update sandbox path: %w", err)
	}

	return sandboxPath, nil
}

// GetWorkspaceStatus retrieves the status of a workspace.
func GetWorkspaceStatus(ctx context.Context, db *DB, taskID string) (*WorkspaceInfo, error) {
	var info WorkspaceInfo
	var expiresAtStr sql.NullString

	err := db.QueryRowContext(ctx,
		`SELECT task_id, worktree_path, base_branch, sandbox_path, status, created_at, expires_at
		FROM workspace_merges WHERE task_id = ?`,
		taskID,
	).Scan(&info.TaskID, &info.WorktreePath, &info.BaseBranch, &info.SandboxPath,
		&info.Status, &info.CreatedAt, &expiresAtStr)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No workspace found
		}
		return nil, fmt.Errorf("query workspace status: %w", err)
	}

	if expiresAtStr.Valid {
		t, err := time.Parse(time.RFC3339, expiresAtStr.String)
		if err == nil {
			info.ExpiresAt = &t
		}
	}

	// Get port if allocated
	portMutex.RLock()
	if port, ok := allocatedPorts[taskID]; ok {
		info.Port = &port
	}
	portMutex.RUnlock()

	return &info, nil
}

// MergeWorkspace merges a workspace back to the main branch via PR.
func MergeWorkspace(ctx context.Context, db *DB, taskID string) (string, error) {
	info, err := GetWorkspaceStatus(ctx, db, taskID)
	if err != nil {
		return "", err
	}
	if info == nil {
		return "", fmt.Errorf("no workspace found for task %s", taskID)
	}

	// Check if gh CLI is available
	_, err = exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh CLI not found: %w", err)
	}

	// Create PR using gh
	branchName := fmt.Sprintf("task/%s", filepath.Base(info.WorktreePath))
	prTitle := fmt.Sprintf("Task %s: Automated PR", taskID[:8])
	prBody := fmt.Sprintf("Automated PR for task %s\n\nWorkspace: %s", taskID, info.WorktreePath)

	repoURL, err := getRepoURL(info.WorktreePath)
	if err != nil {
		return "", fmt.Errorf("get repo URL: %w", err)
	}

	cmd := exec.Command("gh", "pr", "create",
		"--base", info.BaseBranch,
		"--head", branchName,
		"--title", prTitle,
		"--body", prBody,
		"--repo", repoURL,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create PR: %w, output: %s", err, string(output))
	}

	prURL := string(output)

	// Update status
	_, err = db.ExecContext(ctx,
		"UPDATE workspace_merges SET status = 'merged', pr_url = ? WHERE task_id = ?",
		prURL, taskID,
	)
	if err != nil {
		return "", fmt.Errorf("update merge status: %w", err)
	}

	return prURL, nil
}

// AbandonWorkspace abandons a workspace and cleans up resources.
func AbandonWorkspace(ctx context.Context, db *DB, taskID string) error {
	info, err := GetWorkspaceStatus(ctx, db, taskID)
	if err != nil {
		return err
	}
	if info == nil {
		return nil // Nothing to abandon
	}

	// Release port
	if err := ReleasePort(ctx, db, taskID); err != nil {
		return fmt.Errorf("release port: %w", err)
	}

	// Remove worktree
	if info.WorktreePath != "" {
		if err := os.RemoveAll(info.WorktreePath); err != nil {
			return fmt.Errorf("remove worktree: %w", err)
		}
	}

	// Remove sandbox
	if info.SandboxPath != "" {
		if err := os.RemoveAll(info.SandboxPath); err != nil {
			return fmt.Errorf("remove sandbox: %w", err)
		}
	}

	// Update database
	_, err = db.ExecContext(ctx,
		"UPDATE workspace_merges SET status = 'abandoned' WHERE task_id = ?",
		taskID,
	)
	if err != nil {
		return fmt.Errorf("update abandon status: %w", err)
	}

	return nil
}

// GetPendingWorkspacesForProduct returns pending workspace merges for a product.
func GetPendingWorkspacesForProduct(ctx context.Context, db *DB, productID string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT task_id FROM workspace_merges wm
		JOIN tasks t ON t.id = wm.task_id
		WHERE t.product_id = ? AND wm.status IN ('pending', 'setup', 'active')
		ORDER BY wm.created_at ASC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending workspaces: %w", err)
	}
	defer rows.Close()

	var taskIDs []string
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err != nil {
			continue
		}
		taskIDs = append(taskIDs, taskID)
	}

	return taskIDs, nil
}

// getRepoURL extracts the repo URL from a git directory.
func getRepoURL(worktreePath string) (string, error) {
	cmd := exec.Command("git", "-C", worktreePath, "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get remote origin for %s: %w", worktreePath, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetPicoHome returns the PicoClaw home directory.
func GetPicoHome() (string, error) {
	// Try environment variable first
	if home := os.Getenv("PICOCLAW_HOME"); home != "" {
		return home, nil
	}

	// Try default location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	picoHome := filepath.Join(homeDir, ".picoclaw")
	return picoHome, nil
}
