package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goauth "github.com/tavocg/go-auth"
	"github.com/tavocg/go-auth/jwt"
	secrets "github.com/tavocg/go-secrets"
	"sifeucr/database"
)

type Authenticator struct {
	db         database.Database
	tokener    *jwt.Tokener[*Claims]
	refreshTTL time.Duration
}

func NewAuthenticator(db database.Database, secret string, accessTTL time.Duration, refreshTTL time.Duration) (*Authenticator, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}

	if accessTTL < time.Second || refreshTTL < time.Second {
		return nil, fmt.Errorf("token TTLs must be at least one second")
	}

	tokener, err := jwt.NewHS256Tokener[*Claims](secret, accessTTL)
	if err != nil {
		return nil, err
	}

	return &Authenticator{
		db:         db,
		tokener:    tokener,
		refreshTTL: refreshTTL,
	}, nil
}

func (a *Authenticator) Issue(ctx context.Context, identity *Claims) (at *goauth.Token, rt *goauth.Token, err error) {
	sub, err := subject(identity)
	if err != nil {
		return nil, nil, err
	}

	err = a.db.WithTx(ctx, func(q database.Querier) error {
		var err error
		at, rt, err = a.issueLocked(ctx, q, sub, identity.UserAgent)
		return err
	})
	if err != nil {
		return nil, nil, err
	}

	return at, rt, nil
}

func (a *Authenticator) Refresh(ctx context.Context, refreshToken string) (at *goauth.Token, rt *goauth.Token, err error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, nil, goauth.ErrInvalidToken
	}

	tokenHash := hashRefreshToken(refreshToken)
	// Discover the subject outside the transaction. SQLite can then acquire its
	// write lock without upgrading a read snapshot that may have become stale.
	saved, err := a.db.Querier().SelectRefreshTokenByHash(ctx, tokenHash)
	if a.db.IsErrNotFound(err) {
		return nil, nil, goauth.ErrInvalidToken
	}
	if err != nil {
		return nil, nil, err
	}
	if saved.ExpiresAt != 0 && time.Now().UTC().Unix() >= saved.ExpiresAt {
		return nil, nil, goauth.ErrExpiredToken
	}

	err = a.db.WithTx(ctx, func(q database.Querier) error {
		// Match Issue/RevokeAll's lock order: account, then refresh token.
		authID, err := q.LockAccountAuthID(ctx, saved.Sub)
		if a.db.IsErrNotFound(err) {
			return goauth.ErrInvalidToken
		}
		if err != nil {
			return err
		}
		if authID == "" || saved.AuthID != authID {
			return goauth.ErrInvalidToken
		}

		consumed, err := q.ConsumeRefreshTokenByHash(ctx, tokenHash)
		if err != nil {
			if a.db.IsErrNotFound(err) {
				return goauth.ErrInvalidToken
			}
			return err
		}
		if consumed.Sub != saved.Sub || consumed.AuthID != authID || consumed.SessionID != saved.SessionID {
			return goauth.ErrInvalidToken
		}
		// The token may have expired while waiting for either lock.
		if consumed.ExpiresAt != 0 && time.Now().UTC().Unix() >= consumed.ExpiresAt {
			return goauth.ErrExpiredToken
		}
		// Retention covers the original credentials' lifetime, including access
		// tokens that outlive refresh. Legacy non-expiring refreshes get one TTL.
		expiresAt := consumed.ExpiresAt
		if expiresAt == 0 {
			expiresAt = time.Now().UTC().Add(a.refreshTTL).Unix()
		}
		if err := q.InsertConsumedRefreshToken(ctx, database.ConsumedRefreshToken{
			TokenHash: consumed.TokenHash, Sub: consumed.Sub, AuthID: consumed.AuthID,
			SessionID: consumed.SessionID, ExpiresAt: max(expiresAt, consumed.AccessExpiresAt),
		}); err != nil {
			return err
		}

		at, rt, err = a.signSession(ctx, q, consumed)
		return err
	})
	if err != nil {
		return nil, nil, err
	}

	return at, rt, nil
}

func (a *Authenticator) Verify(ctx context.Context, accessToken string) (*Claims, error) {
	claims, err := a.tokener.Verify(accessToken)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrInvalidToken):
			return nil, goauth.ErrInvalidToken
		case errors.Is(err, jwt.ErrExpiredToken):
			return nil, goauth.ErrExpiredToken
		default:
			return nil, err
		}
	}
	if claims == nil || claims.AuthID == "" || claims.SessionID == "" || claims.ExpiresAt() <= 0 {
		return nil, goauth.ErrInvalidToken
	}

	sub, err := subject(claims)
	if err != nil {
		return nil, goauth.ErrInvalidToken
	}
	authID, err := a.db.Querier().SelectSessionAuthID(ctx, sub, claims.SessionID)
	if a.db.IsErrNotFound(err) {
		return nil, goauth.ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	if authID != claims.AuthID {
		return nil, goauth.ErrInvalidToken
	}

	return claims, nil
}

