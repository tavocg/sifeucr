// Package queries adapts generated MySQL queries to the database interface.
package queries

import (
	"context"
	"database/sql"

	"sifeucr/database"
	"sifeucr/database/mysql/db"
)

type MySQLQuerier struct {
	conn    *sql.DB
	queries *db.Queries
	tx      *sql.Tx
}

var _ database.Querier = (*MySQLQuerier)(nil)

func New(conn *sql.DB) *MySQLQuerier {
	return &MySQLQuerier{
		conn:    conn,
		queries: db.New(conn),
	}
}

func NewTx(tx *sql.Tx) *MySQLQuerier {
	return &MySQLQuerier{
		queries: db.New(tx),
		tx:      tx,
	}
}

// withTx runs fn in a transaction unless the querier already belongs to one.
func (q *MySQLQuerier) withTx(ctx context.Context, fn func(q *MySQLQuerier) error) error {
	if q.tx != nil {
		return fn(q)
	}

	tx, err := q.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(NewTx(tx)); err != nil {
		return err
	}

	return tx.Commit()
}
