// Package service implements the business-logic layer between HTTP handlers and the repository.
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"confero/internal/api"
	"confero/internal/repository"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a uniqueness constraint would be violated.
var ErrConflict = errors.New("conflict")

// ErrValidation is returned when input fails business-rule validation.
var ErrValidation = errors.New("validation error")

// ValidationError carries the field name and reason for a single validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Is makes ValidationError satisfy errors.Is(err, ErrValidation).
func (e *ValidationError) Is(target error) bool {
	return target == ErrValidation
}

// ConferenceService handles conference business logic.
type ConferenceService struct {
	pool *pgxpool.Pool
	q    *repository.Queries
}

// NewConferenceService creates a ConferenceService backed by the given connection pool.
func NewConferenceService(pool *pgxpool.Pool) *ConferenceService {
	return &ConferenceService{pool: pool, q: repository.New(pool)}
}

// ListParams holds optional filters for listing conferences.
type ListParams struct {
	IncludeArchived bool
	TagSlug         *string
	TrackCode       *string
	Search          *string
}

// List returns all conferences matching the given filters, with tags and tracks embedded.
func (s *ConferenceService) List(ctx context.Context, p ListParams) ([]api.Conference, error) {
	var includeArchived *bool
	if p.IncludeArchived {
		t := true
		includeArchived = &t
	}

	rows, err := s.q.ListConferences(ctx, repository.ListConferencesParams{
		IncludeArchived: includeArchived,
		TagSlug:         p.TagSlug,
		TrackCode:       p.TrackCode,
		Search:          p.Search,
	})
	if err != nil {
		return nil, fmt.Errorf("list conferences: %w", err)
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

// Get returns a single conference by ID with tags and tracks embedded.
func (s *ConferenceService) Get(ctx context.Context, id uuid.UUID) (api.Conference, error) {
	row, err := s.q.GetConference(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.Conference{}, ErrNotFound
		}
		return api.Conference{}, fmt.Errorf("get conference: %w", err)
	}
	tags, err := s.q.GetConferenceTags(ctx, id)
	if err != nil {
		return api.Conference{}, fmt.Errorf("get tags: %w", err)
	}
	tracks, err := s.q.GetConferenceTracks(ctx, id)
	if err != nil {
		return api.Conference{}, fmt.Errorf("get tracks: %w", err)
	}
	return toAPIConference(row, tags, tracks), nil
}

// Create validates and inserts a new conference, then assigns tags and tracks in a transaction.
func (s *ConferenceService) Create(ctx context.Context, input api.ConferenceInput, actorID uuid.UUID) (api.Conference, error) {
	if err := validateInput(input); err != nil {
		return api.Conference{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return api.Conference{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	actor := pgtype.UUID{Bytes: actorID, Valid: true}
	row, err := qtx.CreateConference(ctx, repository.CreateConferenceParams{
		Name:              input.Name,
		Acronym:           input.Acronym,
		Year:              int32(input.Year),
		Location:          input.Location,
		WebsiteUrl:        input.WebsiteUrl,
		CfpUrl:            input.CfpUrl,
		PrimaryDeadline:   timePtrToTimestamptz(input.PrimaryDeadline),
		AbstractDeadline:  timePtrToTimestamptz(input.AbstractDeadline),
		NotificationDate:  timePtrToTimestamptz(input.NotificationDate),
		CameraReadyDate:   timePtrToTimestamptz(input.CameraReadyDate),
		EventStartDate:    datePtrToDate(input.EventStartDate),
		EventEndDate:      datePtrToDate(input.EventEndDate),
		CoreRank:          coreRankToPtr(input.CoreRank),
		H5Index:           intPtrToInt32Ptr(input.H5Index),
		AcceptanceRatePct: float32PtrToNumeric(input.AcceptanceRatePct),
		DblpKey:           input.DblpKey,
		Notes:             input.Notes,
		CreatedBy:         actor,
		UpdatedBy:         actor,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return api.Conference{}, fmt.Errorf("%w: a conference with this acronym and year already exists", ErrConflict)
		}
		return api.Conference{}, fmt.Errorf("create conference: %w", err)
	}

	tags, tracks, err := setTagsAndTracks(ctx, qtx, row.ID, input.TagSlugs, input.TrackCodes)
	if err != nil {
		return api.Conference{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return api.Conference{}, fmt.Errorf("commit: %w", err)
	}
	return toAPIConference(row, tags, tracks), nil
}

// Update validates and replaces all editable fields of a conference in a transaction.
func (s *ConferenceService) Update(ctx context.Context, id uuid.UUID, input api.ConferenceInput, actorID uuid.UUID) (api.Conference, error) {
	if err := validateInput(input); err != nil {
		return api.Conference{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return api.Conference{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	actor := pgtype.UUID{Bytes: actorID, Valid: true}
	row, err := qtx.UpdateConference(ctx, repository.UpdateConferenceParams{
		ID:                id,
		Name:              input.Name,
		Acronym:           input.Acronym,
		Year:              int32(input.Year),
		Location:          input.Location,
		WebsiteUrl:        input.WebsiteUrl,
		CfpUrl:            input.CfpUrl,
		PrimaryDeadline:   timePtrToTimestamptz(input.PrimaryDeadline),
		AbstractDeadline:  timePtrToTimestamptz(input.AbstractDeadline),
		NotificationDate:  timePtrToTimestamptz(input.NotificationDate),
		CameraReadyDate:   timePtrToTimestamptz(input.CameraReadyDate),
		EventStartDate:    datePtrToDate(input.EventStartDate),
		EventEndDate:      datePtrToDate(input.EventEndDate),
		CoreRank:          coreRankToPtr(input.CoreRank),
		H5Index:           intPtrToInt32Ptr(input.H5Index),
		AcceptanceRatePct: float32PtrToNumeric(input.AcceptanceRatePct),
		DblpKey:           input.DblpKey,
		Notes:             input.Notes,
		UpdatedBy:         actor,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.Conference{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return api.Conference{}, fmt.Errorf("%w: a conference with this acronym and year already exists", ErrConflict)
		}
		return api.Conference{}, fmt.Errorf("update conference: %w", err)
	}

	tags, tracks, err := setTagsAndTracks(ctx, qtx, row.ID, input.TagSlugs, input.TrackCodes)
	if err != nil {
		return api.Conference{}, err
	}

	// Re-materialize pending reminders for all users who starred this conference,
	// since any deadline may have changed.
	if err := rematerializeConferenceReminders(ctx, qtx, row); err != nil {
		return api.Conference{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return api.Conference{}, fmt.Errorf("commit: %w", err)
	}
	return toAPIConference(row, tags, tracks), nil
}

// Delete permanently removes a conference. Returns ErrNotFound if no row matched.
func (s *ConferenceService) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteConference(ctx, id)
	if err != nil {
		return fmt.Errorf("delete conference: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Archive sets archived_at on a conference and cancels all pending reminders.
func (s *ConferenceService) Archive(ctx context.Context, id uuid.UUID) (api.Conference, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return api.Conference{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)

	row, err := qtx.ArchiveConference(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.Conference{}, ErrNotFound
		}
		return api.Conference{}, fmt.Errorf("archive conference: %w", err)
	}

	if err := qtx.CancelConferenceReminders(ctx, id); err != nil {
		return api.Conference{}, fmt.Errorf("cancel reminders: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return api.Conference{}, fmt.Errorf("commit: %w", err)
	}
	return s.attachTagsAndTracks(ctx, row)
}

// Unarchive clears archived_at on a conference (idempotent).
func (s *ConferenceService) Unarchive(ctx context.Context, id uuid.UUID) (api.Conference, error) {
	row, err := s.q.UnarchiveConference(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.Conference{}, ErrNotFound
		}
		return api.Conference{}, fmt.Errorf("unarchive conference: %w", err)
	}
	return s.attachTagsAndTracks(ctx, row)
}

// ImportInput holds the data parsed from a YAML import.
type ImportInput struct {
	Name              string
	Acronym           string
	Year              int32
	Location          string
	WebsiteURL        *string
	CFPURL            *string
	PrimaryDeadline   *time.Time
	AbstractDeadline  *time.Time
	NotificationDate  *time.Time
	CameraReadyDate   *time.Time
	EventStartDate    *openapi_types.Date
	EventEndDate      *openapi_types.Date
	CoreRank          *string
	H5Index           *int32
	AcceptanceRatePct *float32
	DblpKey           *string
	Notes             *string
	Tags              []string
	Tracks            []string
}

// UpsertFromImport creates or updates a conference based on the import input.
// Returns (true, nil) for a create, (false, nil) for an update.
func (s *ConferenceService) UpsertFromImport(ctx context.Context, in ImportInput, actorID uuid.UUID) (bool, error) {
	existing, lookupErr := s.q.GetConferenceByAcronymYear(ctx, repository.GetConferenceByAcronymYearParams{
		Acronym: in.Acronym,
		Year:    in.Year,
	})

	tx, txErr := s.pool.Begin(ctx)
	if txErr != nil {
		return false, fmt.Errorf("begin tx: %w", txErr)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := repository.New(tx)
	actor := pgtype.UUID{Bytes: actorID, Valid: true}

	if lookupErr != nil {
		// Not found — create.
		row, createErr := qtx.CreateConference(ctx, repository.CreateConferenceParams{
			Name:              in.Name,
			Acronym:           in.Acronym,
			Year:              in.Year,
			Location:          in.Location,
			WebsiteUrl:        in.WebsiteURL,
			CfpUrl:            in.CFPURL,
			PrimaryDeadline:   timePtrToTimestamptz(in.PrimaryDeadline),
			AbstractDeadline:  timePtrToTimestamptz(in.AbstractDeadline),
			NotificationDate:  timePtrToTimestamptz(in.NotificationDate),
			CameraReadyDate:   timePtrToTimestamptz(in.CameraReadyDate),
			EventStartDate:    datePtrToDate(in.EventStartDate),
			EventEndDate:      datePtrToDate(in.EventEndDate),
			CoreRank:          in.CoreRank,
			H5Index:           in.H5Index,
			AcceptanceRatePct: float32PtrToNumeric(in.AcceptanceRatePct),
			DblpKey:           in.DblpKey,
			Notes:             in.Notes,
			CreatedBy:         actor,
			UpdatedBy:         actor,
		})
		if createErr != nil {
			return false, fmt.Errorf("create conference: %w", createErr)
		}
		if _, _, err2 := setTagsAndTracks(ctx, qtx, row.ID, ptrSlice(in.Tags), ptrSlice(in.Tracks)); err2 != nil {
			return false, err2
		}
		if err2 := tx.Commit(ctx); err2 != nil {
			return false, fmt.Errorf("commit: %w", err2)
		}
		return true, nil
	}

	// Found — update.
	row, updateErr := qtx.UpdateConference(ctx, repository.UpdateConferenceParams{
		ID:                existing.ID,
		Name:              in.Name,
		Acronym:           in.Acronym,
		Year:              in.Year,
		Location:          in.Location,
		WebsiteUrl:        in.WebsiteURL,
		CfpUrl:            in.CFPURL,
		PrimaryDeadline:   timePtrToTimestamptz(in.PrimaryDeadline),
		AbstractDeadline:  timePtrToTimestamptz(in.AbstractDeadline),
		NotificationDate:  timePtrToTimestamptz(in.NotificationDate),
		CameraReadyDate:   timePtrToTimestamptz(in.CameraReadyDate),
		EventStartDate:    datePtrToDate(in.EventStartDate),
		EventEndDate:      datePtrToDate(in.EventEndDate),
		CoreRank:          in.CoreRank,
		H5Index:           in.H5Index,
		AcceptanceRatePct: float32PtrToNumeric(in.AcceptanceRatePct),
		DblpKey:           in.DblpKey,
		Notes:             in.Notes,
		UpdatedBy:         actor,
	})
	if updateErr != nil {
		return false, fmt.Errorf("update conference: %w", updateErr)
	}
	if _, _, err2 := setTagsAndTracks(ctx, qtx, row.ID, ptrSlice(in.Tags), ptrSlice(in.Tracks)); err2 != nil {
		return false, err2
	}
	if err2 := rematerializeConferenceReminders(ctx, qtx, row); err2 != nil {
		return false, fmt.Errorf("rematerialize: %w", err2)
	}
	if err2 := tx.Commit(ctx); err2 != nil {
		return false, fmt.Errorf("commit: %w", err2)
	}
	return false, nil
}

func ptrSlice(s []string) *[]string {
	if len(s) == 0 {
		return nil
	}
	return &s
}

// ListTags returns all tags sorted by slug.
func (s *ConferenceService) ListTags(ctx context.Context) ([]api.Tag, error) {
	rows, err := s.q.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	tags := make([]api.Tag, len(rows))
	for i, t := range rows {
		tags[i] = api.Tag{Id: t.ID, Slug: t.Slug, Name: t.Name}
	}
	return tags, nil
}

// ListTracks returns all tracks sorted by sort_order.
func (s *ConferenceService) ListTracks(ctx context.Context) ([]api.Track, error) {
	rows, err := s.q.ListTracks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	tracks := make([]api.Track, len(rows))
	for i, t := range rows {
		tracks[i] = api.Track{Code: t.Code, DisplayName: t.DisplayName, SortOrder: int(t.SortOrder)}
	}
	return tracks, nil
}

// ------- helpers -------

func (s *ConferenceService) attachTagsAndTracks(ctx context.Context, row repository.Conference) (api.Conference, error) {
	tags, err := s.q.GetConferenceTags(ctx, row.ID)
	if err != nil {
		return api.Conference{}, fmt.Errorf("get tags: %w", err)
	}
	tracks, err := s.q.GetConferenceTracks(ctx, row.ID)
	if err != nil {
		return api.Conference{}, fmt.Errorf("get tracks: %w", err)
	}
	return toAPIConference(row, tags, tracks), nil
}

// setTagsAndTracks replaces all tags and tracks for a conference within an existing transaction.
func setTagsAndTracks(
	ctx context.Context,
	qtx *repository.Queries,
	confID uuid.UUID,
	tagSlugs *[]string,
	trackCodes *[]string,
) ([]repository.GetConferenceTagsRow, []repository.GetConferenceTracksRow, error) {
	if err := qtx.DeleteAllConferenceTags(ctx, confID); err != nil {
		return nil, nil, fmt.Errorf("clear tags: %w", err)
	}
	if tagSlugs != nil {
		for _, slug := range *tagSlugs {
			slug = Slugify(slug)
			tag, err := qtx.UpsertTagBySlug(ctx, repository.UpsertTagBySlugParams{
				Slug: slug,
				Name: slug,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("upsert tag %q: %w", slug, err)
			}
			if err := qtx.AddConferenceTag(ctx, repository.AddConferenceTagParams{
				ConferenceID: confID,
				TagID:        tag.ID,
			}); err != nil {
				return nil, nil, fmt.Errorf("add tag %q: %w", slug, err)
			}
		}
	}

	if err := qtx.DeleteAllConferenceTracks(ctx, confID); err != nil {
		return nil, nil, fmt.Errorf("clear tracks: %w", err)
	}
	if trackCodes != nil {
		for _, code := range *trackCodes {
			if err := qtx.AddConferenceTrack(ctx, repository.AddConferenceTrackParams{
				ConferenceID: confID,
				TrackCode:    code,
			}); err != nil {
				return nil, nil, fmt.Errorf("add track %q: %w", code, err)
			}
		}
	}

	tags, err := qtx.GetConferenceTags(ctx, confID)
	if err != nil {
		return nil, nil, fmt.Errorf("get tags after set: %w", err)
	}
	tracks, err := qtx.GetConferenceTracks(ctx, confID)
	if err != nil {
		return nil, nil, fmt.Errorf("get tracks after set: %w", err)
	}
	return tags, tracks, nil
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string to a URL-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func validateInput(input api.ConferenceInput) error {
	if input.Year < 2000 || input.Year > 2100 {
		return &ValidationError{Field: "year", Message: "must be between 2000 and 2100"}
	}
	if strings.TrimSpace(input.Name) == "" {
		return &ValidationError{Field: "name", Message: "must not be blank"}
	}
	if strings.TrimSpace(input.Acronym) == "" {
		return &ValidationError{Field: "acronym", Message: "must not be blank"}
	}
	if strings.TrimSpace(input.Location) == "" {
		return &ValidationError{Field: "location", Message: "must not be blank"}
	}
	if input.AbstractDeadline != nil && input.PrimaryDeadline != nil {
		if input.AbstractDeadline.After(*input.PrimaryDeadline) {
			return &ValidationError{Field: "abstract_deadline", Message: "must be on or before primary_deadline"}
		}
	}
	if input.PrimaryDeadline != nil && input.NotificationDate != nil {
		if input.PrimaryDeadline.After(*input.NotificationDate) {
			return &ValidationError{Field: "primary_deadline", Message: "must be on or before notification_date"}
		}
	}
	if input.NotificationDate != nil && input.CameraReadyDate != nil {
		if input.NotificationDate.After(*input.CameraReadyDate) {
			return &ValidationError{Field: "notification_date", Message: "must be on or before camera_ready_date"}
		}
	}
	if input.EventStartDate != nil && input.EventEndDate != nil {
		if input.EventStartDate.After(input.EventEndDate.Time) {
			return &ValidationError{Field: "event_start_date", Message: "must be on or before event_end_date"}
		}
	}
	if input.AcceptanceRatePct != nil && (*input.AcceptanceRatePct < 0 || *input.AcceptanceRatePct > 100) {
		return &ValidationError{Field: "acceptance_rate_pct", Message: "must be between 0 and 100"}
	}
	if input.H5Index != nil && *input.H5Index < 0 {
		return &ValidationError{Field: "h5_index", Message: "must be non-negative"}
	}
	return nil
}

// toAPIConference maps repository types to the API Conference struct.
func toAPIConference(
	c repository.Conference,
	tags []repository.GetConferenceTagsRow,
	tracks []repository.GetConferenceTracksRow,
) api.Conference {
	apiTags := make([]api.Tag, len(tags))
	for i, t := range tags {
		apiTags[i] = api.Tag{Id: t.ID, Slug: t.Slug, Name: t.Name}
	}
	apiTracks := make([]api.Track, len(tracks))
	for i, t := range tracks {
		apiTracks[i] = api.Track{Code: t.Code, DisplayName: t.DisplayName, SortOrder: int(t.SortOrder)}
	}

	conf := api.Conference{
		Id:                c.ID,
		Name:              c.Name,
		Acronym:           c.Acronym,
		Year:              int(c.Year),
		Location:          c.Location,
		WebsiteUrl:        c.WebsiteUrl,
		CfpUrl:            c.CfpUrl,
		PrimaryDeadline:   timestamptzToTimePtr(c.PrimaryDeadline),
		AbstractDeadline:  timestamptzToTimePtr(c.AbstractDeadline),
		NotificationDate:  timestamptzToTimePtr(c.NotificationDate),
		CameraReadyDate:   timestamptzToTimePtr(c.CameraReadyDate),
		EventStartDate:    dateToDatePtr(c.EventStartDate),
		EventEndDate:      dateToDatePtr(c.EventEndDate),
		DblpKey:           c.DblpKey,
		Notes:             c.Notes,
		ArchivedAt:        timestamptzToTimePtr(c.ArchivedAt),
		CreatedAt:         c.CreatedAt.Time,
		UpdatedAt:         c.UpdatedAt.Time,
		Tags:              apiTags,
		Tracks:            apiTracks,
	}

	if c.H5Index != nil {
		v := int(*c.H5Index)
		conf.H5Index = &v
	}
	if c.CoreRank != nil {
		v := api.ConferenceCoreRank(*c.CoreRank)
		conf.CoreRank = &v
	}
	if c.AcceptanceRatePct.Valid {
		f64, err := c.AcceptanceRatePct.Float64Value()
		if err == nil && f64.Valid {
			v := float32(f64.Float64)
			conf.AcceptanceRatePct = &v
		}
	}
	if c.CreatedBy.Valid {
		v := openapi_types.UUID(c.CreatedBy.Bytes)
		conf.CreatedBy = &v
	}
	if c.UpdatedBy.Valid {
		v := openapi_types.UUID(c.UpdatedBy.Bytes)
		conf.UpdatedBy = &v
	}
	return conf
}

func timestamptzToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func dateToDatePtr(d pgtype.Date) *openapi_types.Date {
	if !d.Valid {
		return nil
	}
	v := openapi_types.Date{Time: d.Time}
	return &v
}

func timePtrToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func datePtrToDate(d *openapi_types.Date) pgtype.Date {
	if d == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: d.Time, Valid: true}
}

func intPtrToInt32Ptr(i *int) *int32 {
	if i == nil {
		return nil
	}
	v := int32(*i)
	return &v
}

func coreRankToPtr(r *api.ConferenceInputCoreRank) *string {
	if r == nil {
		return nil
	}
	s := string(*r)
	return &s
}

func float32PtrToNumeric(f *float32) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{}
	}
	n := pgtype.Numeric{}
	_ = n.Scan(strconv.FormatFloat(float64(*f), 'f', 2, 32))
	return n
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unique")
}

// rematerializeConferenceReminders deletes all pending reminders for this conference
// (per DATA_MODEL.md §3.8 deadline-edit rule) and re-inserts fresh rows for every
// user who starred it, using their current settings.
func rematerializeConferenceReminders(ctx context.Context, qtx *repository.Queries, conf repository.Conference) error {
	if err := qtx.DeleteConferencePendingReminders(ctx, conf.ID); err != nil {
		return fmt.Errorf("delete conference pending reminders: %w", err)
	}
	users, err := qtx.ListUsersStarringConferenceWithSettings(ctx, conf.ID)
	if err != nil {
		return fmt.Errorf("list starring users: %w", err)
	}
	deadlines := conferenceDeadlines(conf)
	for _, u := range users {
		if err := materializeReminders(ctx, qtx, u.UserID, conf.ID,
			u.Timezone, u.ReminderLeadDays, deadlines); err != nil {
			return err
		}
	}
	return nil
}
