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

func Open(ctx context.Context, connStr string) (database.Database, error) {
	if connStr == "" {
		return nil, fmt.Errorf("empty connection string")
	}

	switch {
	case isPostgresConnStr(connStr):
		return postgres.NewPostgres(ctx, connStr)
	case isMySQLConnStr(connStr):
		return mysql.NewMySQL(ctx, connStr)
	default:
		return sqlite.NewSQLite(ctx, connStr, sqlite.WithForeignKeys())
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
