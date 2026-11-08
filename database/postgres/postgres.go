// Package postgres implements the database interface with PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"sifeucr/database"
	"sifeucr/database/postgres/db"
	"sifeucr/database/postgres/queries"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

var _ database.Database = (*Postgres)(nil)

func NewPostgres(ctx context.Context, connStr string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}
	// Pool connection attempts can outlive a canceled acquisition.
	if cfg.ConnConfig.ConnectTimeout <= 0 {
		cfg.ConnConfig.ConnectTimeout = database.ConnectTimeout
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := database.Ping(ctx, pool.Ping); err != nil {
		pool.Close()
		return nil, err
	}

	if err := applyMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	return &Postgres{
		pool:    pool,
		queries: db.New(pool),
	}, nil
}

func (p *Postgres) Querier() database.Querier {
	return queries.New(p.queries)
}

func (p *Postgres) WithTx(ctx context.Context, fn func(q database.Querier) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := queries.New(p.queries.WithTx(tx))
	if err := fn(qtx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (p *Postgres) IsErrNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return nil
}
