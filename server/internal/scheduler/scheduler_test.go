package scheduler_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"confero/internal/database"
	"confero/internal/mail"
	"confero/internal/repository"
	"confero/internal/scheduler"
)

// ---- Test infrastructure ----

func newTestPool(t *testing.T) *pgxpool.Pool {
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
	require.NoError(t, database.RunMigrations(dsn))
	pool, err := database.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func newRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func newSched(pool *pgxpool.Pool, mailer mail.Mailer, now func() time.Time, reg *prometheus.Registry) *scheduler.Scheduler {
	return scheduler.New(scheduler.Config{
		Tick:        time.Hour, // don't auto-tick in tests
		Mailer:      mailer,
		DB:          pool,
		GraceDays:   7,
		Now:         now,
		MaxAttempts: 3,
		Logger:      slog.Default(),
	}, reg)
}

// seedDueReminder inserts a user, conference, star, and a due reminder row.
func seedDueReminder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pastTime time.Time) (userID, confID, reminderID uuid.UUID) {
	t.Helper()
	userID = uuid.New()
	confID = uuid.New()
	reminderID = uuid.New()

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, oidc_issuer, oidc_subject, email, display_name)
		 VALUES ($1, 'test', $2, $3, 'Test User')`,
		userID, "sub-"+userID.String(), userID.String()+"@test.org",
	)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO conferences (id, name, acronym, year, location)
		 VALUES ($1, 'Test Conf', 'TC', 2025, 'Munich')`,
		confID,
	)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO reminder_dispatch_log (id, user_id, conference_id, deadline_kind, lead_time_days, scheduled_for)
		 VALUES ($1, $2, $3, 'submission', 7, $4)`,
		reminderID, userID, confID, pgtype.Timestamptz{Time: pastTime, Valid: true},
	)
	require.NoError(t, err)
	return
}

func reminderStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var status string
	err := pool.QueryRow(ctx,
		`SELECT status FROM reminder_dispatch_log WHERE id = $1`, id,
	).Scan(&status)
	require.NoError(t, err)
	return status
}

func reminderAttempts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) int {
	t.Helper()
	var attempts int
	err := pool.QueryRow(ctx,
		`SELECT attempts FROM reminder_dispatch_log WHERE id = $1`, id,
	).Scan(&attempts)
	require.NoError(t, err)
	return attempts
}

// ---- Tests ----

// TestDispatch_HappyPath verifies that a due reminder is marked 'sent' and the
// FakeMailer captures exactly one message.
func TestDispatch_HappyPath(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Minute)

	_, _, rid := seedDueReminder(t, ctx, pool, past)

	fm := &mail.FakeMailer{}
	sched := newSched(pool, fm, time.Now, newRegistry())
	require.NoError(t, sched.Tick(ctx))

	assert.Equal(t, "sent", reminderStatus(t, ctx, pool, rid))
	assert.Len(t, fm.Sent(), 1)
}

// TestDispatch_SkipLocked verifies that two scheduler goroutines racing on the
// same due row do not double-send: one picks it up, the other sees no rows.
func TestDispatch_SkipLocked(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Minute)
	_, _, rid := seedDueReminder(t, ctx, pool, past)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []int // each goroutine appends the number of messages it sent
	)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fm := &mail.FakeMailer{}
			s := newSched(pool, fm, time.Now, newRegistry())
			err := s.Tick(ctx)
			require.NoError(t, err)
			mu.Lock()
			results = append(results, len(fm.Sent()))
			mu.Unlock()
		}()
	}
	wg.Wait()

	total := results[0] + results[1]
	assert.Equal(t, 1, total, "exactly one goroutine should have sent the reminder")
	assert.Equal(t, "sent", reminderStatus(t, ctx, pool, rid))
}

// TestDispatch_TransientError verifies that a send error increments attempts
// and leaves the row as 'pending'.
func TestDispatch_TransientError(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Minute)
	_, _, rid := seedDueReminder(t, ctx, pool, past)

	fm := &errorMailer{err: fmt.Errorf("smtp timeout")}
	sched := newSched(pool, fm, time.Now, newRegistry())
	require.NoError(t, sched.Tick(ctx))

	assert.Equal(t, "pending", reminderStatus(t, ctx, pool, rid))
	assert.Equal(t, 1, reminderAttempts(t, ctx, pool, rid))
}

// TestDispatch_PermanentError verifies that after MaxAttempts the row is
// marked 'failed'.
func TestDispatch_PermanentError(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Minute)
	_, _, rid := seedDueReminder(t, ctx, pool, past)

	fm := &errorMailer{err: fmt.Errorf("permanent failure")}

	for i := 0; i < 3; i++ {
		sched := newSched(pool, fm, time.Now, newRegistry())
		require.NoError(t, sched.Tick(ctx))
	}

	assert.Equal(t, "failed", reminderStatus(t, ctx, pool, rid))
}

// TestArchiveSweeper verifies that conferences whose event_end_date is older
// than grace_days get archived and their pending reminders are cancelled.
func TestArchiveSweeper(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	q := repository.New(pool)

	// Create a conference that ended 10 days ago (grace_days = 7).
	confID := uuid.New()
	eventEnd := time.Now().UTC().AddDate(0, 0, -10)
	_, err := pool.Exec(ctx,
		`INSERT INTO conferences (id, name, acronym, year, location, event_end_date)
		 VALUES ($1, 'Expired Conf', 'EXP', 2024, 'Munich', $2)`,
		confID, pgtype.Date{Time: eventEnd, Valid: true},
	)
	require.NoError(t, err)

	// Add a user + a pending reminder for this conference.
	userID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, oidc_issuer, oidc_subject, email, display_name)
		 VALUES ($1, 'test', $2, $3, 'Sweeper User')`,
		userID, "sub-"+userID.String(), userID.String()+"@test.org",
	)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO reminder_dispatch_log (user_id, conference_id, deadline_kind, lead_time_days, scheduled_for)
		 VALUES ($1, $2, 'submission', 7, now() + interval '10 days')`,
		userID, confID,
	)
	require.NoError(t, err)

	sched := newSched(pool, &mail.FakeMailer{}, time.Now, newRegistry())
	require.NoError(t, sched.SweepArchive(ctx))

	conf, err := q.GetConference(ctx, confID)
	require.NoError(t, err)
	assert.True(t, conf.ArchivedAt.Valid, "conference should be archived")

	var pending int64
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM reminder_dispatch_log WHERE conference_id = $1 AND status = 'pending'`,
		confID,
	).Scan(&pending)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pending, "pending reminders should be cancelled")
}

// TestGracefulShutdown verifies that cancelling the context stops the Run loop
// promptly without hanging.
func TestGracefulShutdown(t *testing.T) {
	pool := newTestPool(t)

	sched := newSched(pool, &mail.FakeMailer{}, time.Now, newRegistry())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- sched.Run(ctx)
	}()

	// Give it a moment to start, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not shut down within 5 seconds")
	}
}

// ---- Test helpers ----

// errorMailer always returns the configured error.
type errorMailer struct {
	err error
}

func (e *errorMailer) Send(_ context.Context, _ mail.Message) error {
	return e.err
}
