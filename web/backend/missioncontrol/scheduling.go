// Package missioncontrol provides scheduling for autopilot recurring tasks.
package missioncontrol

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ScheduleType is the type of scheduled automation.
type ScheduleType string

const (
	ScheduleResearch       ScheduleType = "research"
	ScheduleIdeation       ScheduleType = "ideation"
	ScheduleMaybeReeval    ScheduleType = "maybe_reevaluation"
	ScheduleSEOAudit       ScheduleType = "seo_audit"
	ScheduleContentRefresh ScheduleType = "content_refresh"
	ScheduleAnalytics     ScheduleType = "analytics_report"
	ScheduleSocialBatch    ScheduleType = "social_batch"
	ScheduleGrowthExp     ScheduleType = "growth_experiment"
)

// Schedule represents a recurring automation schedule.
type Schedule struct {
	ID            string       `json:"id"`
	ProductID     string       `json:"product_id"`
	ScheduleType  ScheduleType `json:"schedule_type"`
	CronExpression string      `json:"cron_expression"`
	Timezone      string       `json:"timezone"`
	Enabled       bool         `json:"enabled"`
	LastRunAt     *time.Time   `json:"last_run_at,omitempty"`
	NextRunAt     *time.Time   `json:"next_run_at,omitempty"`
	Config        string       `json:"config,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// CreateSchedule creates a new schedule.
func CreateSchedule(ctx context.Context, db *DB, productID string, schedType ScheduleType, cronExpr, timezone string, config string) (*Schedule, error) {
	schedule := &Schedule{
		ID:            uuid.New().String(),
		ProductID:     productID,
		ScheduleType:  schedType,
		CronExpression: cronExpr,
		Timezone:      timezone,
		Enabled:       true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Config:        config,
	}

	// Compute next run time
	next := computeNextRun(cronExpr, timezone)
	schedule.NextRunAt = &next

	_, err := db.ExecContext(ctx,
		`INSERT INTO product_schedules
		(id, product_id, schedule_type, cron_expression, timezone, enabled, next_run_at, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		schedule.ID, schedule.ProductID, schedule.ScheduleType, schedule.CronExpression,
		schedule.Timezone, 1, schedule.NextRunAt.Format(time.RFC3339), schedule.Config,
		schedule.CreatedAt.Format(time.RFC3339), schedule.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}

	return schedule, nil
}

// ListSchedules returns all schedules for a product.
func ListSchedules(ctx context.Context, db *DB, productID string) ([]Schedule, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, schedule_type, cron_expression, timezone, enabled,
		last_run_at, next_run_at, config, created_at, updated_at
		FROM product_schedules WHERE product_id = ? ORDER BY created_at DESC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()

	var schedules []Schedule
	for rows.Next() {
		var s Schedule
		var lastRun, nextRun sql.NullString
		var enabledInt int

		if err := rows.Scan(&s.ID, &s.ProductID, &s.ScheduleType, &s.CronExpression,
			&s.Timezone, &enabledInt, &lastRun, &nextRun, &s.Config, &s.CreatedAt, &s.UpdatedAt); err != nil {
			continue
		}
		s.Enabled = enabledInt == 1
		if lastRun.Valid {
			t, _ := time.Parse(time.RFC3339, lastRun.String)
			s.LastRunAt = &t
		}
		if nextRun.Valid {
			t, _ := time.Parse(time.RFC3339, nextRun.String)
			s.NextRunAt = &t
		}
		schedules = append(schedules, s)
	}
	if schedules == nil {
		schedules = []Schedule{}
	}
	return schedules, nil
}

// UpdateSchedule updates a schedule's fields.
func UpdateSchedule(ctx context.Context, db *DB, scheduleID string, cronExpr *string, timezone *string, enabled *bool, config *string) error {
	setClauses := []string{"updated_at = datetime('now')"}
	args := []interface{}{}

	if cronExpr != nil {
		setClauses = append(setClauses, "cron_expression = ?")
		args = append(args, *cronExpr)
		// Recompute next run
		next := computeNextRun(*cronExpr, "America/Denver")
		setClauses = append(setClauses, "next_run_at = ?")
		args = append(args, next.Format(time.RFC3339))
	}
	if timezone != nil {
		setClauses = append(setClauses, "timezone = ?")
		args = append(args, *timezone)
	}
	if enabled != nil {
		setClauses = append(setClauses, "enabled = ?")
		args = append(args, boolToInt(*enabled))
	}
	if config != nil {
		setClauses = append(setClauses, "config = ?")
		args = append(args, *config)
	}

	query := "UPDATE product_schedules SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = ?"
	args = append(args, scheduleID)

	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	return nil
}

