// Package missioncontrol provides the database layer for the Mission Control
// product-autopilot engine embedded in the PicoClaw launcher.
//
// It manages a dedicated SQLite database (mission-control.db) separate from
// the launcher auth store, following the same pattern established by
// dashboardauth.
package missioncontrol

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

const (
	// DBFilename is the canonical filename for the Mission Control database.
	DBFilename = "mission-control.db"

	sqliteDriver = "sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// DB wraps a *sql.DB with Mission Control query helpers.
type DB struct {
	*sql.DB
	path string
}

// Open creates (or opens) the Mission Control database at dir/DBFilename,
// enables WAL mode and foreign keys, then executes the full schema.
func Open(dir string) (*DB, error) {
	path := filepath.Join(dir, DBFilename)
	db, err := sql.Open(sqliteDriver, path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}

	// Apply schema
	schemaBytes, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read schema: %w", err)
	}
	if _, err = db.Exec(string(schemaBytes)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &DB{DB: db, path: path}, nil
}

// Close releases the database handle.
func (d *DB) Close() error { return d.DB.Close() }

// Path returns the absolute path to the SQLite database file.
func (d *DB) Path() string { return d.path }

// QueryOne scans a single row into dest. Returns sql.ErrNoRows if not found.
func (d *DB) QueryOne(ctx context.Context, query string, dest ...interface{}) error {
	return d.DB.QueryRowContext(ctx, query, dest...).Scan(dest...)
}

// Run executes a statement with no return rows.
func (d *DB) Run(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.DB.ExecContext(ctx, query, args...)
}

// RunTx executes fn inside a database transaction, committing on success
// and rolling back on error.
func (d *DB) RunTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}