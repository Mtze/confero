package audit

import (
	"context"

	"github.com/google/uuid"
)

// ExportInitContext exposes initContext for whitebox tests.
func ExportInitContext(ctx context.Context) context.Context {
	return initContext(ctx)
}

// ExportEntityFromContext exposes entityFromContext for whitebox tests.
func ExportEntityFromContext(ctx context.Context) (uuid.UUID, bool) {
	return entityFromContext(ctx)
}
