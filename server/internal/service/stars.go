package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"confero/internal/api"
	"confero/internal/repository"
)

// StarService handles starring and unstarring conferences.
type StarService struct {
	pool *pgxpool.Pool
	q    *repository.Queries
}

// NewStarService creates a StarService backed by the given connection pool.
func NewStarService(pool *pgxpool.Pool) *StarService {
	return &StarService{pool: pool, q: repository.New(pool)}
}

// Star marks a conference as starred by userID. Idempotent. Materializes reminder rows
// in the same transaction.
func (s *StarService) Star(ctx context.Context, userID, conferenceID uuid.UUID) error {
	conf, err := s.q.GetConference(ctx, conferenceID)
	if err != nil {
		return ErrNotFound
	}

	settings, err := s.q.GetUserSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user settings: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	if err := qtx.CreateStar(ctx, repository.CreateStarParams{
		UserID:       userID,
		ConferenceID: conferenceID,
	}); err != nil {
		return fmt.Errorf("create star: %w", err)
	}

	deadlines := conferenceDeadlines(conf)
	if err := materializeReminders(ctx, qtx, userID, conferenceID,
		settings.Timezone, settings.ReminderLeadDays, deadlines); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Unstar removes a user's star from a conference, cancelling any pending reminders.
// Idempotent (no error if the conference doesn't exist or was never starred).
func (s *StarService) Unstar(ctx context.Context, userID, conferenceID uuid.UUID) error {
	// Verify the conference exists.
	if _, err := s.q.GetConference(ctx, conferenceID); err != nil {
		return ErrNotFound
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	if _, err := qtx.DeleteStar(ctx, repository.DeleteStarParams{
		UserID:       userID,
		ConferenceID: conferenceID,
	}); err != nil {
		return fmt.Errorf("delete star: %w", err)
	}

	if err := qtx.CancelUserConferenceReminders(ctx, repository.CancelUserConferenceRemindersParams{
		UserID:       userID,
		ConferenceID: conferenceID,
	}); err != nil {
		return fmt.Errorf("cancel reminders: %w", err)
	}

	return tx.Commit(ctx)
}

// ListStarred returns all conferences starred by userID, with tags and tracks.
func (s *StarService) ListStarred(ctx context.Context, userID uuid.UUID) ([]api.Conference, error) {
	rows, err := s.q.ListUserStarredConferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list starred: %w", err)
	}

	result := make([]api.Conference, 0, len(rows))
	for _, row := range rows {
		c := row
		tags, err := s.q.GetConferenceTags(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("get tags for %s: %w", c.ID, err)
		}
		tracks, err := s.q.GetConferenceTracks(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("get tracks for %s: %w", c.ID, err)
		}
		result = append(result, toAPIConference(c, tags, tracks))
	}
	return result, nil
}
