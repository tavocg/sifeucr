// Package queries adapts generated SQLite queries to the database interface.
package queries

import (
	"sifeucr/database"
	"sifeucr/database/sqlite/db"
)

type SQLiteQuerier struct {
	queries *db.Queries
}

var _ database.Querier = (*SQLiteQuerier)(nil)

func New(queries *db.Queries) *SQLiteQuerier {
	return &SQLiteQuerier{
		queries: queries,
	}
}
