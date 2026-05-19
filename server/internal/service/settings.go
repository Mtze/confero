package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"confero/internal/api"
	"confero/internal/repository"
)

// SettingsService handles per-user reminder preferences.
type SettingsService struct {
	pool *pgxpool.Pool
	q    *repository.Queries
}

// NewSettingsService creates a SettingsService backed by the given connection pool.
func NewSettingsService(pool *pgxpool.Pool) *SettingsService {
	return &SettingsService{pool: pool, q: repository.New(pool)}
}

// Get returns the settings for userID.
func (s *SettingsService) Get(ctx context.Context, userID uuid.UUID) (api.UserSettings, error) {
	row, err := s.q.GetUserSettings(ctx, userID)
	if err != nil {
		return api.UserSettings{}, fmt.Errorf("get user settings: %w", err)
	}
	return toAPISettings(row), nil
}

// Update validates and saves new settings, then re-materializes all pending reminders
// for the user with the updated lead times. All mutations happen in one transaction.
func (s *SettingsService) Update(ctx context.Context, userID uuid.UUID, input api.UserSettingsInput) (api.UserSettings, error) {
	if err := validateSettings(input); err != nil {
		return api.UserSettings{}, err
	}

	leadDays := make([]int32, len(input.ReminderLeadDays))
	for i, d := range input.ReminderLeadDays {
		leadDays[i] = int32(d)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return api.UserSettings{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	row, err := qtx.UpdateUserSettings(ctx, repository.UpdateUserSettingsParams{
		UserID:                   userID,
		Timezone:                 input.Timezone,
		ReminderLeadDays:         leadDays,
		WeeklyDigestEnabled:      input.WeeklyDigestEnabled,
		WeeklyDigestDay:          int16(input.WeeklyDigestDay),
		WeeklyDigestHour:         int16(input.WeeklyDigestHour),
		WeeklyDigestHorizonWeeks: int16(input.WeeklyDigestHorizonWeeks),
	})
	if err != nil {
		return api.UserSettings{}, fmt.Errorf("update user settings: %w", err)
	}

	// Delete all pending reminders for this user and re-insert with new lead times.
	// We delete (not cancel) so that fresh rows with the same (conference, kind, lead_time)
	// tuple can be re-inserted without the UNIQUE constraint blocking them.
	starred, err := qtx.ListUserStarredConferences(ctx, userID)
	if err != nil {
		return api.UserSettings{}, fmt.Errorf("list starred: %w", err)
	}
	for _, conf := range starred {
		if err := qtx.DeleteUserConferencePendingReminders(ctx, repository.DeleteUserConferencePendingRemindersParams{
			UserID:       userID,
			ConferenceID: conf.ID,
		}); err != nil {
			return api.UserSettings{}, fmt.Errorf("delete pending reminders: %w", err)
		}
		deadlines := conferenceDeadlines(conf)
		if err := materializeReminders(ctx, qtx, userID, conf.ID,
			row.Timezone, row.ReminderLeadDays, deadlines); err != nil {
			return api.UserSettings{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return api.UserSettings{}, fmt.Errorf("commit: %w", err)
	}
	return toAPISettings(row), nil
}

func validateSettings(input api.UserSettingsInput) error {
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return &ValidationError{Field: "timezone", Message: "must be a valid IANA timezone name"}
	}
	if len(input.ReminderLeadDays) > 10 {
		return &ValidationError{Field: "reminder_lead_days", Message: "at most 10 entries allowed"}
	}
	for _, d := range input.ReminderLeadDays {
		if d < 0 || d > 365 {
			return &ValidationError{Field: "reminder_lead_days", Message: "each value must be between 0 and 365"}
		}
	}
	if input.WeeklyDigestDay < 0 || input.WeeklyDigestDay > 6 {
		return &ValidationError{Field: "weekly_digest_day", Message: "must be between 0 and 6"}
	}
	if input.WeeklyDigestHour < 0 || input.WeeklyDigestHour > 23 {
		return &ValidationError{Field: "weekly_digest_hour", Message: "must be between 0 and 23"}
	}
	if input.WeeklyDigestHorizonWeeks < 1 || input.WeeklyDigestHorizonWeeks > 52 {
		return &ValidationError{Field: "weekly_digest_horizon_weeks", Message: "must be between 1 and 52"}
	}
	return nil
}

func toAPISettings(row repository.UserSetting) api.UserSettings {
	leads := make([]int, len(row.ReminderLeadDays))
	for i, d := range row.ReminderLeadDays {
		leads[i] = int(d)
	}
	return api.UserSettings{
		Timezone:                 row.Timezone,
		ReminderLeadDays:         leads,
		WeeklyDigestEnabled:      row.WeeklyDigestEnabled,
		WeeklyDigestDay:          int(row.WeeklyDigestDay),
		WeeklyDigestHour:         int(row.WeeklyDigestHour),
		WeeklyDigestHorizonWeeks: int(row.WeeklyDigestHorizonWeeks),
	}
}
