// Package mysql implements the database interface with MySQL.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"sifeucr/database"
	"sifeucr/database/mysql/queries"

	mysqldriver "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	conn *sql.DB
}

var _ database.Database = (*MySQL)(nil)

func NewMySQL(ctx context.Context, connStr string) (*MySQL, error) {
	cfg, err := parseConfig(connStr)
	if err != nil {
		return nil, err
	}
	if cfg.DBName == "" {
		return nil, fmt.Errorf("mysql database name is required")
	}
	// Embedded migrations contain multiple statements per file.
	cfg.MultiStatements = true

	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		return nil, err
	}

	conn := sql.OpenDB(connector)
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(3 * time.Minute)

	if err := database.Ping(ctx, conn.PingContext); err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := applyMigrations(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &MySQL{conn: conn}, nil
}

func parseConfig(connStr string) (*mysqldriver.Config, error) {
	if !strings.HasPrefix(connStr, "mysql://") {
		return mysqldriver.ParseDSN(connStr)
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse mysql URL: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("mysql URL requires a host")
	}

	// Parse driver options from the query string without putting credentials
	// through DSN escaping.
	cfg, err := mysqldriver.ParseDSN("/?" + u.RawQuery)
	if err != nil {
		return nil, err
	}
	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Passwd, _ = u.User.Password()
	}
	cfg.Net = "tcp"
	cfg.Addr = u.Host
	cfg.DBName = strings.TrimPrefix(u.Path, "/")

	return cfg, nil
}

func (m *MySQL) Querier() database.Querier {
	return queries.New(m.conn)
}

func (m *MySQL) WithTx(ctx context.Context, fn func(q database.Querier) error) error {
	tx, err := m.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(queries.NewTx(tx)); err != nil {
		return err
	}

	return tx.Commit()
}

func (m *MySQL) IsErrNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func (m *MySQL) Close() error {
	return m.conn.Close()
}
