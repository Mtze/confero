package database

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"confero/db"
)

// NewPool creates a connection pool using the provided DSN.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pool ping: %w", err)
	}
	return pool, nil
}

// RunMigrations applies all pending up-migrations from the embedded SQL files.
func RunMigrations(dsn string) error {
	src, err := iofs.New(db.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("iofs.New: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
