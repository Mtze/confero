package audit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"confero/internal/audit"
)

// TestMarkEntity_VisibleAfterForInit verifies that MarkEntity writes through
// the pointer that audit.For injects, so the middleware reads the correct ID
// even though the handler only has a derived copy of the context.
func TestMarkEntity_VisibleAfterForInit(t *testing.T) {
	id := uuid.New()
	var capturedID uuid.UUID
	var capturedOK bool

	// Simulate what audit.For does: inject holder, call next, read holder back.
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Middleware injects holder before calling next.
		r2 := r.WithContext(audit.ExportInitContext(r.Context()))

		// Handler receives r2.Context() (or a child of it) and calls MarkEntity.
		handlerCtx := r2.Context()
		audit.MarkEntity(handlerCtx, id)

		// Middleware reads from r2.Context() AFTER handler ran.
		capturedID, capturedOK = audit.ExportEntityFromContext(r2.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, capturedOK)
	require.Equal(t, id, capturedID)
}

func TestMarkEntity_NoOpWhenNoHolder(t *testing.T) {
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	require.NotPanics(t, func() {
		audit.MarkEntity(ctx, uuid.New())
	})
}
