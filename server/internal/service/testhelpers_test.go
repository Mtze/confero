package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"confero/internal/repository"
)

// makeConferenceRow builds a minimal repository.Conference for unit tests.
func makeConferenceRow(primary, abstract, notification, cameraReady time.Time) repository.Conference {
	tsTZ := func(t time.Time) pgtype.Timestamptz {
		if t.IsZero() {
			return pgtype.Timestamptz{}
		}
		return pgtype.Timestamptz{Time: t, Valid: true}
	}
	return repository.Conference{
		ID:               uuid.New(),
		Name:             "Test Conference",
		Acronym:          "TC",
		Year:             2025,
		PrimaryDeadline:  tsTZ(primary),
		AbstractDeadline: tsTZ(abstract),
		NotificationDate: tsTZ(notification),
		CameraReadyDate:  tsTZ(cameraReady),
	}
}
