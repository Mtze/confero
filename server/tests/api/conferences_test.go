package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	apimod "confero/internal/api"
	"confero/internal/auth"
	"confero/internal/database"
	chihttp "confero/internal/http"
	"confero/internal/service"
)

func newPostgresTestDSN(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx,
		"postgres:15-alpine",
		tcpostgres.WithDatabase("confero_test"),
		tcpostgres.WithUsername("confero"),
		tcpostgres.WithPassword("confero"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

func newConferenceTestServer(t *testing.T, dsn string) (*httptest.Server, *auth.TokenManager) {
	t.Helper()
	const secret = "this-is-a-32-byte-test-secret!!"
	ctx := context.Background()
	require.NoError(t, database.RunMigrations(dsn))
	pool, err := database.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Seed test users with known UUIDs so the FK constraint on conferences.created_by is satisfied.
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
	}

	confSvc := service.NewConferenceService(pool)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tm := auth.NewTokenManager(secret)
	srv := chihttp.NewServer(logger, confSvc)
	router := chihttp.NewRouter(srv, tm, nil)
	return httptest.NewServer(router), tm
}

const (
	memberUUID = "550e8400-e29b-41d4-a716-446655440001"
	adminUUID  = "550e8400-e29b-41d4-a716-446655440002"
)

func memberToken(t *testing.T, tm *auth.TokenManager) string {
	t.Helper()
	tok, err := tm.Issue(auth.SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: memberUUID},
		Email:            "member@example.org",
		Name:             "Test Member",
		OIDCSub:          "sub-member",
		Roles:            []string{"member"},
	})
	require.NoError(t, err)
	return tok
}

func adminToken(t *testing.T, tm *auth.TokenManager) string {
	t.Helper()
	tok, err := tm.Issue(auth.SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: adminUUID},
		Email:            "admin@example.org",
		Name:             "Test Admin",
		OIDCSub:          "sub-admin",
		Roles:            []string{"member", "admin"},
	})
	require.NoError(t, err)
	return tok
}

func doJSON(t *testing.T, method, url string, body any, cookie *http.Cookie) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, url, &buf)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func sessionCookie(token string) *http.Cookie {
	return &http.Cookie{Name: "session", Value: token}
}

func createConferenceInput() apimod.ConferenceInput {
	return apimod.ConferenceInput{
		Name:     "SIGCSE Technical Symposium",
		Acronym:  "SIGCSE",
		Year:     2027,
		Location: "Portland, OR, USA",
	}
}

// ---- List conferences ----

func TestListConferences_Public(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, _ := newConferenceTestServer(t, dsn)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/conferences", nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body []apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Empty(t, body)
}

func TestListConferences_WithArchivedFilter(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)

	// Create then archive a conference
	inp := createConferenceInput()
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", inp, sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	resp = doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/conferences/%s/archive", ts.URL, created.Id), nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Default list excludes archived
	resp = doJSON(t, http.MethodGet, ts.URL+"/api/v1/conferences", nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var listed []apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listed))
	require.Empty(t, listed, "archived should not appear in default list")

	// With archived=true, it appears
	resp = doJSON(t, http.MethodGet, ts.URL+"/api/v1/conferences?archived=true", nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listed))
	require.Len(t, listed, 1)
}

// ---- Create conference ----

func TestCreateConference_Anonymous_401(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, _ := newConferenceTestServer(t, dsn)
	defer ts.Close()

	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateConference_Member_201(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "SIGCSE", body.Acronym)
	require.Equal(t, 2027, body.Year)
	require.NotNil(t, body.CreatedBy)
	require.Empty(t, body.Tags)
	require.Empty(t, body.Tracks)
}

func TestCreateConference_WithTagsAndTracks(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	slugs := []string{"cs-education", "programming-tools"}
	codes := []string{"full_paper", "short_paper"}
	inp := createConferenceInput()
	inp.TagSlugs = &slugs
	inp.TrackCodes = &codes

	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", inp, sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Tags, 2)
	require.Len(t, body.Tracks, 2)
}

