package database

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
)

// MigrationBootstrap is applied separately before reading migration versions.
const MigrationBootstrap = "migrations/20260503000_schema_migrations.sql"

// MigrationVersions is implemented by sqlc queries bound to the migration transaction.
type MigrationVersions interface {
	GetSchemaMigrationVersion(context.Context) (int64, error)
	SetSchemaMigrationVersion(context.Context, int64) error
}

type Migration struct {
	Version int64
	Name    string
	SQL     string
}

func ReadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return nil, err
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		versionPart, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}

		version, err := strconv.ParseInt(versionPart, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}

		body, err := fs.ReadFile(fsys, path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    entry.Name(),
			SQL:     string(body),
		})
	}

	slices.SortFunc(migrations, func(a, b Migration) int {
		return cmp.Compare(a.Version, b.Version)
	})

	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].Version == migrations[i].Version {
			return nil, fmt.Errorf("duplicate migration version %d", migrations[i].Version)
		}
	}

	return migrations, nil
}

// ApplyMigrations applies pending files and records each version. The caller must
// already hold the backend lock and initialize bookkeeping, then commit or roll
// back the entire transaction. MySQL uses a separate runner for implicit DDL
// commits and its persistent pending marker.
func ApplyMigrations(ctx context.Context, files fs.FS, versions MigrationVersions, exec func(context.Context, string) error) error {
	migrations, err := ReadMigrations(files)
	if err != nil {
		return err
	}
	currentVersion, err := versions.GetSchemaMigrationVersion(ctx)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}
		if err := exec(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply migration %q: %w", migration.Name, err)
		}
		if err := versions.SetSchemaMigrationVersion(ctx, migration.Version); err != nil {
			return fmt.Errorf("set migration version %d: %w", migration.Version, err)
		}
	}
	return nil
}
