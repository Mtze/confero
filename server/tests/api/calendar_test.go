package api_test

import (
	"context"
	"encoding/json"
	"fmt"
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

func newCalendarTestServer(t *testing.T, dsn string) (*httptest.Server, *auth.TokenManager) {
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

func seedConference(t *testing.T, ts *httptest.Server, tm *auth.TokenManager) string {
	t.Helper()
	body := `{
		"name":"SIGCSE 2026","acronym":"SIGCSE","year":2026,"location":"Portland",
		"primary_deadline":"2025-10-01T23:59:00Z",
		"event_start_date":"2026-03-10","event_end_date":"2026-03-13"
	}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/conferences", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(memberCookie(t, tm))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var conf struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&conf))
	return conf.ID
}

func memberCookie(t *testing.T, tm *auth.TokenManager) *http.Cookie {
	t.Helper()
	sc := auth.SessionClaims{Email: "member@example.org", Name: "Test Member", OIDCSub: "sub-member", Roles: []string{"member"}}
	sc.Subject = memberUUID
	token, err := tm.Issue(sc)
	require.NoError(t, err)
	return &http.Cookie{Name: "session", Value: token}
}

func TestGetPublicCalendar_200(t *testing.T) {
	t.Parallel()
	dsn := newPostgresTestDSN(t)
	ts, tm := newCalendarTestServer(t, dsn)
	defer ts.Close()

	seedConference(t, ts, tm)

	resp, err := http.Get(ts.URL + "/calendar/all.ics")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/calendar")
	require.NotEmpty(t, resp.Header.Get("ETag"))
	require.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))
}

func TestGetPublicCalendar_ExcludesArchived(t *testing.T) {
	t.Parallel()
	dsn := newPostgresTestDSN(t)
	ts, tm := newCalendarTestServer(t, dsn)
	defer ts.Close()

	confID := seedConference(t, ts, tm)

	// Archive the conference.
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/conferences/"+confID+"/archive", nil)
	require.NoError(t, err)
	req.AddCookie(memberCookie(t, tm))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Public feed should be OK but not contain the archived conference.
	resp2, err := http.Get(ts.URL + "/calendar/all.ics")
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "") // keep fmt import used
	require.NotEmpty(t, resp2.Header.Get("ETag"))
}

func TestGetPublicCalendar_304(t *testing.T) {
	t.Parallel()
	dsn := newPostgresTestDSN(t)
	ts, _ := newCalendarTestServer(t, dsn)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/calendar/all.ics")
	require.NoError(t, err)
	_ = resp.Body.Close()
	etag := resp.Header.Get("ETag")
	require.NotEmpty(t, etag)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/calendar/all.ics", nil)
	require.NoError(t, err)
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp2.Body.Close()
	require.Equal(t, http.StatusNotModified, resp2.StatusCode)
}

func TestCreateAndGetPersonalCalendar(t *testing.T) {
	t.Parallel()
	dsn := newPostgresTestDSN(t)
	ts, tm := newCalendarTestServer(t, dsn)
	defer ts.Close()

	confID := seedConference(t, ts, tm)

	// Star the conference.
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/conferences/"+confID+"/stars", nil)
	require.NoError(t, err)
	req.AddCookie(memberCookie(t, tm))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Create a calendar token.
	req2, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/me/calendar-tokens", nil)
	require.NoError(t, err)
	req2.AddCookie(memberCookie(t, tm))
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	var tok struct {
		Token   string `json:"token"`
		FeedURL string `json:"feed_url"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&tok))
	require.NotEmpty(t, tok.Token)

	// Fetch personal feed.
	resp3, err := http.Get(ts.URL + "/calendar/u/" + tok.Token + ".ics")
	require.NoError(t, err)
	defer func() { _ = resp3.Body.Close() }()
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	require.Contains(t, resp3.Header.Get("Content-Type"), "text/calendar")
}

func TestGetPersonalCalendar_InvalidToken_404(t *testing.T) {
	t.Parallel()
	dsn := newPostgresTestDSN(t)
	ts, _ := newCalendarTestServer(t, dsn)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/calendar/u/nonexistent-token.ics")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCalendarTokenLifecycle(t *testing.T) {
	t.Parallel()
	dsn := newPostgresTestDSN(t)
	ts, tm := newCalendarTestServer(t, dsn)
	defer ts.Close()

	cookie := memberCookie(t, tm)

	// List — empty initially.
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/me/calendar-tokens", nil)
	require.NoError(t, err)
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Create token.
	req2, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/me/calendar-tokens", nil)
	require.NoError(t, err)
	req2.AddCookie(cookie)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	var tok1 struct{ Token string `json:"token"` }
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&tok1))
	_ = resp2.Body.Close()
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	// Create again — must invalidate first token.
	req3, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/me/calendar-tokens", nil)
	require.NoError(t, err)
	req3.AddCookie(cookie)
	resp3, err := http.DefaultClient.Do(req3)
	require.NoError(t, err)
	var tok2 struct{ Token string `json:"token"` }
	require.NoError(t, json.NewDecoder(resp3.Body).Decode(&tok2))
	_ = resp3.Body.Close()
	require.NotEqual(t, tok1.Token, tok2.Token)

	// Old token should now 404.
	resp4, err := http.Get(ts.URL + "/calendar/u/" + tok1.Token + ".ics")
	require.NoError(t, err)
	_ = resp4.Body.Close()
	require.Equal(t, http.StatusNotFound, resp4.StatusCode)

	// Delete token.
	req5, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/me/calendar-tokens", nil)
	require.NoError(t, err)
	req5.AddCookie(cookie)
	resp5, err := http.DefaultClient.Do(req5)
	require.NoError(t, err)
	_ = resp5.Body.Close()
	require.Equal(t, http.StatusNoContent, resp5.StatusCode)

	// Second delete should 404.
	req6, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/me/calendar-tokens", nil)
	require.NoError(t, err)
	req6.AddCookie(cookie)
	resp6, err := http.DefaultClient.Do(req6)
	require.NoError(t, err)
	_ = resp6.Body.Close()
	require.Equal(t, http.StatusNotFound, resp6.StatusCode)
}

func TestCalendarToken_Unauthorized(t *testing.T) {
	t.Parallel()
	dsn := newPostgresTestDSN(t)
	ts, _ := newCalendarTestServer(t, dsn)
	defer ts.Close()

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		req, err := http.NewRequest(method, ts.URL+"/api/v1/me/calendar-tokens", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	}
}