func TestCreateConference_DuplicateAcronymYear_409(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	inp := createConferenceInput()
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", inp, sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", inp, sessionCookie(tok))
	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCreateConference_ValidationError_400(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	inp := createConferenceInput()
	inp.Year = 1999 // invalid year
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", inp, sessionCookie(tok))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---- Get conference ----

func TestGetConference_Public(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// Get without auth (public)
	resp = doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/conferences/%s", ts.URL, created.Id), nil, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, created.Id, got.Id)
}

func TestGetConference_NotFound_404(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, _ := newConferenceTestServer(t, dsn)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/conferences/00000000-0000-0000-0000-000000000001", nil, nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ---- Update conference ----

func TestUpdateConference_Member_200(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	updated := createConferenceInput()
	updated.Location = "New York, NY"
	resp = doJSON(t, http.MethodPut, fmt.Sprintf("%s/api/v1/conferences/%s", ts.URL, created.Id), updated, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "New York, NY", body.Location)
}

func TestUpdateConference_Anonymous_401(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	resp = doJSON(t, http.MethodPut, fmt.Sprintf("%s/api/v1/conferences/%s", ts.URL, created.Id), createConferenceInput(), nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUpdateConference_NotFound_404(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPut, ts.URL+"/api/v1/conferences/00000000-0000-0000-0000-000000000001", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ---- Delete conference ----

func TestDeleteConference_Member_403(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	resp = doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/conferences/%s", ts.URL, created.Id), nil, sessionCookie(tok))
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestDeleteConference_Admin_204_WithCascade(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	memberTok := memberToken(t, tm)
	adminTok := adminToken(t, tm)

	// Create
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(memberTok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// Delete as admin
	resp = doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/conferences/%s", ts.URL, created.Id), nil, sessionCookie(adminTok))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify gone
	resp = doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/conferences/%s", ts.URL, created.Id), nil, nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteConference_NotFound_404(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	adminTok := adminToken(t, tm)
	resp := doJSON(t, http.MethodDelete, ts.URL+"/api/v1/conferences/00000000-0000-0000-0000-000000000001", nil, sessionCookie(adminTok))
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ---- Archive / Unarchive ----

func TestArchiveUnarchive_Idempotent(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	url := fmt.Sprintf("%s/api/v1/conferences/%s", ts.URL, created.Id)

	// Archive twice — both 200
	for i := range 2 {
		resp = doJSON(t, http.MethodPost, url+"/archive", nil, sessionCookie(tok))
		require.Equal(t, http.StatusOK, resp.StatusCode, "archive attempt %d", i+1)
		var body apimod.Conference
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.NotNil(t, body.ArchivedAt, "archived_at must be set")
	}

	// Unarchive twice — both 200
	for i := range 2 {
		resp = doJSON(t, http.MethodPost, url+"/unarchive", nil, sessionCookie(tok))
		require.Equal(t, http.StatusOK, resp.StatusCode, "unarchive attempt %d", i+1)
		var body apimod.Conference
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		require.Nil(t, body.ArchivedAt, "archived_at must be nil")
	}
}

func TestArchiveConference_Anonymous_401(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", createConferenceInput(), sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	resp = doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/conferences/%s/archive", ts.URL, created.Id), nil, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ---- Tags ----

func TestListTags_RequiresAuth(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, _ := newConferenceTestServer(t, dsn)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/tags", nil, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListTags_Member(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	// Create a conference with a tag to ensure it's persisted
	tok := memberToken(t, tm)
	slugs := []string{"cs-ed"}
	inp := createConferenceInput()
	inp.TagSlugs = &slugs
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", inp, sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = doJSON(t, http.MethodGet, ts.URL+"/api/v1/tags", nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tags []apimod.Tag
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tags))
	require.Len(t, tags, 1)
	require.Equal(t, "cs-ed", tags[0].Slug)
}

// ---- Tracks ----

func TestListTracks_RequiresAuth(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, _ := newConferenceTestServer(t, dsn)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/tracks", nil, nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListTracks_Member(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/tracks", nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tracks []apimod.Track
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tracks))
	require.NotEmpty(t, tracks, "seed tracks should be present")
}
