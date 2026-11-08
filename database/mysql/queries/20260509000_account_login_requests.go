package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/mysql/db"
)

func (q *MySQLQuerier) UpsertAccountLoginRequest(ctx context.Context, email string, otpHash string, expiresAt int64) error {
	return q.queries.UpsertAccountLoginRequest(ctx, db.UpsertAccountLoginRequestParams{
		Email:     email,
		OTPHash:   otpHash,
		ExpiresAt: expiresAt,
	})
}

func (q *MySQLQuerier) ConsumeAccountLoginRequest(ctx context.Context, email string, otpHash string, now int64) (*database.AccountLoginRequest, error) {
	var request *database.AccountLoginRequest
	err := q.withTx(ctx, func(q *MySQLQuerier) error {
		saved, err := q.queries.SelectAccountLoginRequestForUpdate(ctx, db.SelectAccountLoginRequestForUpdateParams{
			Email:     email,
			OTPHash:   otpHash,
			ExpiresAt: now,
		})
		if err != nil {
			return err
		}

		if err := q.queries.DeleteAccountLoginRequest(ctx, saved.Email); err != nil {
			return err
		}

		request = (*database.AccountLoginRequest)(&saved)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return request, nil
}

func (q *MySQLQuerier) DeleteAccountLoginRequest(ctx context.Context, email string) error {
	return q.queries.DeleteAccountLoginRequest(ctx, email)
}
