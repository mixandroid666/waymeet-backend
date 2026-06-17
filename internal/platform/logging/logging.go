// Package logging builds the application's structured logger (stdlib slog).
package logging

import (
	"log/slog"
	"os"
)

// New returns a slog.Logger. In production it emits JSON (machine-parseable for
// log aggregation); in development it emits human-readable text at debug level.
func New(env string) *slog.Logger {
	if env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
