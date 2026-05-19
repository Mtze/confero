package api_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"confero/internal/auth"
	"confero/internal/calendar"
	"confero/internal/database"
	chihttp "confero/internal/http"
	"confero/internal/repository"
	"confero/internal/service"
)

func newAuditTestServer(t *testing.T, dsn string) (*httptest.Server, *auth.TokenManager) {
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
	queries := repository.New(pool)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tm := auth.NewTokenManager(secret)
	srv := chihttp.NewServer(logger, confSvc, starSvc, settingsSvc, calSvc, calBuilder).
		WithAuditQueries(queries)
	router := chihttp.NewRouter(srv, tm, nil)
	return httptest.NewServer(router), tm
}

func TestAuditLog_CreateConferenceWritesEntry(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newAuditTestServer(t, dsn)

	body := minimalConferenceBody()
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", body, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var conf map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&conf))
	confID := conf["id"].(string)

	// Audit log should have one entry for the create action.
	resp2 := doJSON(t, http.MethodGet, ts.URL+"/api/v1/audit-log", nil, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&entries))
	require.Len(t, entries, 1)
	require.Equal(t, "create", entries[0]["action"])
	require.Equal(t, "conference", entries[0]["entity_type"])
	require.Equal(t, confID, entries[0]["entity_id"])
	require.Equal(t, "Test Admin", entries[0]["actor_display_name"])
}

func TestAuditLog_4xxDoesNotWriteEntry(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newAuditTestServer(t, dsn)

	// Create a conference to establish a known ID.
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", minimalConferenceBody(), sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// Clear audit log by creating a fresh server sharing the same DSN is not
	// practical here; instead count before and after.
	resp2 := doJSON(t, http.MethodGet, ts.URL+"/api/v1/audit-log", nil, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var before []map[string]any
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&before))
	countBefore := len(before)

	// Attempt to create a duplicate - returns 409, no audit row.
	resp3 := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", minimalConferenceBody(), sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusConflict, resp3.StatusCode)

	resp4 := doJSON(t, http.MethodGet, ts.URL+"/api/v1/audit-log", nil, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var after []map[string]any
	require.NoError(t, json.NewDecoder(resp4.Body).Decode(&after))
	require.Equal(t, countBefore, len(after), "no new audit entry for 409 response")
}

func TestAuditLog_MemberForbidden(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newAuditTestServer(t, dsn)

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/audit-log", nil, sessionCookie(memberToken(t, tm)))
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAuditLog_Pagination(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newAuditTestServer(t, dsn)

	// Create two conferences.
	for _, body := range []map[string]any{
		minimalConferenceBody(),
		{"name": "Second Conf", "acronym": "SC", "year": 2027, "location": "Munich"},
	} {
		resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", body, sessionCookie(adminToken(t, tm)))
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// Fetch with limit=1.
	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/audit-log?limit=1", nil, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var entries []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&entries))
	require.Len(t, entries, 1)

	// Fetch second page using before= timestamp.
	ts1, err := time.Parse(time.RFC3339, entries[0]["created_at"].(string))
	require.NoError(t, err)
	before := ts1.UTC().Format(time.RFC3339Nano)

	resp2 := doJSON(t, http.MethodGet, ts.URL+"/api/v1/audit-log?limit=1&before="+before, nil, sessionCookie(adminToken(t, tm)))
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var entries2 []map[string]any
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&entries2))
	require.Len(t, entries2, 1)
	require.NotEqual(t, entries[0]["id"], entries2[0]["id"])
}

func minimalConferenceBody() map[string]any {
	return map[string]any{
		"name":     "Test Conference",
		"acronym":  "TC",
		"year":     2026,
		"location": "Berlin",
	}
}
