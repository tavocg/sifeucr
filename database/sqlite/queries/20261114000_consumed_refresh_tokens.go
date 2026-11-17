package queries

import (
	"context"
	"sifeucr/database"
	"sifeucr/database/sqlite/db"
)

func (q *SQLiteQuerier) InsertConsumedRefreshToken(ctx context.Context, token database.ConsumedRefreshToken) error {
	return q.queries.InsertConsumedRefreshToken(ctx, db.InsertConsumedRefreshTokenParams{
		TokenHash: token.TokenHash, Sub: token.Sub, AuthID: token.AuthID, SessionID: token.SessionID, ExpiresAt: token.ExpiresAt,
	})
}

func (q *SQLiteQuerier) SelectRevocableSessionByHash(ctx context.Context, tokenHash string, now int64) (*database.SessionIdentity, error) {
	row, err := q.queries.SelectRevocableSessionByHash(ctx, db.SelectRevocableSessionByHashParams{TokenHash: tokenHash, Now: now})
	if err != nil {
		return nil, err
	}
	return &database.SessionIdentity{Sub: row.Sub, AuthID: row.AuthID, SessionID: row.SessionID}, nil
}
