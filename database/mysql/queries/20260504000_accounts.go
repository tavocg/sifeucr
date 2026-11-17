package queries

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"sifeucr/database"
	"sifeucr/database/mysql/db"
)

func (q *MySQLQuerier) EnsureAccountByEmail(ctx context.Context, email string, createdAt int64) (sub int64, err error) {
	authID, err := database.NewAuthID()
	if err != nil {
		return 0, err
	}
	err = q.withTx(ctx, func(q *MySQLQuerier) error {
		if err := q.queries.EnsureAccountByEmail(ctx, db.EnsureAccountByEmailParams{
			Email:     email,
			CreatedAt: createdAt,
			AuthID:    authID,
		}); err != nil {
			return err
		}

		var err error
		sub, err = q.queries.SelectAccountSubByEmail(ctx, email)
		return err
	})
	if err != nil {
		return 0, err
	}

	return sub, nil
}

func (q *MySQLQuerier) UpdateAccountEmail(ctx context.Context, sub int64, email string) error {
	err := q.queries.UpdateAccountEmail(ctx, db.UpdateAccountEmailParams{
		Email: email,
		Sub:   sub,
	})
	// This statement only changes email; duplicate keys mean it is unavailable.
	var conflict *mysql.MySQLError
	if errors.As(err, &conflict) && conflict.Number == 1062 {
		return database.ErrEmailUnavailable
	}
	return err
}

func (q *MySQLQuerier) DeleteAccount(ctx context.Context, sub int64) error {
	return q.queries.DeleteAccount(ctx, sub)
}

func (q *MySQLQuerier) SelectAccountBySub(ctx context.Context, sub int64) (*database.Account, error) {
	account, err := q.queries.SelectAccountBySub(ctx, sub)
	if err != nil {
		return nil, err
	}

	return (*database.Account)(&account), nil
}

func (q *MySQLQuerier) SelectAccountAuthID(ctx context.Context, sub int64) (string, error) {
	return q.queries.SelectAccountAuthID(ctx, sub)
}

// LockAccountAuthID must run inside WithTx: MySQL releases row locks at the
// end of the surrounding transaction.
func (q *MySQLQuerier) LockAccountAuthID(ctx context.Context, sub int64) (string, error) {
	return q.queries.LockAccountAuthID(ctx, sub)
}

func (q *MySQLQuerier) SetAccountAuthID(ctx context.Context, sub int64, authID string) error {
	return q.queries.SetAccountAuthID(ctx, db.SetAccountAuthIDParams{
		Sub:    sub,
		AuthID: authID,
	})
}
