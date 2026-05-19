// Package audit provides HTTP middleware for writing audit log rows on 2xx writes.
package audit

import (
	"context"

	"github.com/google/uuid"
)

type contextKey int

const entityKey contextKey = iota

// entityHolder is a mutable cell stored by pointer in the context so that
// handlers can write the entity ID without replacing the context value.
type entityHolder struct {
	id  uuid.UUID
	set bool
}

// initContext injects a fresh entityHolder pointer into ctx.
// Called by the audit middleware before invoking the next handler.
func initContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, entityKey, &entityHolder{})
}

// MarkEntity records the entity ID for the current request.
// The write goes through the pointer injected by initContext, so it is visible
// to the middleware even though the handler's ctx is a derived copy.
func MarkEntity(ctx context.Context, id uuid.UUID) {
	if h, ok := ctx.Value(entityKey).(*entityHolder); ok {
		h.id = id
		h.set = true
	}
}

// entityFromContext retrieves the entity ID previously stored by MarkEntity.
func entityFromContext(ctx context.Context) (uuid.UUID, bool) {
	if h, ok := ctx.Value(entityKey).(*entityHolder); ok && h.set {
		return h.id, true
	}
	return uuid.UUID{}, false
}
