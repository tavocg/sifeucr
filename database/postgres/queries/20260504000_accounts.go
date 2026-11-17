package queries

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"sifeucr/database"
	"sifeucr/database/postgres/db"
)

func (q *PostgresQuerier) EnsureAccountByEmail(ctx context.Context, email string, createdAt int64) (sub int64, err error) {
	authID, err := database.NewAuthID()
	if err != nil {
		return 0, err
	}
	return q.queries.EnsureAccountByEmail(ctx, db.EnsureAccountByEmailParams{
		Email:     email,
		CreatedAt: createdAt,
		AuthID:    authID,
	})
}

func (q *PostgresQuerier) UpdateAccountEmail(ctx context.Context, sub int64, email string) error {
	err := q.queries.UpdateAccountEmail(ctx, db.UpdateAccountEmailParams{
		Sub:   sub,
		Email: email,
	})
	var conflict *pgconn.PgError
	if errors.As(err, &conflict) && conflict.Code == "23505" && conflict.ConstraintName == "accounts_email_key" {
		return database.ErrEmailUnavailable
	}
	return err
}

func (q *PostgresQuerier) DeleteAccount(ctx context.Context, sub int64) error {
	return q.queries.DeleteAccount(ctx, sub)
}

func (q *PostgresQuerier) SelectAccountBySub(ctx context.Context, sub int64) (*database.Account, error) {
	account, err := q.queries.SelectAccountBySub(ctx, sub)
	if err != nil {
		return nil, err
	}

	return (*database.Account)(&account), nil
}

func (q *PostgresQuerier) SelectAccountAuthID(ctx context.Context, sub int64) (string, error) {
	return q.queries.SelectAccountAuthID(ctx, sub)
}

func (q *PostgresQuerier) LockAccountAuthID(ctx context.Context, sub int64) (string, error) {
	return q.queries.LockAccountAuthID(ctx, sub)
}

func (q *PostgresQuerier) SetAccountAuthID(ctx context.Context, sub int64, authID string) error {
	return q.queries.SetAccountAuthID(ctx, db.SetAccountAuthIDParams{
		Sub:    sub,
		AuthID: authID,
	})
}
