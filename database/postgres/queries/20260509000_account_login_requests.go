package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/postgres/db"
)

func (q *PostgresQuerier) UpsertAccountLoginRequest(ctx context.Context, email string, otpHash string, expiresAt int64) error {
	return q.queries.UpsertAccountLoginRequest(ctx, db.UpsertAccountLoginRequestParams{
		Email:     email,
		OTPHash:   otpHash,
		ExpiresAt: expiresAt,
	})
}

func (q *PostgresQuerier) ConsumeAccountLoginRequest(ctx context.Context, email string, otpHash string, now int64) (*database.AccountLoginRequest, error) {
	request, err := q.queries.ConsumeAccountLoginRequest(ctx, db.ConsumeAccountLoginRequestParams{Email: email, OTPHash: otpHash, ExpiresAt: now})
	if err != nil {
		return nil, err
	}

	return (*database.AccountLoginRequest)(&request), nil
}

func (q *PostgresQuerier) DeleteAccountLoginRequest(ctx context.Context, email string) error {
	return q.queries.DeleteAccountLoginRequest(ctx, email)
}
