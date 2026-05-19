package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDigestScheduling_BerlinDST verifies that a user configured for
// Europe/Berlin, weekly_digest_day=1 (Monday), weekly_digest_hour=8 gets a
// digest_dispatch_log row inserted when the scheduler probe runs at
// 06:00 UTC on a Monday in summer (DST, UTC+2 => 08:00 Berlin).
func TestDigestScheduling_BerlinDST(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	// Monday 2026-06-08 06:00 UTC == 08:00 Europe/Berlin (CEST, UTC+2).
	probeTime := time.Date(2026, 6, 8, 6, 0, 0, 0, time.UTC)

	userID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, oidc_issuer, oidc_subject, email, display_name)
		 VALUES ($1, 'test', $2, $3, 'Berlin User')`,
		userID, "sub-"+userID.String(), userID.String()+"@test.org",
	)
	require.NoError(t, err)

	// Set up: digest enabled, Monday (day=1), 08:00, Europe/Berlin.
	_, err = pool.Exec(ctx,
		`INSERT INTO user_settings (user_id, weekly_digest_enabled, weekly_digest_day, weekly_digest_hour, timezone)
		 VALUES ($1, true, 1, 8, 'Europe/Berlin')
		 ON CONFLICT (user_id) DO UPDATE
		   SET weekly_digest_enabled = true,
		       weekly_digest_day     = 1,
		       weekly_digest_hour    = 8,
		       timezone              = 'Europe/Berlin'`,
		userID,
	)
	require.NoError(t, err)

	sched := newSched(pool, nil, func() time.Time { return probeTime }, newRegistry())
	require.NoError(t, sched.ScheduleDigests(ctx))

	// Verify a row was inserted.
	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM digest_dispatch_log WHERE user_id = $1`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "exactly one digest row should be scheduled")

	// Verify week_starting is the Monday of that week.
	var weekStarting pgtype.Date
	err = pool.QueryRow(ctx,
		`SELECT week_starting FROM digest_dispatch_log WHERE user_id = $1`, userID,
	).Scan(&weekStarting)
	require.NoError(t, err)
	expectedMonday := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedMonday, weekStarting.Time, "week_starting should be the Monday of the probe week")
}

// TestDigestScheduling_WrongHour verifies that no row is inserted when
// the probe runs at a different hour than the user's configured digest hour.
func TestDigestScheduling_WrongHour(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	// Monday 2026-06-08 05:00 UTC == 07:00 Berlin — NOT the configured 08:00.
	probeTime := time.Date(2026, 6, 8, 5, 0, 0, 0, time.UTC)

	userID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, oidc_issuer, oidc_subject, email, display_name)
		 VALUES ($1, 'test', $2, $3, 'Berlin User2')`,
		userID, "sub-"+userID.String(), userID.String()+"@test.org",
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO user_settings (user_id, weekly_digest_enabled, weekly_digest_day, weekly_digest_hour, timezone)
		 VALUES ($1, true, 1, 8, 'Europe/Berlin')
		 ON CONFLICT (user_id) DO UPDATE
		   SET weekly_digest_enabled = true,
		       weekly_digest_day     = 1,
		       weekly_digest_hour    = 8,
		       timezone              = 'Europe/Berlin'`,
		userID,
	)
	require.NoError(t, err)

	sched := newSched(pool, nil, func() time.Time { return probeTime }, newRegistry())
	require.NoError(t, sched.ScheduleDigests(ctx))

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM digest_dispatch_log WHERE user_id = $1`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "no digest row should be inserted at the wrong hour")
}

// TestDigestScheduling_Idempotent verifies that running the probe twice in
// the same hour does not insert duplicate rows (ON CONFLICT DO NOTHING).
func TestDigestScheduling_Idempotent(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	probeTime := time.Date(2026, 6, 8, 6, 0, 0, 0, time.UTC) // Monday 08:00 Berlin

	userID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, oidc_issuer, oidc_subject, email, display_name)
		 VALUES ($1, 'test', $2, $3, 'Idempotent User')`,
		userID, "sub-"+userID.String(), userID.String()+"@test.org",
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO user_settings (user_id, weekly_digest_enabled, weekly_digest_day, weekly_digest_hour, timezone)
		 VALUES ($1, true, 1, 8, 'Europe/Berlin')
		 ON CONFLICT (user_id) DO UPDATE
		   SET weekly_digest_enabled = true,
		       weekly_digest_day     = 1,
		       weekly_digest_hour    = 8,
		       timezone              = 'Europe/Berlin'`,
		userID,
	)
	require.NoError(t, err)

	sched := newSched(pool, nil, func() time.Time { return probeTime }, newRegistry())
	require.NoError(t, sched.ScheduleDigests(ctx))
	require.NoError(t, sched.ScheduleDigests(ctx)) // second call same hour

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM digest_dispatch_log WHERE user_id = $1`, userID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "exactly one row despite two probe calls")
}
