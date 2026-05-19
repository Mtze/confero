package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OIDCClaimName is the OIDC token claim that carries group/role membership.
// Set via -ldflags "-X confero/internal/auth.OIDCClaimName=<name>".
// Default: "groups".
var OIDCClaimName = "groups"

const (
	tokenIssuer = "confero"
	tokenTTL    = time.Hour
	cookieName  = "session"
)

// SessionClaims is the set of claims stored in the session JWT.
type SessionClaims struct {
	jwt.RegisteredClaims
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	OIDCSub string   `json:"oidc_sub"`
	Roles   []string `json:"roles"`
}

// TokenManager signs and verifies HS256 JWTs.
type TokenManager struct {
	secret []byte
}

// NewTokenManager creates a TokenManager from the given secret.
func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret)}
}

// Issue signs a new JWT for the given claims.
func (m *TokenManager) Issue(claims SessionClaims) (string, error) {
	now := time.Now()
	sub := claims.Subject // preserve caller-set subject
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    tokenIssuer,
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates a signed JWT string, returning its claims.
func (m *TokenManager) Verify(tokenStr string) (SessionClaims, error) {
	var claims SessionClaims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(tokenIssuer), jwt.WithExpirationRequired())
	if err != nil {
		return SessionClaims{}, fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return SessionClaims{}, errors.New("token is not valid")
	}
	return claims, nil
}
