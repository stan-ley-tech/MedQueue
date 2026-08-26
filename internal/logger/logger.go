// Package logger configures the application-wide structured logger.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New builds a slog.Logger writing structured logs to stdout. Format is
// either "json" (production) or "text" (local development readability).
func New(level, format string) *slog.Logger {
	handlerOpts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	var handler slog.Handler
	if strings.EqualFold(format, "text") {
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
