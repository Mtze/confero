package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimod "confero/internal/api"
)

// ---- Helpers ----

func createConferenceWithDeadlines(t *testing.T, ts *httptest.Server, tok string) apimod.Conference {
	t.Helper()
	primary := time.Now().UTC().Add(60 * 24 * time.Hour)
	inp := apimod.ConferenceInput{
		Name:            "Star Test Conference",
		Acronym:         "STC",
		Year:            2027,
		Location:        "Munich, Germany",
		PrimaryDeadline: &primary,
	}
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/conferences", inp, sessionCookie(tok))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var conf apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&conf))
	return conf
}

// ---- Star / Unstar ----

func TestStarConference_Member_204(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	conf := createConferenceWithDeadlines(t, ts, tok)

	resp := doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id), nil, sessionCookie(tok))
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestStarConference_Idempotent(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	conf := createConferenceWithDeadlines(t, ts, tok)

	starURL := fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id)

	resp := doJSON(t, http.MethodPost, starURL, nil, sessionCookie(tok))
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Second star is idempotent.
	resp = doJSON(t, http.MethodPost, starURL, nil, sessionCookie(tok))
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestStarConference_Anonymous_401(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	conf := createConferenceWithDeadlines(t, ts, tok)

	resp := doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id), nil, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestStarConference_NotFound_404(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/conferences/00000000-0000-0000-0000-000000000000/stars", ts.URL), nil, sessionCookie(tok))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUnstarConference_Member_204(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	conf := createConferenceWithDeadlines(t, ts, tok)

	starURL := fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id)
	doJSON(t, http.MethodPost, starURL, nil, sessionCookie(tok))

	resp := doJSON(t, http.MethodDelete, starURL, nil, sessionCookie(tok))
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestUnstarConference_Idempotent(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	conf := createConferenceWithDeadlines(t, ts, tok)

	starURL := fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id)

	// Unstar without having starred first — idempotent.
	resp := doJSON(t, http.MethodDelete, starURL, nil, sessionCookie(tok))
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestUnstarConference_CancelsPendingButNotSentReminders(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	conf := createConferenceWithDeadlines(t, ts, tok)

	starURL := fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id)

	// Star → reminders are materialized.
	resp := doJSON(t, http.MethodPost, starURL, nil, sessionCookie(tok))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Unstar → pending reminders should be cancelled.
	resp = doJSON(t, http.MethodDelete, starURL, nil, sessionCookie(tok))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// The conference should no longer appear in my stars.
	resp = doJSON(t, http.MethodGet, ts.URL+"/api/v1/me/stars", nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var starred []apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&starred))
	assert.Empty(t, starred)
}

// ---- List my stars ----

func TestListMyStars_Empty(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/me/stars", nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var starred []apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&starred))
	assert.Empty(t, starred)
}

func TestListMyStars_ReturnsStarredConferences(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	conf := createConferenceWithDeadlines(t, ts, tok)

	doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id), nil, sessionCookie(tok))

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/me/stars", nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var starred []apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&starred))
	require.Len(t, starred, 1)
	assert.Equal(t, conf.Id, starred[0].Id)
}

func TestListMyStars_Anonymous_401(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, _ := newConferenceTestServer(t, dsn)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/me/stars", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ---- Settings ----

func TestGetMySettings_Member_200(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/me/settings", nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var settings apimod.UserSettings
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&settings))
	assert.Equal(t, "Europe/Berlin", settings.Timezone)
	assert.NotEmpty(t, settings.ReminderLeadDays)
}

func TestGetMySettings_Anonymous_401(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, _ := newConferenceTestServer(t, dsn)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/me/settings", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUpdateMySettings_Member_200(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	input := apimod.UserSettingsInput{
		Timezone:                 "America/New_York",
		ReminderLeadDays:         []int{30, 14},
		WeeklyDigestEnabled:      true,
		WeeklyDigestDay:          1,
		WeeklyDigestHour:         8,
		WeeklyDigestHorizonWeeks: 4,
	}

	resp := doJSON(t, http.MethodPut, ts.URL+"/api/v1/me/settings", input, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var settings apimod.UserSettings
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&settings))
	assert.Equal(t, "America/New_York", settings.Timezone)
	assert.Equal(t, []int{30, 14}, settings.ReminderLeadDays)
	assert.True(t, settings.WeeklyDigestEnabled)
}

func TestUpdateMySettings_InvalidTimezone_400(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)
	input := apimod.UserSettingsInput{
		Timezone:                 "Not/ATimezone",
		ReminderLeadDays:         []int{7},
		WeeklyDigestEnabled:      false,
		WeeklyDigestDay:          1,
		WeeklyDigestHour:         8,
		WeeklyDigestHorizonWeeks: 4,
	}

	resp := doJSON(t, http.MethodPut, ts.URL+"/api/v1/me/settings", input, sessionCookie(tok))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdateMySettings_RematerializesReminders(t *testing.T) {
	dsn := newPostgresTestDSN(t)
	ts, tm := newConferenceTestServer(t, dsn)
	defer ts.Close()

	tok := memberToken(t, tm)

	// Create a conference with a primary deadline.
	conf := createConferenceWithDeadlines(t, ts, tok)

	// Star it (materializes reminders with default lead times [28,14,7,1] = 4 rows).
	doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/conferences/%s/stars", ts.URL, conf.Id), nil, sessionCookie(tok))

	// Update settings with new lead times — re-materialization runs.
	input := apimod.UserSettingsInput{
		Timezone:                 "Europe/Berlin",
		ReminderLeadDays:         []int{10, 3},
		WeeklyDigestEnabled:      false,
		WeeklyDigestDay:          1,
		WeeklyDigestHour:         8,
		WeeklyDigestHorizonWeeks: 4,
	}
	resp := doJSON(t, http.MethodPut, ts.URL+"/api/v1/me/settings", input, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// The conference is still starred.
	resp = doJSON(t, http.MethodGet, ts.URL+"/api/v1/me/stars", nil, sessionCookie(tok))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var starred []apimod.Conference
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&starred))
	assert.Len(t, starred, 1)
}
