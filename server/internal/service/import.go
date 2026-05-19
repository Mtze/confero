package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"confero/internal/importer"
)

// RunImport executes an import, calling UpsertFromImport for each entry.
// In strict mode, the first validation or DB error aborts the whole import.
func (s *ConferenceService) RunImport(ctx context.Context, entries []importer.ConferenceInput, actorID uuid.UUID, strict bool) (created, updated, skipped int, errs []string) {
	for i, e := range entries {
		if msg := e.Validate(); msg != "" {
			errs = append(errs, fmt.Sprintf("entry %d (%s): %s", i+1, e.Acronym, msg))
			if strict {
				return
			}
			skipped++
			continue
		}

		in := importInputFromYAML(e)
		was_created, err := s.UpsertFromImport(ctx, in, actorID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("entry %d (%s): %s", i+1, e.Acronym, err.Error()))
			if strict {
				return
			}
			skipped++
			continue
		}
		if was_created {
			created++
		} else {
			updated++
		}
	}
	return
}

func importInputFromYAML(e importer.ConferenceInput) ImportInput {
	in := ImportInput{
		Name:     e.Name,
		Acronym:  e.Acronym,
		Year:     e.Year,
		Location: e.Location,
		Tags:     e.Tags,
		Tracks:   e.Tracks,
	}
	if e.WebsiteURL != "" {
		in.WebsiteURL = &e.WebsiteURL
	}
	if e.CFPURL != "" {
		in.CFPURL = &e.CFPURL
	}
	if e.DblpKey != "" {
		in.DblpKey = &e.DblpKey
	}
	if e.Notes != "" {
		in.Notes = &e.Notes
	}
	if e.CoreRank != "" {
		in.CoreRank = &e.CoreRank
	}
	if e.H5Index > 0 {
		v := e.H5Index
		in.H5Index = &v
	}
	if e.AcceptanceRatePct > 0 {
		v := float32(e.AcceptanceRatePct)
		in.AcceptanceRatePct = &v
	}
	in.PrimaryDeadline = parseTimestamp(e.PrimaryDeadline)
	in.AbstractDeadline = parseTimestamp(e.AbstractDeadline)
	in.NotificationDate = parseTimestamp(e.NotificationDate)
	in.CameraReadyDate = parseTimestamp(e.CameraReadyDate)
	in.EventStartDate = parseDate(e.EventStartDate)
	in.EventEndDate = parseDate(e.EventEndDate)
	return in
}

func parseTimestamp(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func parseDate(s string) *openapi_types.Date {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	d := openapi_types.Date{Time: t}
	return &d
}
