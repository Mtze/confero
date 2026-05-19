package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"confero/internal/auth"
	"confero/internal/config"
	"confero/internal/database"
	chihttp "confero/internal/http"
	"confero/internal/mail"
	"confero/internal/repository"
	"confero/internal/scheduler"
	"confero/internal/service"
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

	confSvc := service.NewConferenceService(pool)
	starSvc := service.NewStarService(pool)
	settingsSvc := service.NewSettingsService(pool)
	srv := chihttp.NewServer(logger, confSvc, starSvc, settingsSvc)
	router := chihttp.NewRouter(srv, tm, oidcHandler)

	mailer := &mail.FakeMailer{}
	sched := scheduler.New(scheduler.Config{
		Tick:      60 * time.Second,
		Mailer:    mailer,
		DB:        pool,
		GraceDays: cfg.ArchiveGraceDays,
		Logger:    logger,
	}, nil)

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("starting server",
			"addr", cfg.HTTPAddr,
			"version", version.Version,
			"commit", version.Commit,
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		return sched.Run(gctx)
	})

	g.Go(func() error {
		<-gctx.Done()
		logger.Info("shutting down")
		return httpServer.Close()
	})

	if err := g.Wait(); err != nil {
		logger.Error("server error", "err", err)
		return 1
	}
	return 0
}
