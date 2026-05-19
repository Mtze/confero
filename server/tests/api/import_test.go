package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"confero/internal/auth"
	"confero/internal/calendar"
	"confero/internal/database"
	chihttp "confero/internal/http"
	"confero/internal/service"
)

func newImportTestServer(t *testing.T, dsn string) (*httptest.Server, *auth.TokenManager) {
	t.Helper()
	const secret = "this-is-a-32-byte-test-secret!!"
	ctx := context.Background()
	require.NoError(t, database.RunMigrations(dsn))
	pool, err := database.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	for _, u := range []struct{ id, sub, email, name string }{
		{memberUUID, "sub-member", "member@example.org", "Test Member"},
		{adminUUID, "sub-admin", "admin@example.org", "Test Admin"},
	} {
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, oidc_issuer, oidc_subject, email, display_name)
			 VALUES ($1, 'test', $2, $3, $4)
			 ON CONFLICT (oidc_issuer, oidc_subject) DO NOTHING`,
			u.id, u.sub, u.email, u.name,
		)
		require.NoError(t, err)
		_, err = pool.Exec(ctx,
			`INSERT INTO user_settings (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
			u.id,
		)
		require.NoError(t, err)
	}

	confSvc := service.NewConferenceService(pool)
	starSvc := service.NewStarService(pool)
	settingsSvc := service.NewSettingsService(pool)
	calSvc := service.NewCalendarService(pool, "http://localhost")
	calBuilder := calendar.NewBuilder(pool)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tm := auth.NewTokenManager(secret)
	srv := chihttp.NewServer(logger, confSvc, starSvc, settingsSvc, calSvc, calBuilder)
	router := chihttp.NewRouter(srv, tm, nil)
	return httptest.NewServer(router), tm
}

func doYAML(t *testing.T, url, body string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/x-yaml")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

const validYAML = `conferences:
  - name: Test Conference
    acronym: TC
    year: 2026
    location: Berlin
    tags: [systems]
    tracks: []
`

const twoConfsYAML = `conferences:
  - name: Test Conference
    acronym: TC
    year: 2026
    location: Berlin
  - name: Second Conference
    acronym: SC
    year: 2026
    location: Munich
`

const invalidYAML = `conferences:
  - name: ""
    acronym: NONAME
    year: 2026
    location: Berlin
`

const unknownFieldYAML = `conferences:
  - name: Test Conference
    acronym: TC
    year: 2026
    location: Berlin
    unknown_field: oops
`

func decodeImportResult(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func TestImport_CreateTwo(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newImportTestServer(t, dsn)

	resp := doYAML(t, ts.URL+"/api/v1/import", twoConfsYAML, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeImportResult(t, resp)
	require.Equal(t, float64(2), result["created"])
	require.Equal(t, float64(0), result["updated"])
	require.Equal(t, float64(0), result["skipped"])
}

func TestImport_Idempotent(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newImportTestServer(t, dsn)

	// First import creates.
	resp := doYAML(t, ts.URL+"/api/v1/import", validYAML, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	r1 := decodeImportResult(t, resp)
	require.Equal(t, float64(1), r1["created"])

	// Second import of the same data updates.
	resp2 := doYAML(t, ts.URL+"/api/v1/import", validYAML, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	r2 := decodeImportResult(t, resp2)
	require.Equal(t, float64(0), r2["created"])
	require.Equal(t, float64(1), r2["updated"])
}

func TestImport_ValidationError_StrictMode(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newImportTestServer(t, dsn)

	resp := doYAML(t, ts.URL+"/api/v1/import", invalidYAML, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	result := decodeImportResult(t, resp)
	require.Equal(t, float64(0), result["created"])
	errs, _ := result["errors"].([]any)
	require.NotEmpty(t, errs)
}

func TestImport_UnknownField_Rejected(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newImportTestServer(t, dsn)

	// Unknown fields cause a YAML parse error -> 400 Bad Request.
	resp := doYAML(t, ts.URL+"/api/v1/import", unknownFieldYAML, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestImport_Unauthorized(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newImportTestServer(t, dsn)

	// No cookie.
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/import", bytes.NewBufferString(validYAML))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/x-yaml")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Members are allowed (import is member-level, per spec).
	resp2 := doYAML(t, ts.URL+"/api/v1/import", validYAML, sessionCookie(memberToken(t, tm)))
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	_ = tm
}
