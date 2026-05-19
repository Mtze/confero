// Package http wires the chi router, middleware, and handler implementations.
package http

import (
	"context"
	"log/slog"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"confero/internal/api"
	"confero/internal/auth"
)

// Server implements api.StrictServerInterface.
type Server struct {
	logger *slog.Logger
}

// NewServer constructs a Server.
func NewServer(logger *slog.Logger) *Server {
	return &Server{logger: logger}
}

// NewRouter builds and returns the fully-wired chi.Router.
// Pass nil for tm and oidcHandler to skip auth wiring (used in unit tests).
func NewRouter(s *Server, tm *auth.TokenManager, oidcHandler *auth.OIDCHandler) chi.Router {
	si := api.NewStrictHandler(s, nil)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public endpoints
	r.Get("/healthz", si.GetHealth)

	if oidcHandler != nil {
		r.Get("/auth/login", oidcHandler.Login)
		r.Get("/auth/callback", oidcHandler.Callback)
		r.Post("/auth/logout", oidcHandler.Logout)
	}

	r.Route("/api/v1", func(r chi.Router) {
		if tm != nil {
			r.Use(auth.RequireToken(tm))
		}
		r.Get("/me", si.GetMe)
	})

	return r
}

// GetHealth implements api.StrictServerInterface.
func (s *Server) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: api.Ok}, nil
}

// GetMe implements api.StrictServerInterface.
func (s *Server) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return unauth("authentication required"), nil
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return unauth("invalid subject claim"), nil
	}

	roles := make([]api.CurrentUserRoles, 0, len(claims.Roles))
	for _, r := range claims.Roles {
		roles = append(roles, api.CurrentUserRoles(r))
	}

	return api.GetMe200JSONResponse{
		Id:    id,
		Email: openapi_types.Email(claims.Email),
		Name:  claims.Name,
		Roles: roles,
	}, nil
}

var _ api.StrictServerInterface = (*Server)(nil)

func unauth(detail string) api.GetMe401ApplicationProblemPlusJSONResponse {
	return api.GetMe401ApplicationProblemPlusJSONResponse{
		UnauthorizedApplicationProblemPlusJSONResponse: api.UnauthorizedApplicationProblemPlusJSONResponse{
			Title:  ptr("Unauthorized"),
			Status: ptr(401),
			Detail: ptr(detail),
		},
	}
}

func ptr[T any](v T) *T { return &v }
