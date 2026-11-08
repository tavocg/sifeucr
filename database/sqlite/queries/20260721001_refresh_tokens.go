package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/sqlite/db"
)

func (q *SQLiteQuerier) InsertRefreshToken(ctx context.Context, token database.RefreshToken) error {
	return q.queries.InsertRefreshToken(ctx, db.InsertRefreshTokenParams{
		TokenHash:       token.TokenHash,
		Sub:             token.Sub,
		AuthID:          token.AuthID,
		ExpiresAt:       token.ExpiresAt,
		CreatedAt:       token.CreatedAt,
		SessionID:       token.SessionID,
		UserAgent:       token.UserAgent,
		LastUsedAt:      token.LastUsedAt,
		AccessExpiresAt: token.AccessExpiresAt,
	})
}

func (q *SQLiteQuerier) SelectRefreshTokenByHash(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
	token, err := q.queries.SelectRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	return (*database.RefreshToken)(&token), nil
}

func (q *SQLiteQuerier) ConsumeRefreshTokenByHash(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
	token, err := q.queries.ConsumeRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	return (*database.RefreshToken)(&token), nil
}

func (q *SQLiteQuerier) DeleteRefreshTokenByHash(ctx context.Context, tokenHash string) error {
	return q.queries.DeleteRefreshTokenByHash(ctx, tokenHash)
}

func (q *SQLiteQuerier) DeleteRefreshTokensBySub(ctx context.Context, sub int64) error {
	return q.queries.DeleteRefreshTokensBySub(ctx, sub)
}
