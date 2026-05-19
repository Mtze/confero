package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"confero/internal/auth"
)

const testSecret = "this-is-a-32-byte-test-secret!!"

func TestTokenRoundTrip(t *testing.T) {
	tm := auth.NewTokenManager(testSecret)

	claims := auth.SessionClaims{
		Email:   "user@example.org",
		Name:    "Test User",
		OIDCSub: "oidc-sub-123",
		Roles:   []string{"member"},
	}
	claims.Subject = "550e8400-e29b-41d4-a716-446655440000"

	token, err := tm.Issue(claims)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := tm.Verify(token)
	require.NoError(t, err)
	require.Equal(t, claims.Email, got.Email)
	require.Equal(t, claims.Name, got.Name)
	require.Equal(t, claims.OIDCSub, got.OIDCSub)
	require.Equal(t, claims.Roles, got.Roles)
	require.Equal(t, claims.Subject, got.Subject)
}

func TestTokenExpired(t *testing.T) {
	tm := auth.NewTokenManager(testSecret)

	// Issue a token with an already-expired time by crafting one manually
	expired := auth.SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "confero",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			Subject:   "550e8400-e29b-41d4-a716-446655440000",
		},
		Email: "old@example.org",
		Roles: []string{"member"},
	}
	rawToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expired)
	signed, err := rawToken.SignedString([]byte(testSecret))
	require.NoError(t, err)

	_, err = tm.Verify(signed)
	require.Error(t, err, "expired token should be rejected")
}

func TestTokenTampered(t *testing.T) {
	tm := auth.NewTokenManager(testSecret)

	claims := auth.SessionClaims{
		Email: "user@example.org",
		Roles: []string{"member"},
	}
	claims.Subject = "550e8400-e29b-41d4-a716-446655440000"

	token, err := tm.Issue(claims)
	require.NoError(t, err)

	// Flip a byte in the signature (last part after the second dot)
	tampered := token[:len(token)-2] + "XX"
	_, err = tm.Verify(tampered)
	require.Error(t, err, "tampered token should be rejected")
}

func TestTokenWrongSecret(t *testing.T) {
	issuer := auth.NewTokenManager(testSecret)
	verifier := auth.NewTokenManager("different-secret-that-is-32bytes")

	claims := auth.SessionClaims{Email: "user@example.org"}
	claims.Subject = "550e8400-e29b-41d4-a716-446655440000"

	token, err := issuer.Issue(claims)
	require.NoError(t, err)

	_, err = verifier.Verify(token)
	require.Error(t, err, "token signed with different secret should be rejected")
}
