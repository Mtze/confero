// Package scheduler implements the in-process background job runner.
//
// A single goroutine ticks at a configurable interval. Each tick it:
//  1. Dispatches due reminder emails.
//  2. Runs the archive sweeper (auto-archives expired conferences).
//
// SELECT ... FOR UPDATE SKIP LOCKED is used throughout so a future move to
// multiple replicas or an extracted scheduler process requires no code change.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"confero/internal/mail"
	"confero/internal/repository"
)

const (
	defaultMaxAttempts = 3
	archiveInterval    = time.Hour
	dispatchBatchLimit = 50
)

// Config holds scheduler runtime configuration.
type Config struct {
	Tick      time.Duration
	Mailer    mail.Mailer
	DB        *pgxpool.Pool
	GraceDays int
	// Now returns the current time. Defaults to time.Now; injectable for tests.
	Now func() time.Time
	// MaxAttempts before a reminder is marked 'failed'. Defaults to 3.
	MaxAttempts int
	Logger      *slog.Logger
}

// Scheduler is the in-process background job runner.
type Scheduler struct {
	cfg           Config
	q             *repository.Queries
	pendingGauge  prometheus.Gauge
	lastArchiveSweep time.Time
}

// New creates a Scheduler with the given configuration.
func New(cfg Config, reg prometheus.Registerer) *Scheduler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = defaultMaxAttempts
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	gauge := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Name: "confero_scheduler_pending_reminders",
		Help: "Number of pending reminder_dispatch_log rows.",
	})
	return &Scheduler{
		cfg:          cfg,
		q:            repository.New(cfg.DB),
		pendingGauge: gauge,
	}
}

// Run starts the scheduler tick loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.Tick)
	defer ticker.Stop()

	// Run immediately on start.
	if err := s.tick(ctx); err != nil {
		s.cfg.Logger.ErrorContext(ctx, "scheduler tick", "err", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				s.cfg.Logger.ErrorContext(ctx, "scheduler tick", "err", err)
			}
		}
	}
}

// Tick runs one scheduler cycle synchronously. Exported for testing.
func (s *Scheduler) Tick(ctx context.Context) error {
	return s.tick(ctx)
}

// SweepArchive runs the archive sweeper synchronously. Exported for testing.
func (s *Scheduler) SweepArchive(ctx context.Context) error {
	return s.sweepArchive(ctx)
}

func (s *Scheduler) tick(ctx context.Context) error {
	if err := s.dispatchReminders(ctx); err != nil {
		return fmt.Errorf("dispatch reminders: %w", err)
	}
	if err := s.updatePendingGauge(ctx); err != nil {
		s.cfg.Logger.WarnContext(ctx, "pending gauge update failed", "err", err)
	}
	if time.Since(s.lastArchiveSweep) >= archiveInterval {
		if err := s.sweepArchive(ctx); err != nil {
			return fmt.Errorf("archive sweep: %w", err)
		}
		s.lastArchiveSweep = s.cfg.Now()
	}
	return nil
}

func (s *Scheduler) dispatchReminders(ctx context.Context) error {
	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)
	now := pgtype.Timestamptz{Time: s.cfg.Now(), Valid: true}

	rows, err := qtx.SelectDueReminders(ctx, now)
	if err != nil {
		return fmt.Errorf("select due reminders: %w", err)
	}

	for _, row := range rows {
		msg := mail.Message{
			To:             row.UserEmail,
			Subject:        reminderSubject(row),
			ConferenceID:   row.ConferenceID.String(),
			ConferenceName: row.ConferenceName,
			DeadlineKind:   row.DeadlineKind,
			LeadTimeDays:   row.LeadTimeDays,
		}

		if err := s.cfg.Mailer.Send(ctx, msg); err != nil {
			newAttempts := int(row.Attempts) + 1
			if newAttempts >= s.cfg.MaxAttempts {
				if ferr := qtx.MarkReminderFailed(ctx, repository.MarkReminderFailedParams{
					ID:        row.ID,
					LastError: ptr(err.Error()),
				}); ferr != nil {
					s.cfg.Logger.ErrorContext(ctx, "mark reminder failed", "id", row.ID, "err", ferr)
				}
			} else {
				if aerr := qtx.IncrementReminderAttempt(ctx, repository.IncrementReminderAttemptParams{
					ID:        row.ID,
					LastError: ptr(err.Error()),
				}); aerr != nil {
					s.cfg.Logger.ErrorContext(ctx, "increment reminder attempt", "id", row.ID, "err", aerr)
				}
			}
			s.cfg.Logger.WarnContext(ctx, "reminder send failed",
				"id", row.ID, "attempt", newAttempts, "err", err)
			continue
		}

		if err := qtx.MarkReminderSent(ctx, row.ID); err != nil {
			s.cfg.Logger.ErrorContext(ctx, "mark reminder sent", "id", row.ID, "err", err)
			continue
		}
		s.cfg.Logger.InfoContext(ctx, "reminder sent",
			"id", row.ID, "to", row.UserEmail, "conference", row.ConferenceName, "kind", row.DeadlineKind)
	}

	return tx.Commit(ctx)
}

func (s *Scheduler) sweepArchive(ctx context.Context) error {
	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	ids, err := qtx.SelectExpiredConferences(ctx, int32(s.cfg.GraceDays))
	if err != nil {
		return fmt.Errorf("select expired: %w", err)
	}

	for _, id := range ids {
		if err := qtx.CancelConferenceReminders(ctx, id); err != nil {
			return fmt.Errorf("cancel reminders for %s: %w", id, err)
		}
		if _, err := qtx.ArchiveConference(ctx, id); err != nil {
			return fmt.Errorf("archive conference %s: %w", id, err)
		}
		s.cfg.Logger.InfoContext(ctx, "auto-archived conference", "id", id)
	}

	return tx.Commit(ctx)
}

func (s *Scheduler) updatePendingGauge(ctx context.Context) error {
	count, err := s.q.CountPendingReminders(ctx)
	if err != nil {
		return err
	}
	s.pendingGauge.Set(float64(count))
	return nil
}

func reminderSubject(row repository.SelectDueRemindersRow) string {
	return fmt.Sprintf("[Confero] Reminder: %s — %s deadline",
		row.ConferenceAcronym, row.DeadlineKind)
}

func ptr(s string) *string { return &s }
