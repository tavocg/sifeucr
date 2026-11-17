// Package testdb supplies isolated databases for integration tests.
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"sifeucr/cache/memory"
	"sifeucr/database"
	"sifeucr/database/cached"
	"sifeucr/database/provider"
)

// Run always tests SQLite. External backends require an administrative test DSN:
// TEST_POSTGRES_DSN (URL) or TEST_MYSQL_DSN (driver DSN), with CREATE DATABASE rights.
// Each case creates and drops only its own randomly named database.
func Run(t *testing.T, test func(*testing.T, database.Database)) {
	t.Helper()
	run(t, test, true)
}

// RunUncached supplies a raw database for tests of the cache wrapper itself.
func RunUncached(t *testing.T, test func(*testing.T, database.Database)) {
	t.Helper()
	run(t, test, false)
}

func run(t *testing.T, test func(*testing.T, database.Database), cacheReads bool) {
	t.Helper()
	for _, backend := range []string{"sqlite", "postgres", "mysql"} {
		t.Run(backend, func(t *testing.T) { test(t, open(t, backend, cacheReads)) })
	}
}

func open(t *testing.T, backend string, cacheReads bool) database.Database {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "sifeucr.sqlite")
	if backend != "sqlite" {
		env := "TEST_POSTGRES_DSN"
		if backend == "mysql" {
			env = "TEST_MYSQL_DSN"
		}
		dsn = os.Getenv(env)
		if dsn == "" {
			t.Skipf("set %s to run this backend", env)
		}
		name := "sifeucr_test_" + rand.Text()
		driver, quoted := "pgx", `"`+name+`"`
		if backend == "mysql" {
			driver, quoted = "mysql", "`"+name+"`"
		}
		admin, err := sql.Open(driver, dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { checkClose(t, admin.Close()) })
		if _, err := admin.ExecContext(t.Context(), "CREATE DATABASE "+quoted); err != nil {
			t.Fatalf("create isolated database: %v", err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := admin.ExecContext(ctx, "DROP DATABASE "+quoted); err != nil {
				t.Errorf("drop isolated database: %v", err)
			}
		})
		if backend == "postgres" {
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatal(err)
			}
			u.Path = "/" + name
			dsn = u.String()
		} else {
			cfg, err := mysql.ParseDSN(dsn)
			if err != nil {
				t.Fatal(err)
			}
			cfg.DBName = name
			dsn = cfg.FormatDSN()
		}
	}
	db, err := provider.Open(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { checkClose(t, db.Close()) })
	if !cacheReads {
		return db
	}
	store := memory.New()
	t.Cleanup(func() { checkClose(t, store.Close()) })
	return cached.New(db, store)
}

func checkClose(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("close test resource: %v", err)
	}
}