func (a *Authenticator) Revoke(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return goauth.ErrInvalidToken
	}

	// Resolve the stable session before locking, as Refresh does for SQLite.
	// A concurrent rotation may replace the hash while we wait for the account.
	saved, err := a.db.Querier().SelectRevocableSessionByHash(ctx, hashRefreshToken(refreshToken), time.Now().Unix())
	if a.db.IsErrNotFound(err) {
		return nil // Unknown or no longer retained; keep revocation idempotent.
	}
	if err != nil {
		return err
	}
	return a.db.WithTx(ctx, func(q database.Querier) error {
		authID, err := q.LockAccountAuthID(ctx, saved.Sub)
		if a.db.IsErrNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if authID != saved.AuthID {
			return nil // This generation has already been revoked.
		}
		return q.DeleteAccountSession(ctx, saved.Sub, saved.SessionID)
	})
}

func (a *Authenticator) RevokeAll(ctx context.Context, identity *Claims) error {
	sub, err := subject(identity)
	if err != nil {
		return err
	}

	// Rotate instead of clearing so every access token carrying the previous
	// generation fails verification, not just future refreshes.
	authID, err := database.NewAuthID()
	if err != nil {
		return err
	}

	return a.db.WithTx(ctx, func(q database.Querier) error {
		if _, err := q.LockAccountAuthID(ctx, sub); err != nil {
			// Account deletion is idempotent: nothing remains to revoke.
			if a.db.IsErrNotFound(err) {
				return nil
			}
			return err
		}

		if err := q.SetAccountAuthID(ctx, sub, authID); err != nil {
			return err
		}

		return q.DeleteRefreshTokensBySub(ctx, sub)
	})
}

func (a *Authenticator) issueLocked(ctx context.Context, q database.Querier, sub int64, userAgent string) (*goauth.Token, *goauth.Token, error) {
	authID, err := q.LockAccountAuthID(ctx, sub)
	if a.db.IsErrNotFound(err) {
		return nil, nil, goauth.ErrInvalidIdentity
	}
	if err != nil {
		return nil, nil, err
	}
	if authID == "" {
		return nil, nil, goauth.ErrInvalidIdentity
	}

	// Consume the singleton marker in the session transaction. Failed issuance
	// rolls the claim back so the first successful login still owns setup.
	claimed, err := q.ClaimSetup(ctx)
	if err != nil {
		return nil, nil, err
	}
	if claimed == 1 {
		if err := q.AssignRoleToAccount(ctx, sub, database.RoleAdmin); err != nil {
			return nil, nil, err
		}
	}

	sessionID, err := database.NewAuthID()
	if err != nil {
		return nil, nil, err
	}
	return a.signSession(ctx, q, &database.RefreshToken{
		Sub:       sub,
		AuthID:    authID,
		SessionID: sessionID,
		UserAgent: normalizeUserAgent(userAgent),
		CreatedAt: time.Now().UTC().Unix(),
	})
}

func (a *Authenticator) signSession(ctx context.Context, q database.Querier, session *database.RefreshToken) (*goauth.Token, *goauth.Token, error) {
	roles, err := q.SelectAccountRolesBySub(ctx, session.Sub)
	if err != nil {
		return nil, nil, err
	}

	claims := &Claims{
		AuthID:    session.AuthID,
		SessionID: session.SessionID,
		Sub:       strconv.FormatInt(session.Sub, 10),
		Roles:     roles,
	}
	accessToken, err := a.tokener.Sign(claims)
	if err != nil {
		return nil, nil, err
	}

	refreshToken, err := secrets.RandStr()
	if err != nil {
		return nil, nil, err
	}

	var refreshExpiresAt int64
	if a.refreshTTL > 0 {
		refreshExpiresAt = time.Now().UTC().Add(a.refreshTTL).Unix()
	}

	session.TokenHash = hashRefreshToken(refreshToken)
	session.ExpiresAt = refreshExpiresAt
	session.AccessExpiresAt = max(session.AccessExpiresAt, claims.ExpiresAt())
	session.LastUsedAt = time.Now().UTC().Unix()
	if err := q.InsertRefreshToken(ctx, *session); err != nil {
		return nil, nil, err
	}

	return &goauth.Token{
		Value:     accessToken,
		ExpiresAt: claims.ExpiresAt(),
	}, &goauth.Token{
		Value:     refreshToken,
		ExpiresAt: refreshExpiresAt,
	}, nil
}

func subject(identity *Claims) (int64, error) {
	if identity == nil || strings.TrimSpace(identity.Subject()) == "" {
		return 0, goauth.ErrInvalidIdentity
	}

	sub, err := strconv.ParseInt(identity.Subject(), 10, 64)
	if err != nil || sub <= 0 {
		return 0, goauth.ErrInvalidIdentity
	}

	return sub, nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
