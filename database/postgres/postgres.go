// Package postgres implements the database interface with PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"sifeucr/database"
	"sifeucr/database/postgres/db"
	"sifeucr/database/postgres/queries"
)

type Postgres struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

var _ database.Database = (*Postgres)(nil)

func NewPostgres(ctx context.Context, connStr string) (*Postgres, error) {
	return Open(ctx, connStr, database.OpenOptions{})
}

// Open connects with explicit startup options.
func Open(ctx context.Context, connStr string, options database.OpenOptions) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}
	// Pool connection attempts can outlive a canceled acquisition.
	if cfg.ConnConfig.ConnectTimeout <= 0 {
		cfg.ConnConfig.ConnectTimeout = database.ConnectTimeout
	}
	if options.MaxConnections > 0 {
		cfg.MaxConns = int32(options.MaxConnections)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := database.Ping(ctx, pool.Ping); err != nil {
		pool.Close()
		return nil, err
	}

	if err := database.PrepareSchema(ctx, migrationFiles, db.New(pool).GetSchemaMigrationVersion, func(ctx context.Context) error { return applyMigrations(ctx, pool) }, options.SkipMigrations); err != nil {
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

func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
