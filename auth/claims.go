package auth

type Claims struct {
	AuthID    string   `json:"aid"`
	SessionID string   `json:"sid"`
	Sub       string   `json:"sub"`
	Roles     []string `json:"roles,omitempty"`
	Exp       int64    `json:"exp,omitempty"`

	// UserAgent is issuance metadata only; it is never included in the JWT.
	UserAgent string `json:"-"`
}

func (c *Claims) ExpiresAt() int64 {
	if c == nil {
		return 0
	}

	return c.Exp
}

func (c *Claims) SetExpiresAt(exp int64) {
	if c == nil {
		return
	}

	c.Exp = exp
}

func (c *Claims) Subject() string {
	if c == nil {
		return ""
	}

	return c.Sub
}
