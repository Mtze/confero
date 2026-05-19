package tests_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"confero/internal/database"
	"confero/internal/repository"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := newPostgresContainer(t)
	require.NoError(t, database.RunMigrations(dsn))

	ctx := context.Background()
	pool, err := database.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestConferenceCreateAndGet(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	q := repository.New(pool)

	name := "International Conference on CS Education"
	acronym := "SIGCSE"
	year := int32(time.Now().Year())
	location := "Virtual"

	created, err := q.CreateConference(ctx, repository.CreateConferenceParams{
		Name:     name,
		Acronym:  acronym,
		Year:     year,
		Location: location,
	})
	require.NoError(t, err)
	require.Equal(t, name, created.Name)
	require.Equal(t, acronym, created.Acronym)
	require.Equal(t, year, created.Year)

	fetched, err := q.GetConference(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, fetched.ID)
	require.Equal(t, name, fetched.Name)
	require.Equal(t, acronym, fetched.Acronym)
}

func TestConferenceList(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	q := repository.New(pool)

	_, err := q.CreateConference(ctx, repository.CreateConferenceParams{
		Name:     "Conference A",
		Acronym:  "CONF-A",
		Year:     2026,
		Location: "Munich",
	})
	require.NoError(t, err)

	_, err = q.CreateConference(ctx, repository.CreateConferenceParams{
		Name:     "Conference B",
		Acronym:  "CONF-B",
		Year:     2026,
		Location: "Berlin",
	})
	require.NoError(t, err)

	list, err := q.ListConferences(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(list), 2)
}
