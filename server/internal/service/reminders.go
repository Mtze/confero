package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"confero/internal/repository"
)

// deadlineEntry pairs a deadline kind with its timestamp.
type deadlineEntry struct {
	kind string
	at   time.Time
}

// conferenceDeadlines extracts non-nil deadlines from a conference row.
func conferenceDeadlines(c repository.Conference) []deadlineEntry {
	var out []deadlineEntry
	add := func(kind string, ts pgtype.Timestamptz) {
		if ts.Valid {
			out = append(out, deadlineEntry{kind: kind, at: ts.Time})
		}
	}
	add("submission", c.PrimaryDeadline)
	add("abstract", c.AbstractDeadline)
	add("notification", c.NotificationDate)
	add("camera_ready", c.CameraReadyDate)
	return out
}

// materializeReminders inserts reminder rows for a single (user, conference) pair.
// Uses ON CONFLICT DO NOTHING so calling it twice is safe.
func materializeReminders(
	ctx context.Context,
	qtx *repository.Queries,
	userID uuid.UUID,
	conferenceID uuid.UUID,
	tz string,
	leadDays []int32,
	deadlines []deadlineEntry,
) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	for _, d := range deadlines {
		for _, lead := range leadDays {
			sendAt := scheduledFor(d.at, int(lead), loc)
			if err := qtx.InsertReminderRow(ctx, repository.InsertReminderRowParams{
				UserID:       userID,
				ConferenceID: conferenceID,
				DeadlineKind: d.kind,
				LeadTimeDays: lead,
				ScheduledFor: pgtype.Timestamptz{Time: sendAt, Valid: true},
			}); err != nil {
				return fmt.Errorf("insert reminder %s lead=%d: %w", d.kind, lead, err)
			}
		}
	}
	return nil
}

// scheduledFor computes the send time: deadline minus lead days at 09:00 in the user's timezone.
func scheduledFor(deadline time.Time, leadDays int, loc *time.Location) time.Time {
	d := deadline.In(loc).AddDate(0, 0, -leadDays)
	return time.Date(d.Year(), d.Month(), d.Day(), 9, 0, 0, 0, loc).UTC()
}
