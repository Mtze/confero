package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"confero/internal/database"
	chihttp "confero/internal/http"
	"confero/internal/version"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	addr := ":8080"
	if a := os.Getenv("CONFERO_HTTP_ADDR"); a != "" {
		addr = a
	}

	srv := chihttp.NewServer(logger)
	router := chihttp.NewRouter(srv)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Graceful shutdown on SIGTERM / SIGINT.
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

	if dsn := os.Getenv("CONFERO_DATABASE_URL"); dsn != "" {
		logger.Info("running migrations")
		if err := database.RunMigrations(dsn); err != nil {
			logger.Error("migration failed", "err", err)
			os.Exit(1)
		}
	}

	logger.Info("starting server",
		"addr", addr,
		"version", version.Version,
		"commit", version.Commit,
	)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
	<-done
}
