// Package scheduler implements the in-process background job runner.
//
// A single goroutine ticks at a configurable interval. Each tick it:
//  1. Dispatches due reminder emails.
//  2. Dispatches due digest emails.
//  3. Runs the archive sweeper (auto-archives expired conferences).
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
	// MaxAttempts before a reminder/digest is marked 'failed'. Defaults to 3.
	MaxAttempts int
	Logger      *slog.Logger
}

// Scheduler is the in-process background job runner.
type Scheduler struct {
	cfg              Config
	q                *repository.Queries
	pendingReminders prometheus.Gauge
	pendingDigests   prometheus.Gauge
	emailsSent       *prometheus.CounterVec
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
	prom := promauto.With(reg)
	return &Scheduler{
		cfg: cfg,
		q:   repository.New(cfg.DB),
		pendingReminders: prom.NewGauge(prometheus.GaugeOpts{
			Name: "confero_scheduler_pending_reminders",
			Help: "Number of pending reminder_dispatch_log rows.",
		}),
		pendingDigests: prom.NewGauge(prometheus.GaugeOpts{
			Name: "confero_scheduler_pending_digests",
			Help: "Number of pending digest_dispatch_log rows.",
		}),
		emailsSent: prom.NewCounterVec(prometheus.CounterOpts{
			Name: "confero_emails_sent_total",
			Help: "Total emails dispatched by the scheduler.",
		}, []string{"kind", "result"}),
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

// ScheduleDigests runs the digest scheduling probe synchronously. Exported for testing.
func (s *Scheduler) ScheduleDigests(ctx context.Context) error {
	return s.scheduleDigests(ctx)
}

func (s *Scheduler) tick(ctx context.Context) error {
	if err := s.dispatchReminders(ctx); err != nil {
		return fmt.Errorf("dispatch reminders: %w", err)
	}
	if err := s.scheduleDigests(ctx); err != nil {
		s.cfg.Logger.WarnContext(ctx, "digest scheduling probe failed", "err", err)
	}
	if err := s.dispatchDigests(ctx); err != nil {
		return fmt.Errorf("dispatch digests: %w", err)
	}
	if err := s.updateGauges(ctx); err != nil {
		s.cfg.Logger.WarnContext(ctx, "gauge update failed", "err", err)
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
		deadlineTime := reminderDeadlineTime(row)

		bodyText, bodyHTML, err := mail.RenderReminder(mail.ReminderData{
			UserName:          row.UserName,
			ConferenceName:    row.ConferenceName,
			ConferenceAcronym: row.ConferenceAcronym,
			DeadlineKind:      row.DeadlineKind,
			DeadlineDate:      deadlineTime,
			LeadTimeDays:      row.LeadTimeDays,
		})
		if err != nil {
			s.cfg.Logger.ErrorContext(ctx, "render reminder template", "id", row.ID, "err", err)
			continue
		}

		msg := mail.Message{
			To:       row.UserEmail,
			Subject:  reminderSubject(row),
			BodyText: bodyText,
			BodyHTML: bodyHTML,
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
				s.emailsSent.WithLabelValues("reminder", "failed").Inc()
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
		s.emailsSent.WithLabelValues("reminder", "sent").Inc()
		s.cfg.Logger.InfoContext(ctx, "reminder sent",
			"id", row.ID, "to", row.UserEmail, "conference", row.ConferenceName, "kind", row.DeadlineKind)
	}

	return tx.Commit(ctx)
}

func (s *Scheduler) dispatchDigests(ctx context.Context) error {
	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)
	now := pgtype.Timestamptz{Time: s.cfg.Now(), Valid: true}

	rows, err := qtx.SelectDueDigests(ctx, now)
	if err != nil {
		return fmt.Errorf("select due digests: %w", err)
	}

	for _, row := range rows {
		items, err := s.buildDigestItems(ctx, row)
		if err != nil {
			s.cfg.Logger.ErrorContext(ctx, "build digest items", "id", row.ID, "err", err)
			continue
		}

		if len(items) == 0 {
			if err := qtx.MarkDigestSkipped(ctx, row.ID); err != nil {
				s.cfg.Logger.ErrorContext(ctx, "mark digest skipped", "id", row.ID, "err", err)
			}
			s.cfg.Logger.InfoContext(ctx, "digest skipped (no upcoming deadlines)", "id", row.ID, "user", row.UserEmail)
			continue
		}

		weekStart := row.WeekStarting.Time
		bodyText, bodyHTML, err := mail.RenderDigest(mail.DigestData{
			UserName:  row.UserName,
			WeekStart: weekStart,
			Items:     items,
		})
		if err != nil {
			s.cfg.Logger.ErrorContext(ctx, "render digest template", "id", row.ID, "err", err)
			continue
		}

		msg := mail.Message{
			To:       row.UserEmail,
			Subject:  fmt.Sprintf("[Confero] Weekly digest — %s", weekStart.Format("02 Jan 2006")),
			BodyText: bodyText,
			BodyHTML: bodyHTML,
		}

		if err := s.cfg.Mailer.Send(ctx, msg); err != nil {
			newAttempts := int(row.Attempts) + 1
			if newAttempts >= s.cfg.MaxAttempts {
				if ferr := qtx.MarkDigestFailed(ctx, repository.MarkDigestFailedParams{
					ID:        row.ID,
					LastError: ptr(err.Error()),
				}); ferr != nil {
					s.cfg.Logger.ErrorContext(ctx, "mark digest failed", "id", row.ID, "err", ferr)
				}
				s.emailsSent.WithLabelValues("digest", "failed").Inc()
			} else {
				if aerr := qtx.IncrementDigestAttempt(ctx, repository.IncrementDigestAttemptParams{
					ID:        row.ID,
					LastError: ptr(err.Error()),
				}); aerr != nil {
					s.cfg.Logger.ErrorContext(ctx, "increment digest attempt", "id", row.ID, "err", aerr)
				}
			}
			s.cfg.Logger.WarnContext(ctx, "digest send failed",
				"id", row.ID, "attempt", newAttempts, "err", err)
			continue
		}

		if err := qtx.MarkDigestSent(ctx, row.ID); err != nil {
			s.cfg.Logger.ErrorContext(ctx, "mark digest sent", "id", row.ID, "err", err)
			continue
		}
		s.emailsSent.WithLabelValues("digest", "sent").Inc()
		s.cfg.Logger.InfoContext(ctx, "digest sent",
			"id", row.ID, "to", row.UserEmail, "items", len(items))
	}

	return tx.Commit(ctx)
}

// buildDigestItems fetches upcoming deadlines for the user within their horizon.
func (s *Scheduler) buildDigestItems(ctx context.Context, row repository.SelectDueDigestsRow) ([]mail.DigestItem, error) {
	confs, err := s.q.ListUserStarredConferences(ctx, row.UserID)
	if err != nil {
		return nil, fmt.Errorf("list starred: %w", err)
	}

	horizon := s.cfg.Now().AddDate(0, 0, int(row.WeeklyDigestHorizonWeeks)*7)
	now := s.cfg.Now()

	var items []mail.DigestItem
	for _, c := range confs {
		if c.ArchivedAt.Valid {
			continue
		}
		type dl struct {
			kind string
			at   time.Time
			set  bool
		}
		candidates := []dl{
			{"submission", c.PrimaryDeadline.Time, c.PrimaryDeadline.Valid},
			{"abstract", c.AbstractDeadline.Time, c.AbstractDeadline.Valid},
			{"notification", c.NotificationDate.Time, c.NotificationDate.Valid},
			{"camera_ready", c.CameraReadyDate.Time, c.CameraReadyDate.Valid},
		}
		for _, d := range candidates {
			if !d.set {
				continue
			}
			if d.at.After(now) && !d.at.After(horizon) {
				items = append(items, mail.DigestItem{
					ConferenceName:    c.Name,
					ConferenceAcronym: c.Acronym,
					DeadlineKind:      d.kind,
					DeadlineDate:      d.at,
				})
			}
		}
	}
	return items, nil
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

func (s *Scheduler) updateGauges(ctx context.Context) error {
	rc, err := s.q.CountPendingReminders(ctx)
	if err != nil {
		return err
	}
	s.pendingReminders.Set(float64(rc))

	dc, err := s.q.CountPendingDigests(ctx)
	if err != nil {
		return err
	}
	s.pendingDigests.Set(float64(dc))
	return nil
}

// reminderDeadlineTime picks the concrete timestamptz for the given deadline kind.
func reminderDeadlineTime(row repository.SelectDueRemindersRow) time.Time {
	switch row.DeadlineKind {
	case "submission":
		return row.PrimaryDeadline.Time
	case "abstract":
		return row.AbstractDeadline.Time
	case "notification":
		return row.NotificationDate.Time
	case "camera_ready":
		return row.CameraReadyDate.Time
	}
	return time.Time{}
}

func reminderSubject(row repository.SelectDueRemindersRow) string {
	return fmt.Sprintf("[Confero] Reminder: %s - %s deadline",
		row.ConferenceAcronym, row.DeadlineKind)
}

func ptr(s string) *string { return &s }
