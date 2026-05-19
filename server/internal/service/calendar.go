package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"confero/internal/api"
	"confero/internal/repository"
)

const calendarTokenKind = "personal_starred"

// CalendarService manages calendar tokens.
type CalendarService struct {
	pool          *pgxpool.Pool
	q             *repository.Queries
	publicBaseURL string
}

// NewCalendarService creates a CalendarService.
func NewCalendarService(pool *pgxpool.Pool, publicBaseURL string) *CalendarService {
	return &CalendarService{pool: pool, q: repository.New(pool), publicBaseURL: publicBaseURL}
}

// List returns active calendar tokens for the user.
func (s *CalendarService) List(ctx context.Context, userID uuid.UUID) ([]api.CalendarToken, error) {
	rows, err := s.q.ListCalendarTokensByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list calendar tokens: %w", err)
	}
	out := make([]api.CalendarToken, len(rows))
	for i, r := range rows {
		out[i] = s.toAPI(r)
	}
	return out, nil
}

// Create revokes any existing token for the user+kind and issues a new one.
func (s *CalendarService) Create(ctx context.Context, userID uuid.UUID) (api.CalendarToken, error) {
	rawToken, err := generateToken()
	if err != nil {
		return api.CalendarToken{}, fmt.Errorf("generate token: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return api.CalendarToken{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	if err := qtx.RevokeCalendarTokensByUser(ctx, repository.RevokeCalendarTokensByUserParams{
		UserID: userID,
		Kind:   calendarTokenKind,
	}); err != nil {
		return api.CalendarToken{}, fmt.Errorf("revoke existing tokens: %w", err)
	}

	row, err := qtx.CreateCalendarToken(ctx, repository.CreateCalendarTokenParams{
		UserID: userID,
		Token:  rawToken,
		Kind:   calendarTokenKind,
	})
	if err != nil {
		return api.CalendarToken{}, fmt.Errorf("create calendar token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return api.CalendarToken{}, fmt.Errorf("commit: %w", err)
	}

	return s.toAPI(row), nil
}

// Delete revokes all active tokens for the user+kind.
func (s *CalendarService) Delete(ctx context.Context, userID uuid.UUID) error {
	tokens, err := s.q.ListCalendarTokensByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("list tokens: %w", err)
	}
	if len(tokens) == 0 {
		return ErrNotFound
	}
	return s.q.RevokeCalendarTokensByUser(ctx, repository.RevokeCalendarTokensByUserParams{
		UserID: userID,
		Kind:   calendarTokenKind,
	})
}

func (s *CalendarService) toAPI(r repository.UserCalendarToken) api.CalendarToken {
	t := api.CalendarToken{
		Id:      openapi_types.UUID(r.ID),
		Token:   r.Token,
		Kind:    api.PersonalStarred,
		FeedUrl: s.publicBaseURL + "/calendar/u/" + r.Token + ".ics",
	}
	if r.CreatedAt.Valid {
		t.CreatedAt = r.CreatedAt.Time.UTC()
	}
	if r.LastUsedAt.Valid {
		ts := r.LastUsedAt.Time.UTC()
		t.LastUsedAt = &ts
	}
	return t
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
