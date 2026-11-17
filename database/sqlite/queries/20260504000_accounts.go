package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/sqlite/db"
)

func (q *SQLiteQuerier) EnsureAccountByEmail(ctx context.Context, email string, createdAt int64) (sub int64, err error) {
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

func (q *SQLiteQuerier) DeleteAccount(ctx context.Context, sub int64) error {
	return q.queries.DeleteAccount(ctx, sub)
}

func (q *SQLiteQuerier) SelectAccountBySub(ctx context.Context, sub int64) (*database.Account, error) {
	account, err := q.queries.SelectAccountBySub(ctx, sub)
	if err != nil {
		return nil, err
	}

	return (*database.Account)(&account), nil
}

func (q *SQLiteQuerier) SelectAccountAuthID(ctx context.Context, sub int64) (string, error) {
	return q.queries.SelectAccountAuthID(ctx, sub)
}

func (q *SQLiteQuerier) LockAccountAuthID(ctx context.Context, sub int64) (string, error) {
	return q.queries.LockAccountAuthID(ctx, sub)
}

func (q *SQLiteQuerier) SetAccountAuthID(ctx context.Context, sub int64, authID string) error {
	return q.queries.SetAccountAuthID(ctx, db.SetAccountAuthIDParams{
		AuthID: authID,
		Sub:    sub,
	})
}
