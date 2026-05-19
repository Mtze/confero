// Package http wires the chi router, middleware, and handler implementations.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"confero/internal/api"
	"confero/internal/auth"
	"confero/internal/service"
)

// Server implements api.StrictServerInterface.
type Server struct {
	logger      *slog.Logger
	confSvc     *service.ConferenceService
	starSvc     *service.StarService
	settingsSvc *service.SettingsService
}

// NewServer constructs a Server.
func NewServer(logger *slog.Logger, confSvc *service.ConferenceService, starSvc *service.StarService, settingsSvc *service.SettingsService) *Server {
	return &Server{logger: logger, confSvc: confSvc, starSvc: starSvc, settingsSvc: settingsSvc}
}

// NewRouter builds and returns the fully-wired chi.Router.
// Pass nil for tm and oidcHandler to skip auth wiring (used in unit tests).
func NewRouter(s *Server, tm *auth.TokenManager, oidcHandler *auth.OIDCHandler) chi.Router {
	si := api.NewStrictHandler(s, nil)
	w := &api.ServerInterfaceWrapper{
		Handler: si,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public endpoints — no authentication required.
	r.Group(func(r chi.Router) {
		r.Get("/healthz", si.GetHealth)
		r.Get("/api/v1/conferences", w.ListConferences)
		r.Get("/api/v1/conferences/{id}", w.GetConference)
	})

	if oidcHandler != nil {
		r.Get("/auth/login", oidcHandler.Login)
		r.Get("/auth/callback", oidcHandler.Callback)
		r.Post("/auth/logout", oidcHandler.Logout)
	}

	// Token-gated endpoints.
	r.Group(func(r chi.Router) {
		if tm != nil {
			r.Use(auth.RequireToken(tm))
		}

		r.Get("/api/v1/me", si.GetMe)
		r.Get("/api/v1/tags", si.ListTags)
		r.Get("/api/v1/tracks", si.ListTracks)

		// Member-only write operations.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireMember)
			r.Post("/api/v1/conferences", w.CreateConference)
			r.Put("/api/v1/conferences/{id}", w.UpdateConference)
			r.Post("/api/v1/conferences/{id}/archive", w.ArchiveConference)
			r.Post("/api/v1/conferences/{id}/unarchive", w.UnarchiveConference)
			r.Post("/api/v1/conferences/{id}/stars", w.StarConference)
			r.Delete("/api/v1/conferences/{id}/stars", w.UnstarConference)
			r.Get("/api/v1/me/stars", si.ListMyStars)
			r.Get("/api/v1/me/settings", si.GetMySettings)
			r.Put("/api/v1/me/settings", si.UpdateMySettings)
		})

		// Admin-only operations.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Delete("/api/v1/conferences/{id}", w.DeleteConference)
		})
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
		return api.GetMe401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized("authentication required"),
		}, nil
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return api.GetMe401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized("invalid subject claim"),
		}, nil
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

// ListConferences implements api.StrictServerInterface.
func (s *Server) ListConferences(ctx context.Context, req api.ListConferencesRequestObject) (api.ListConferencesResponseObject, error) {
	p := service.ListParams{
		TagSlug:   req.Params.Tag,
		TrackCode: req.Params.Track,
		Search:    req.Params.Q,
	}
	if req.Params.Archived != nil {
		p.IncludeArchived = *req.Params.Archived
	}

	confs, err := s.confSvc.List(ctx, p)
	if err != nil {
		s.logger.ErrorContext(ctx, "list conferences", "error", err)
		return nil, err
	}
	return api.ListConferences200JSONResponse(confs), nil
}

// GetConference implements api.StrictServerInterface.
func (s *Server) GetConference(ctx context.Context, req api.GetConferenceRequestObject) (api.GetConferenceResponseObject, error) {
	conf, err := s.confSvc.Get(ctx, uuid.UUID(req.Id))
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.GetConference404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: notFound(),
			}, nil
		}
		s.logger.ErrorContext(ctx, "get conference", "id", req.Id, "error", err)
		return nil, err
	}
	return api.GetConference200JSONResponse(conf), nil
}

