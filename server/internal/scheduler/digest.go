package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"confero/internal/repository"
)

// scheduleDigests is the hourly probe that inserts digest_dispatch_log rows
// for users whose configured digest day+hour matches the current time in their
// timezone and who have not yet received a digest for the current week.
func (s *Scheduler) scheduleDigests(ctx context.Context) error {
	nowTime := s.cfg.Now()
	weekStart := mondayOfWeek(nowTime)
	weekStartDate := pgtype.Date{Time: weekStart, Valid: true}
	now := pgtype.Timestamptz{Time: nowTime, Valid: true}

	userIDs, err := s.q.SelectDigestDueUsers(ctx, repository.SelectDigestDueUsersParams{
		Now:          now,
		WeekStarting: weekStartDate,
	})
	if err != nil {
		return fmt.Errorf("select digest due users: %w", err)
	}

	for _, uid := range userIDs {
		if err := s.q.InsertDigestRow(ctx, repository.InsertDigestRowParams{
			UserID:       uid,
			WeekStarting: weekStartDate,
			ScheduledFor: now,
		}); err != nil {
			s.cfg.Logger.ErrorContext(ctx, "insert digest row", "user_id", uid, "err", err)
		}
	}
	return nil
}

// mondayOfWeek returns the UTC Monday of the week containing t, at midnight UTC.
func mondayOfWeek(t time.Time) time.Time {
	t = t.UTC()
	wd := int(t.Weekday()) // Sunday=0, Monday=1, ..., Saturday=6
	if wd == 0 {
		wd = 7 // treat Sunday as day 7 so Monday is day 1
	}
	offset := wd - 1 // days since Monday
	return time.Date(t.Year(), t.Month(), t.Day()-offset, 0, 0, 0, 0, time.UTC)
}
