package database

import secrets "github.com/tavocg/go-secrets"

// NewAuthID returns a 256-bit random session generation as 64 lowercase hex characters.
// Existing identities remain valid until the account's sessions are revoked.
func NewAuthID() (string, error) {
	return secrets.RandStr(256)
}
