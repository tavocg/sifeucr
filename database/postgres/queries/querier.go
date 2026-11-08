// Package queries adapts generated PostgreSQL queries to the database interface.
package queries

import (
	"sifeucr/database"
	"sifeucr/database/postgres/db"
)

type PostgresQuerier struct {
	queries *db.Queries
}

var _ database.Querier = (*PostgresQuerier)(nil)

func New(queries *db.Queries) *PostgresQuerier {
	return &PostgresQuerier{
		queries: queries,
	}
}
