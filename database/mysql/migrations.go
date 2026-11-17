package mysql

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"sifeucr/database"
	"sifeucr/database/mysql/db"
)

const (
	migrationLockName    = "sifeucr:migrations"
	migrationLockTimeout = 10
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func applyMigrations(ctx context.Context, cfg *mysqldriver.Config) error {
	// Migration files contain multiple statements. Never return this connection
	// to the runtime pool, including after failed or interrupted migrations.
	cfg = cfg.Clone()
	cfg.MultiStatements = true
	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		return err
	}
	conn := sql.OpenDB(connector)
	conn.SetMaxOpenConns(1)
	defer func() { _ = conn.Close() }()
	if err := database.Ping(ctx, conn.PingContext); err != nil {
		return err
	}
	c, err := conn.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	// GET_LOCK is session-scoped, so every migration statement must run on this
	// connection. MySQL DDL commits implicitly, so a transaction cannot wrap
	// the whole batch.
	var locked sql.NullInt64
	if err := c.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", migrationLockName, migrationLockTimeout).Scan(&locked); err != nil {
		return fmt.Errorf("lock mysql migrations: %w", err)
	}
	if !locked.Valid || locked.Int64 != 1 {
		return fmt.Errorf("mysql migrations are already running")
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = c.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", migrationLockName)
	}()

	if err := ensureMigrationTable(ctx, c); err != nil {
		return err
	}

	migrations, err := database.ReadMigrations(migrationFiles)
	if err != nil {
		return err
	}

	queries := db.New(c)
	pending, err := queries.GetPendingSchemaMigration(ctx)
	if err != nil {
		return err
	}
	if pending != 0 {
		return fmt.Errorf("mysql migration %d is incomplete; inspect and repair the database before clearing schema_migration_pending (see AGENTS.md: MySQL migration recovery)", pending)
	}
	currentVersion, err := queries.GetSchemaMigrationVersion(ctx)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		// Commit the marker before DDL, which may partially commit on failure.
		if err := migrationMetadataTx(ctx, c, func(q *db.Queries) error {
			return q.SetPendingSchemaMigration(ctx, migration.Version)
		}); err != nil {
			return fmt.Errorf("mark mysql migration %d pending: %w", migration.Version, err)
		}
		if _, err := c.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply mysql migration %q: %w", migration.Name, err)
		}

		if err := migrationMetadataTx(ctx, c, func(q *db.Queries) error {
			if err := q.SetSchemaMigrationVersion(ctx, migration.Version); err != nil {
				return err
			}
			return q.ClearPendingSchemaMigration(ctx)
		}); err != nil {
			return fmt.Errorf("finish mysql migration %d: %w", migration.Version, err)
		}
	}

	return nil
}

func migrationMetadataTx(ctx context.Context, conn *sql.Conn, fn func(*db.Queries) error) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(db.New(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureMigrationTable(ctx context.Context, conn *sql.Conn) error {
	schema, err := migrationFiles.ReadFile(database.MigrationBootstrap)
	if err != nil {
		return err
	}

	_, err = conn.ExecContext(ctx, string(schema))
	return err
}
