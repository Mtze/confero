// Package http wires the chi router, middleware, and handler implementations.
package http

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"confero/internal/api"
)

// Server implements api.StrictServerInterface and owns the handler implementations.
type Server struct {
	logger *slog.Logger
}

// NewServer constructs a Server.
func NewServer(logger *slog.Logger) *Server {
	return &Server{logger: logger}
}

// NewRouter builds and returns the fully-wired chi.Router.
func NewRouter(s *Server) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	api.HandlerFromMux(api.NewStrictHandler(s, nil), r)

	return r
}

// GetHealth implements api.StrictServerInterface.
func (s *Server) GetHealth(_ context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: api.Ok}, nil
}

// compile-time interface check
var _ api.StrictServerInterface = (*Server)(nil)
