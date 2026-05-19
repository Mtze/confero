package tests_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"confero/internal/api"
	"confero/internal/repository"
	"confero/internal/service"
)

// seedUser inserts a user + user_settings row with known ID and returns the ID.
func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) uuid.UUID {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, oidc_issuer, oidc_subject, email, display_name)
		 VALUES ($1, 'test', $2, $3, $4)
		 ON CONFLICT (oidc_issuer, oidc_subject) DO NOTHING`,
		id, "sub-"+id.String(), id.String()+"@example.org", "Test User",
	)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO user_settings (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		id,
	)
	require.NoError(t, err)
	return id
}

// seedConferenceWithDeadlines creates a conference with three set deadlines.
func seedConferenceWithDeadlines(t *testing.T, ctx context.Context, q *repository.Queries) repository.Conference {
	t.Helper()
	now := time.Now().UTC()
	ts := func(d time.Duration) pgtype.Timestamptz {
		return pgtype.Timestamptz{Time: now.Add(d), Valid: true}
	}
	conf, err := q.CreateConference(ctx, repository.CreateConferenceParams{
		Name:             "Reminder Test Conference",
		Acronym:          "RTC",
		Year:             int32(now.Year() + 1),
		Location:         "Munich",
		PrimaryDeadline:  ts(90 * 24 * time.Hour),
		AbstractDeadline: ts(60 * 24 * time.Hour),
		NotificationDate: ts(120 * 24 * time.Hour),
	})
	require.NoError(t, err)
	return conf
}

// TestMaterializationProducesCorrectRows verifies that starring a conference
// with 3 set deadlines and 2 user lead times produces exactly 6 reminder rows.
func TestMaterializationProducesCorrectRows(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	q := repository.New(pool)

	userID := seedUser(t, ctx, pool, uuid.New())
	conf := seedConferenceWithDeadlines(t, ctx, q)

	// Set 2 lead times on the user.
	_, err := pool.Exec(ctx,
		`UPDATE user_settings SET reminder_lead_days = ARRAY[14,7] WHERE user_id = $1`, userID)
	require.NoError(t, err)

	svc := service.NewStarService(pool)
	err = svc.Star(ctx, userID, conf.ID)
	require.NoError(t, err)

	count, err := q.CountReminderRows(ctx, repository.CountReminderRowsParams{
		UserID:       userID,
		ConferenceID: conf.ID,
	})
	require.NoError(t, err)
	// 3 deadlines × 2 lead times = 6 rows
	assert.Equal(t, int64(6), count)
}

// TestUnstarCancelsPendingButNotSentReminders verifies the cancellation semantics:
// pending rows become cancelled, but rows already in 'sent' state remain untouched.
func TestUnstarCancelsPendingButNotSentReminders(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	q := repository.New(pool)

	userID := seedUser(t, ctx, pool, uuid.New())
	conf := seedConferenceWithDeadlines(t, ctx, q)

	_, err := pool.Exec(ctx,
		`UPDATE user_settings SET reminder_lead_days = ARRAY[7] WHERE user_id = $1`, userID)
	require.NoError(t, err)

	svc := service.NewStarService(pool)
	require.NoError(t, svc.Star(ctx, userID, conf.ID))

	// Manually mark one row as 'sent' to simulate a sent reminder.
	_, err = pool.Exec(ctx,
		`UPDATE reminder_dispatch_log
		 SET status = 'sent', sent_at = now()
		 WHERE id = (
		     SELECT id FROM reminder_dispatch_log
		     WHERE user_id = $1 AND conference_id = $2 AND status = 'pending'
		     LIMIT 1
		 )`,
		userID, conf.ID,
	)
	require.NoError(t, err)

	// Unstar — should cancel only the 'pending' rows, not the 'sent' one.
	require.NoError(t, svc.Unstar(ctx, userID, conf.ID))

	var pendingCount, sentCount int64
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reminder_dispatch_log WHERE user_id = $1 AND status = 'pending'`,
		userID).Scan(&pendingCount)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pendingCount, "all pending reminders should be cancelled")

	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reminder_dispatch_log WHERE user_id = $1 AND status = 'sent'`,
		userID).Scan(&sentCount)
	require.NoError(t, err)
	assert.Equal(t, int64(1), sentCount, "sent reminders must not be affected")
}

// TestDeadlineEditRematerializesReminders verifies that updating a conference's
// deadlines through the service layer deletes old pending reminders and inserts
// fresh ones at the updated scheduled_for times.
func TestDeadlineEditRematerializesReminders(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	q := repository.New(pool)

	// Seed a user with a single lead time so row counts are predictable.
	userID := seedUser(t, ctx, pool, uuid.New())
	_, err := pool.Exec(ctx,
		`UPDATE user_settings SET reminder_lead_days = ARRAY[7] WHERE user_id = $1`, userID)
	require.NoError(t, err)

	// Create a conference with one deadline (primary only).
	now := time.Now().UTC()
	originalDeadline := now.Add(90 * 24 * time.Hour)
	conf, err := q.CreateConference(ctx, repository.CreateConferenceParams{
		Name:            "Deadline Edit Test",
		Acronym:         "DET",
		Year:            int32(now.Year() + 1),
		Location:        "Munich",
		PrimaryDeadline: pgtype.Timestamptz{Time: originalDeadline, Valid: true},
	})
	require.NoError(t, err)

	// Star the conference → 1 pending reminder row (1 deadline × 1 lead time).
	starSvc := service.NewStarService(pool)
	require.NoError(t, starSvc.Star(ctx, userID, conf.ID))

	// Capture the original scheduled_for.
	var originalScheduledFor time.Time
	err = pool.QueryRow(ctx,
		`SELECT scheduled_for FROM reminder_dispatch_log WHERE user_id=$1 AND conference_id=$2 AND status='pending'`,
		userID, conf.ID,
	).Scan(&originalScheduledFor)
	require.NoError(t, err)

	// Update the primary deadline 30 days later via the conference service (triggers re-materialization).
	newDeadline := originalDeadline.Add(30 * 24 * time.Hour)
	confSvc := service.NewConferenceService(pool)
	_, err = confSvc.Update(ctx, conf.ID, api.ConferenceInput{
		Name:            conf.Name,
		Acronym:         conf.Acronym,
		Year:            int(conf.Year),
		Location:        conf.Location,
		PrimaryDeadline: &newDeadline,
	}, userID)
	require.NoError(t, err)

	// There should be exactly 1 pending reminder with the new scheduled_for.
	var newScheduledFor time.Time
	err = pool.QueryRow(ctx,
		`SELECT scheduled_for FROM reminder_dispatch_log WHERE user_id=$1 AND conference_id=$2 AND status='pending'`,
		userID, conf.ID,
	).Scan(&newScheduledFor)
	require.NoError(t, err)

	assert.True(t, newScheduledFor.After(originalScheduledFor),
		"new scheduled_for (%v) should be later than original (%v)", newScheduledFor, originalScheduledFor)
}