// CreateConference implements api.StrictServerInterface.
func (s *Server) CreateConference(ctx context.Context, req api.CreateConferenceRequestObject) (api.CreateConferenceResponseObject, error) {
	actorID, err := actorFromContext(ctx)
	if err != nil {
		return api.CreateConference401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized(err.Error()),
		}, nil
	}

	conf, err := s.confSvc.Create(ctx, *req.Body, actorID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrValidation):
			return api.CreateConference400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: badRequest(err.Error()),
			}, nil
		case errors.Is(err, service.ErrConflict):
			return api.CreateConference409ApplicationProblemPlusJSONResponse{
				ConflictApplicationProblemPlusJSONResponse: conflict(err.Error()),
			}, nil
		}
		s.logger.ErrorContext(ctx, "create conference", "error", err)
		return nil, err
	}
	return api.CreateConference201JSONResponse(conf), nil
}

// UpdateConference implements api.StrictServerInterface.
func (s *Server) UpdateConference(ctx context.Context, req api.UpdateConferenceRequestObject) (api.UpdateConferenceResponseObject, error) {
	actorID, err := actorFromContext(ctx)
	if err != nil {
		return api.UpdateConference401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized(err.Error()),
		}, nil
	}

	conf, err := s.confSvc.Update(ctx, uuid.UUID(req.Id), *req.Body, actorID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			return api.UpdateConference404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: notFound(),
			}, nil
		case errors.Is(err, service.ErrValidation):
			return api.UpdateConference400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: badRequest(err.Error()),
			}, nil
		case errors.Is(err, service.ErrConflict):
			return api.UpdateConference409ApplicationProblemPlusJSONResponse{
				ConflictApplicationProblemPlusJSONResponse: conflict(err.Error()),
			}, nil
		}
		s.logger.ErrorContext(ctx, "update conference", "id", req.Id, "error", err)
		return nil, err
	}
	return api.UpdateConference200JSONResponse(conf), nil
}

// DeleteConference implements api.StrictServerInterface.
func (s *Server) DeleteConference(ctx context.Context, req api.DeleteConferenceRequestObject) (api.DeleteConferenceResponseObject, error) {
	err := s.confSvc.Delete(ctx, uuid.UUID(req.Id))
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.DeleteConference404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: notFound(),
			}, nil
		}
		s.logger.ErrorContext(ctx, "delete conference", "id", req.Id, "error", err)
		return nil, err
	}
	return api.DeleteConference204Response{}, nil
}

// ArchiveConference implements api.StrictServerInterface.
func (s *Server) ArchiveConference(ctx context.Context, req api.ArchiveConferenceRequestObject) (api.ArchiveConferenceResponseObject, error) {
	conf, err := s.confSvc.Archive(ctx, uuid.UUID(req.Id))
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.ArchiveConference404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: notFound(),
			}, nil
		}
		s.logger.ErrorContext(ctx, "archive conference", "id", req.Id, "error", err)
		return nil, err
	}
	return api.ArchiveConference200JSONResponse(conf), nil
}

// UnarchiveConference implements api.StrictServerInterface.
func (s *Server) UnarchiveConference(ctx context.Context, req api.UnarchiveConferenceRequestObject) (api.UnarchiveConferenceResponseObject, error) {
	conf, err := s.confSvc.Unarchive(ctx, uuid.UUID(req.Id))
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.UnarchiveConference404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: notFound(),
			}, nil
		}
		s.logger.ErrorContext(ctx, "unarchive conference", "id", req.Id, "error", err)
		return nil, err
	}
	return api.UnarchiveConference200JSONResponse(conf), nil
}

// ListTags implements api.StrictServerInterface.
func (s *Server) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	tags, err := s.confSvc.ListTags(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "list tags", "error", err)
		return nil, err
	}
	return api.ListTags200JSONResponse(tags), nil
}

// ListTracks implements api.StrictServerInterface.
func (s *Server) ListTracks(ctx context.Context, _ api.ListTracksRequestObject) (api.ListTracksResponseObject, error) {
	tracks, err := s.confSvc.ListTracks(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "list tracks", "error", err)
		return nil, err
	}
	return api.ListTracks200JSONResponse(tracks), nil
}

// StarConference implements api.StrictServerInterface.
func (s *Server) StarConference(ctx context.Context, req api.StarConferenceRequestObject) (api.StarConferenceResponseObject, error) {
	actorID, err := actorFromContext(ctx)
	if err != nil {
		return api.StarConference401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized(err.Error()),
		}, nil
	}
	if err := s.starSvc.Star(ctx, actorID, uuid.UUID(req.Id)); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.StarConference404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: notFound(),
			}, nil
		}
		s.logger.ErrorContext(ctx, "star conference", "id", req.Id, "error", err)
		return nil, err
	}
	return api.StarConference204Response{}, nil
}

