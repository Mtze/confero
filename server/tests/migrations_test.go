package tests_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"confero/db"
	"confero/internal/database"
)

func newPostgresContainer(t *testing.T) (dsn string) {
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

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return connStr
}

func TestMigrationsRoundTrip(t *testing.T) {
	dsn := newPostgresContainer(t)

	src, err := iofs.New(db.Migrations, "migrations")
	require.NoError(t, err)

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })

	require.NoError(t, m.Up(), "migrate up failed")

	require.NoError(t, m.Down(), "migrate down failed")
}

func TestRunMigrationsHelper(t *testing.T) {
	dsn := newPostgresContainer(t)

	err := database.RunMigrations(dsn)
	require.NoError(t, err, "RunMigrations should apply all migrations")

	err = database.RunMigrations(dsn)
	require.NoError(t, err, "RunMigrations should be idempotent (ErrNoChange)")
}
