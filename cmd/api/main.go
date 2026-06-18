// Command api is the Ruammit HTTP API server.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"ruammit-backend/internal/platform/config"
	"ruammit-backend/internal/platform/logging"
	"ruammit-backend/internal/platform/server"
	"ruammit-backend/internal/platform/storage"
)

func main() {
	// Load .env into the environment for local dev. Missing file is fine —
	// in production, config comes from real environment variables.
	_ = godotenv.Load()

	cfg := config.Load()
	log := logging.New(cfg.Env)

	db, err := storage.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database connected")

	srv := server.New(cfg, log, db)

	// Run the server; on error, signal the main goroutine to exit.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for an interrupt or a fatal server error.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Error("server error", "err", err)
	case sig := <-stop:
		log.Info("shutting down", "signal", sig.String())
	}

	// Graceful shutdown with a bounded timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	log.Info("stopped")
}