// UnstarConference implements api.StrictServerInterface.
func (s *Server) UnstarConference(ctx context.Context, req api.UnstarConferenceRequestObject) (api.UnstarConferenceResponseObject, error) {
	actorID, err := actorFromContext(ctx)
	if err != nil {
		return api.UnstarConference401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized(err.Error()),
		}, nil
	}
	if err := s.starSvc.Unstar(ctx, actorID, uuid.UUID(req.Id)); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return api.UnstarConference404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse: notFound(),
			}, nil
		}
		s.logger.ErrorContext(ctx, "unstar conference", "id", req.Id, "error", err)
		return nil, err
	}
	return api.UnstarConference204Response{}, nil
}

// ListMyStars implements api.StrictServerInterface.
func (s *Server) ListMyStars(ctx context.Context, _ api.ListMyStarsRequestObject) (api.ListMyStarsResponseObject, error) {
	actorID, err := actorFromContext(ctx)
	if err != nil {
		return api.ListMyStars401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized(err.Error()),
		}, nil
	}
	confs, err := s.starSvc.ListStarred(ctx, actorID)
	if err != nil {
		s.logger.ErrorContext(ctx, "list my stars", "error", err)
		return nil, err
	}
	return api.ListMyStars200JSONResponse(confs), nil
}

// GetMySettings implements api.StrictServerInterface.
func (s *Server) GetMySettings(ctx context.Context, _ api.GetMySettingsRequestObject) (api.GetMySettingsResponseObject, error) {
	actorID, err := actorFromContext(ctx)
	if err != nil {
		return api.GetMySettings401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized(err.Error()),
		}, nil
	}
	settings, err := s.settingsSvc.Get(ctx, actorID)
	if err != nil {
		s.logger.ErrorContext(ctx, "get my settings", "error", err)
		return nil, err
	}
	return api.GetMySettings200JSONResponse(settings), nil
}

// UpdateMySettings implements api.StrictServerInterface.
func (s *Server) UpdateMySettings(ctx context.Context, req api.UpdateMySettingsRequestObject) (api.UpdateMySettingsResponseObject, error) {
	actorID, err := actorFromContext(ctx)
	if err != nil {
		return api.UpdateMySettings401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: unauthorized(err.Error()),
		}, nil
	}
	settings, err := s.settingsSvc.Update(ctx, actorID, *req.Body)
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			return api.UpdateMySettings400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: badRequest(err.Error()),
			}, nil
		}
		s.logger.ErrorContext(ctx, "update my settings", "error", err)
		return nil, err
	}
	return api.UpdateMySettings200JSONResponse(settings), nil
}

var _ api.StrictServerInterface = (*Server)(nil)

// actorFromContext extracts the actor UUID from session claims in ctx.
func actorFromContext(ctx context.Context) (uuid.UUID, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return uuid.Nil, errors.New("authentication required")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errors.New("invalid session subject")
	}
	return id, nil
}

func unauthorized(detail string) api.UnauthorizedApplicationProblemPlusJSONResponse {
	return api.UnauthorizedApplicationProblemPlusJSONResponse{
		Title: ptr("Unauthorized"), Status: ptr(401), Detail: ptr(detail),
	}
}

func badRequest(detail string) api.BadRequestApplicationProblemPlusJSONResponse {
	return api.BadRequestApplicationProblemPlusJSONResponse{
		Title: ptr("Bad Request"), Status: ptr(400), Detail: ptr(detail),
	}
}

func conflict(detail string) api.ConflictApplicationProblemPlusJSONResponse {
	return api.ConflictApplicationProblemPlusJSONResponse{
		Title: ptr("Conflict"), Status: ptr(409), Detail: ptr(detail),
	}
}

func notFound() api.NotFoundApplicationProblemPlusJSONResponse {
	return api.NotFoundApplicationProblemPlusJSONResponse{
		Title: ptr("Not Found"), Status: ptr(404), Detail: ptr("conference not found"),
	}
}

func ptr[T any](v T) *T { return &v }
