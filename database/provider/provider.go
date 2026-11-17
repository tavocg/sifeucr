// Package provider opens database backends from connection strings.
package provider

import (
	"context"
	"fmt"
	"strings"

	"sifeucr/database"
	"sifeucr/database/mysql"
	"sifeucr/database/postgres"
	"sifeucr/database/sqlite"
)

func Open(ctx context.Context, connStr string, settings ...database.OpenOptions) (database.Database, error) {
	if connStr == "" {
		return nil, fmt.Errorf("empty connection string")
	}

	var options database.OpenOptions
	if len(settings) > 0 {
		options = settings[0]
	}
	if options.MaxConnections < 0 || int64(options.MaxConnections) > 2147483647 {
		return nil, fmt.Errorf("invalid maximum database connections")
	}
	switch {
	case isPostgresConnStr(connStr):
		return postgres.Open(ctx, connStr, options)
	case isMySQLConnStr(connStr):
		return mysql.Open(ctx, connStr, options)
	default:
		return sqlite.Open(ctx, connStr, options, sqlite.WithForeignKeys())
	}
}

func isPostgresConnStr(connStr string) bool {
	return strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://")
}

func isMySQLConnStr(connStr string) bool {
	return strings.HasPrefix(connStr, "mysql://") ||
		strings.Contains(connStr, "@tcp(") ||
		strings.Contains(connStr, "@unix(")
}
