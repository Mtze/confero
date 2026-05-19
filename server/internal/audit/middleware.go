package audit

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"confero/internal/auth"
	"confero/internal/repository"
)

var writeFailures = promauto.NewCounter(prometheus.CounterOpts{
	Name: "confero_audit_write_failures_total",
	Help: "Number of audit log write failures.",
})

// responseRecorder wraps http.ResponseWriter and captures the status code.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) status200() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

// For returns a middleware that writes one audit row when the wrapped handler
// responds with a 2xx status code. entityType and action are fixed per route.
// The handler must call MarkEntity(ctx, id) when it knows the entity ID.
func For(entityType, action string, q *repository.Queries, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(initContext(r.Context()))
			rr := &responseRecorder{ResponseWriter: w}
			next.ServeHTTP(rr, r)

			status := rr.status200()
			if status < 200 || status >= 300 {
				return
			}

			ctx := r.Context()
			entityID, ok := entityFromContext(ctx)
			if !ok {
				return
			}

			claims, hasClaims := auth.ClaimsFromContext(ctx)
			if !hasClaims {
				return
			}

			actorUID, _ := uuid.Parse(claims.Subject)
			actorPG := pgtype.UUID{Bytes: actorUID, Valid: actorUID != uuid.Nil}

			if err := q.InsertAuditEntry(ctx, repository.InsertAuditEntryParams{
				ActorUserID:      actorPG,
				ActorDisplayName: claims.Name,
				ActorOidcSubject: claims.OIDCSub,
				Action:           action,
				EntityType:       entityType,
				EntityID:         entityID,
			}); err != nil {
				writeFailures.Inc()
				logger.ErrorContext(ctx, "audit write failed", "entity_type", entityType, "action", action, "err", err)
			}
		})
	}
}