// DeleteSchedule deletes a schedule.
func DeleteSchedule(ctx context.Context, db *DB, scheduleID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM product_schedules WHERE id = ?`, scheduleID)
	if err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}
	return nil
}

// cronMatches checks if a cron expression matches a given time.
// Supports: * (any), */N (every N), comma-separated lists.
func cronMatches(cronExpr string, t time.Time) bool {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return false
	}

	// minute, hour, day-of-month, month, day-of-week
	return matchesField(fields[0], t.Minute()) &&
		matchesField(fields[1], t.Hour()) &&
		matchesField(fields[2], t.Day()) &&
		matchesField(fields[3], int(t.Month())) &&
		matchesField(fields[4], int(t.Weekday()))
}

func matchesField(expr string, value int) bool {
	if expr == "*" {
		return true
	}

	// Handle */N (step)
	if strings.HasPrefix(expr, "*/") {
		step, err := strconv.Atoi(expr[2:])
		if err != nil || step == 0 {
			return false
		}
		return value%step == 0
	}

	// Handle comma-separated list
	for _, part := range strings.Split(expr, ",") {
		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(part[2:])
			if err == nil && step > 0 && value%step == 0 {
				return true
			}
		} else {
			// Single value or range
			if strings.Contains(part, "-") {
				rangeParts := strings.Split(part, "-")
				if len(rangeParts) == 2 {
					start, _ := strconv.Atoi(rangeParts[0])
					end, _ := strconv.Atoi(rangeParts[1])
					if value >= start && value <= end {
						return true
					}
				}
			} else {
				v, err := strconv.Atoi(part)
				if err == nil && v == value {
					return true
				}
			}
		}
	}

	return false
}

func computeNextRun(cronExpr string, timezone string) time.Time {
	// Parse timezone (simplified - defaults to UTC)
	loc := time.UTC
	if timezone != "" {
		if tz, err := time.LoadLocation(timezone); err == nil {
			loc = tz
		}
	}

	now := time.Now().In(loc)

	// Try next 1440 minutes (1 day) to find a match
	for i := 1; i <= 1440; i++ {
		candidate := now.Add(time.Duration(i) * time.Minute)
		if cronMatches(cronExpr, candidate) {
			return candidate
		}
	}

	// Default: 1 hour from now
	return now.Add(time.Hour)
}

// CheckAndRunDueSchedules finds and runs all due schedules.
func CheckAndRunDueSchedules(ctx context.Context, db *DB, triggerFn func(ctx context.Context, db *DB, productID string, schedType ScheduleType)) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, product_id, schedule_type, cron_expression, last_run_at, next_run_at
		FROM product_schedules
		WHERE enabled = 1
		AND (next_run_at IS NULL OR next_run_at <= datetime('now'))`,
	)
	if err != nil {
		return fmt.Errorf("find due schedules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, productID, schedType, cronExpr string
		var lastRun, nextRun sql.NullString

		if err := rows.Scan(&id, &productID, &schedType, &cronExpr, &lastRun, &nextRun); err != nil {
			continue
		}

		// Check if cron matches current time
		now := time.Now()
		if !cronMatches(cronExpr, now) {
			continue
		}

		// Check if we ran recently (within 60 seconds)
		if lastRun.Valid {
			lastRunTime, err := time.Parse(time.RFC3339, lastRun.String)
			if err == nil && now.Sub(lastRunTime) < 60*time.Second {
				continue
			}
		}

		// Trigger the schedule
		if triggerFn != nil {
			triggerFn(ctx, db, productID, ScheduleType(schedType))
		}

		// Update last_run_at and next_run_at
		next := computeNextRun(cronExpr, "America/Denver")
		db.ExecContext(ctx,
			`UPDATE product_schedules
			SET last_run_at = ?, next_run_at = ?, updated_at = datetime('now')
			WHERE id = ?`,
			now.Format(time.RFC3339), next.Format(time.RFC3339), id,
		)
	}

	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}