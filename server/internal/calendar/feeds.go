// Package calendar assembles RFC 5545 iCalendar feeds from Confero data.
package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"confero/internal/ical"
	"confero/internal/repository"
)

const prodID = "-//Confero//Conference Tracker//EN"

// Builder assembles ICS feeds.
type Builder struct {
	q    *repository.Queries
	pool *pgxpool.Pool
}

// NewBuilder creates a Builder backed by a connection pool.
func NewBuilder(pool *pgxpool.Pool) *Builder {
	return &Builder{q: repository.New(pool), pool: pool}
}

// ErrTokenNotFound is returned by LookupToken when the token is unknown or revoked.
var ErrTokenNotFound = fmt.Errorf("calendar token not found")

// LookupToken resolves a token string to the owning user ID, updating last_used_at.
func (b *Builder) LookupToken(ctx context.Context, token string) (uuid.UUID, error) {
	row, err := b.q.GetCalendarTokenByValue(ctx, token)
	if err != nil {
		return uuid.Nil, ErrTokenNotFound
	}
	return row.UserID, nil
}

// BuildPublicFeed returns the ICS for all non-archived conferences.
func (b *Builder) BuildPublicFeed(ctx context.Context) ([]byte, error) {
	confs, err := b.q.ListConferences(ctx, repository.ListConferencesParams{})
	if err != nil {
		return nil, fmt.Errorf("list conferences: %w", err)
	}
	cal := &ical.Calendar{ProdID: prodID, Version: "2.0"}
	for _, c := range confs {
		cal.Events = append(cal.Events, eventsForConference(c)...)
	}
	return cal.Encode(), nil
}

// BuildPersonalFeed returns the ICS for conferences starred by userID.
func (b *Builder) BuildPersonalFeed(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	confs, err := b.q.ListUserStarredConferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list starred conferences: %w", err)
	}
	cal := &ical.Calendar{ProdID: prodID, Version: "2.0"}
	for _, c := range confs {
		cal.Events = append(cal.Events, eventsForConference(c)...)
	}
	return cal.Encode(), nil
}

// eventsForConference produces up to 5 VEVENTs per conference per ARCHITECTURE §7.2.
func eventsForConference(c repository.Conference) []ical.Event {
	var events []ical.Event

	// Conference dates (all-day)
	if c.EventStartDate.Valid && c.EventEndDate.Valid {
		start := c.EventStartDate.Time
		end := c.EventEndDate.Time.AddDate(0, 0, 1) // DTEND is exclusive in iCal
		events = append(events, ical.Event{
			UID:      uid(c.ID, "dates"),
			Summary:  c.Name,
			Location: c.Location,
			URL:      derefStr(c.WebsiteUrl),
			DTStart:  start,
			DTEnd:    end,
			AllDay:   true,
		})
	}

	// Deadlines (point-in-time, 15-minute duration)
	if c.PrimaryDeadline.Valid {
		events = append(events, deadlineEvent(c.ID, c.Name, "Submission Deadline", "submission", c.PrimaryDeadline))
	}
	if c.AbstractDeadline.Valid {
		events = append(events, deadlineEvent(c.ID, c.Name, "Abstract Deadline", "abstract", c.AbstractDeadline))
	}
	if c.NotificationDate.Valid {
		events = append(events, deadlineEvent(c.ID, c.Name, "Author Notification", "notification", c.NotificationDate))
	}
	if c.CameraReadyDate.Valid {
		events = append(events, deadlineEvent(c.ID, c.Name, "Camera-Ready Deadline", "camera-ready", c.CameraReadyDate))
	}

	return events
}

func deadlineEvent(confID uuid.UUID, confName, label, kind string, ts pgtype.Timestamptz) ical.Event {
	end := ts.Time.Add(15 * time.Minute)
	return ical.Event{
		UID:     uid(confID, kind),
		Summary: fmt.Sprintf("%s: %s", confName, label),
		DTStart: ts.Time,
		DTEnd:   end,
	}
}

func uid(confID uuid.UUID, kind string) string {
	return fmt.Sprintf("%s:%s@confero", confID, kind)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
