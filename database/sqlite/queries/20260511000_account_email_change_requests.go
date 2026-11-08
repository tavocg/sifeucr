package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/sqlite/db"
)

func (q *SQLiteQuerier) UpsertAccountEmailChangeRequest(ctx context.Context, sub int64, email string, otpHash string, expiresAt int64) error {
	return q.queries.UpsertAccountEmailChangeRequest(ctx, db.UpsertAccountEmailChangeRequestParams{
		Sub:       sub,
		Email:     email,
		OTPHash:   otpHash,
		ExpiresAt: expiresAt,
	})
}

func (q *SQLiteQuerier) ConsumeAccountEmailChangeRequest(ctx context.Context, sub int64, otpHash string, now int64) (*database.AccountEmailChangeRequest, error) {
	request, err := q.queries.ConsumeAccountEmailChangeRequest(ctx, db.ConsumeAccountEmailChangeRequestParams{Sub: sub, OTPHash: otpHash, ExpiresAt: now})
	if err != nil {
		return nil, err
	}

	return (*database.AccountEmailChangeRequest)(&request), nil
}

func (q *SQLiteQuerier) DeleteAccountEmailChangeRequest(ctx context.Context, sub int64) error {
	return q.queries.DeleteAccountEmailChangeRequest(ctx, sub)
}
