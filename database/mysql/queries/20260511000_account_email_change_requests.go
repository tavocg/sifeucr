package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/mysql/db"
)

func (q *MySQLQuerier) UpsertAccountEmailChangeRequest(ctx context.Context, sub int64, email string, otpHash string, expiresAt int64) error {
	return q.queries.UpsertAccountEmailChangeRequest(ctx, db.UpsertAccountEmailChangeRequestParams{
		Sub:       sub,
		Email:     email,
		OTPHash:   otpHash,
		ExpiresAt: expiresAt,
	})
}

func (q *MySQLQuerier) ConsumeAccountEmailChangeRequest(ctx context.Context, sub int64, otpHash string, now int64) (*database.AccountEmailChangeRequest, error) {
	var request *database.AccountEmailChangeRequest
	err := q.withTx(ctx, func(q *MySQLQuerier) error {
		saved, err := q.queries.SelectAccountEmailChangeRequestForUpdate(ctx, db.SelectAccountEmailChangeRequestForUpdateParams{
			Sub:       sub,
			OTPHash:   otpHash,
			ExpiresAt: now,
		})
		if err != nil {
			return err
		}

		if err := q.queries.DeleteAccountEmailChangeRequest(ctx, saved.Sub); err != nil {
			return err
		}

		request = (*database.AccountEmailChangeRequest)(&saved)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return request, nil
}

func (q *MySQLQuerier) DeleteAccountEmailChangeRequest(ctx context.Context, sub int64) error {
	return q.queries.DeleteAccountEmailChangeRequest(ctx, sub)
}
