package database

const (
	RoleDefault     = "role.default"
	RoleAdmin       = "role.admin"
	PermissionAdmin = "perm.admin"
)

type Account struct {
	Sub       int64
	Email     string
	CreatedAt int64
}

type RefreshToken struct {
	TokenHash       string
	Sub             int64
	AuthID          string
	ExpiresAt       int64
	CreatedAt       int64
	SessionID       string
	UserAgent       string
	LastUsedAt      int64
	AccessExpiresAt int64
}

// SessionIdentity identifies an account-owned session without credentials.
type SessionIdentity struct {
	Sub       int64
	AuthID    string
	SessionID string
}

// ConsumedRefreshToken permits revocation after rotation, never renewal.
type ConsumedRefreshToken struct {
	TokenHash string
	Sub       int64
	AuthID    string
	SessionID string
	ExpiresAt int64
}

// Session contains display metadata; credentials and their hashes stay private.
type Session struct {
	SessionID       string
	UserAgent       string
	CreatedAt       int64
	LastUsedAt      int64
	ExpiresAt       int64
	AccessExpiresAt int64
}
