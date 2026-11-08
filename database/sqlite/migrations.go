package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"sifeucr/database"
	"sifeucr/database/sqlite/db"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func applyMigrations(ctx context.Context, conn *sql.DB) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureMigrationTable(ctx, tx); err != nil {
		return err
	}
	// Acquire SQLite's writer lock even if the table already exists or is empty.
	if _, err := tx.ExecContext(ctx, "UPDATE schema_migrations SET version = version WHERE id = 1"); err != nil {
		return fmt.Errorf("lock sqlite migrations: %w", err)
	}

	if err := database.ApplyMigrations(ctx, migrationFiles, db.New(tx), func(ctx context.Context, statement string) error {
		_, err := tx.ExecContext(ctx, statement)
		return err
	}); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}

	return tx.Commit()
}

func ensureMigrationTable(ctx context.Context, tx *sql.Tx) error {
	schema, err := migrationFiles.ReadFile(database.MigrationBootstrap)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, string(schema))
	return err
}
