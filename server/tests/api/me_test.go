package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	apimod "confero/internal/api"
	"confero/internal/auth"
	chihttp "confero/internal/http"
	"log/slog"
	"os"
)

const meTestSecret = "this-is-a-32-byte-test-secret!!"

func newAuthTestServer(t *testing.T) (*httptest.Server, *auth.TokenManager) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tm := auth.NewTokenManager(meTestSecret)
	srv := chihttp.NewServer(logger, nil, nil, nil, nil, nil)
	router := chihttp.NewRouter(srv, tm, nil)
	return httptest.NewServer(router), tm
}

func issueTestToken(t *testing.T, tm *auth.TokenManager, roles []string, expiredBy time.Duration) string {
	t.Helper()
	claims := auth.SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "confero",
			Subject:   "550e8400-e29b-41d4-a716-446655440000",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-expiredBy - time.Second)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour - expiredBy)),
		},
		Email:   "member@example.org",
		Name:    "Test Member",
		OIDCSub: "oidc-sub-123",
		Roles:   roles,
	}
	if expiredBy > 0 {
		// Build an already-expired token
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second))
		raw := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := raw.SignedString([]byte(meTestSecret))
		require.NoError(t, err)
		return signed
	}
	token, err := tm.Issue(claims)
	require.NoError(t, err)
	return token
}

func TestGetMe_401_NoCookie(t *testing.T) {
	ts, _ := newAuthTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/me")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGetMe_200_ValidToken(t *testing.T) {
	ts, tm := newAuthTestServer(t)
	defer ts.Close()

	token := issueTestToken(t, tm, []string{"member"}, 0)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/me", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body apimod.CurrentUser
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "member@example.org", string(body.Email))
	require.Equal(t, "Test Member", body.Name)
	require.Contains(t, body.Roles, apimod.Member)
}

func TestGetMe_401_ExpiredToken(t *testing.T) {
	ts, tm := newAuthTestServer(t)
	defer ts.Close()

	expiredToken := issueTestToken(t, tm, []string{"member"}, 2*time.Hour)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/me", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session", Value: expiredToken})

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
