package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"confero/internal/auth"
	"confero/internal/config"
	"confero/internal/database"
	chihttp "confero/internal/http"
	"confero/internal/repository"
	"confero/internal/version"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg := config.Load()

	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "err", err)
		return 1
	}

	ctx := context.Background()

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		logger.Error("migration failed", "err", err)
		return 1
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database pool failed", "err", err)
		return 1
	}
	defer pool.Close()

	queries := repository.New(pool)
	tm := auth.NewTokenManager(cfg.SessionSecret)

	oidcHandler, err := auth.NewOIDCHandler(
		ctx,
		cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.OIDCRedirectURL,
		cfg.OIDCMemberValue, cfg.OIDCAdminValue,
		tm, queries,
		logger,
	)
	if err != nil {
		logger.Error("OIDC provider discovery failed", "err", err)
		return 1
	}

	srv := chihttp.NewServer(logger)
	router := chihttp.NewRouter(srv, tm, oidcHandler)

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
	}

	done := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		logger.Info("shutting down")
		if err := httpServer.Close(); err != nil {
			logger.Error("close error", "err", err)
		}
		close(done)
	}()

	logger.Info("starting server",
		"addr", cfg.HTTPAddr,
		"version", version.Version,
		"commit", version.Commit,
	)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
		return 1
	}
	<-done
	return 0
}
