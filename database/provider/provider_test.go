package provider

import (
	"context"
	"database/sql"
	"path/filepath"
	"sifeucr/database"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenAutoMigratesSQLite(t *testing.T) {
	t.Parallel()

	connStr := filepath.Join(t.TempDir(), "sifeucr.sqlite")

	dbi, err := Open(context.Background(), connStr)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := dbi.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}()

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		t.Fatalf("open sqlite file: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close sqlite file: %v", err)
		}
	}()

	var version int64
	if err := db.QueryRow(`SELECT version FROM schema_migrations WHERE id = 1`).Scan(&version); err != nil {
		t.Fatalf("read schema_migrations version: %v", err)
	}

	if version != 20261118004 {
		t.Fatalf("got schema version %d, want 20261118004", version)
	}

	var accountColumns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('accounts')`).Scan(&accountColumns); err != nil {
		t.Fatalf("read accounts schema: %v", err)
	}

	if accountColumns == 0 {
		t.Fatal("expected accounts table to exist after auto-migration")
	}
}

func TestOpenChecksSchemaWithoutMigrating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sifeucr.sqlite")
	options := database.OpenOptions{SkipMigrations: true}
	if db, err := Open(t.Context(), path, options); err == nil {
		db.Close()
		t.Fatal("accepted an uninitialized schema")
	}
	db, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = Open(t.Context(), path, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	db.Close()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	for _, version := range []int64{0, 99999999999} {
		if _, err := raw.Exec("UPDATE schema_migrations SET version = ?", version); err != nil {
			t.Fatal(err)
		}
		if db, err := Open(t.Context(), path, options); err == nil {
			db.Close()
			t.Fatalf("accepted incompatible version %d", version)
		}
	}
}
