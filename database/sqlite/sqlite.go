// Package sqlite implements the database interface with SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"

	"sifeucr/database"
	"sifeucr/database/sqlite/db"
	"sifeucr/database/sqlite/queries"

	_ "modernc.org/sqlite"
)

type SQLite struct {
	db      *sql.DB
	queries *db.Queries
}

// OptFunc configures DSN parameters applied to every new connection.
type OptFunc func(url.Values)

var _ database.Database = (*SQLite)(nil)

func NewSQLite(ctx context.Context, connStr string, opts ...OptFunc) (*SQLite, error) {
	filename, query, _ := strings.Cut(connStr, "?")
	params, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(params)
	}
	connStr = filename
	if query := params.Encode(); query != "" {
		connStr += "?" + query
	}

	if err := prepareStorage(filename, params); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, err
	}

	// Keep the template's SQLite pool serialized, independently of pragmas.
	conn.SetMaxOpenConns(1)

	if err := database.Ping(ctx, conn.PingContext); err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := applyMigrations(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	s := &SQLite{
		db:      conn,
		queries: db.New(conn),
	}

	return s, nil
}

func WithForeignKeys() OptFunc {
	return func(params url.Values) {
		// The driver gives the alias precedence over _foreign_keys.
		params.Del("_fk")
		params.Set("_foreign_keys", "on")
	}
}

func (s *SQLite) Querier() database.Querier {
	return queries.New(s.queries)
}

func (s *SQLite) WithTx(ctx context.Context, fn func(q database.Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := queries.New(s.queries.WithTx(tx))
	if err := fn(qtx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLite) IsErrNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func (s *SQLite) Close() error {
	return s.db.Close()
}
