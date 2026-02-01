// Package logger provides structured logging setup using Go's slog package.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Options configures the logger setup.
type Options struct {
	Level   string // debug, info, warn, error
	Console bool   // pretty print for dev (LOG_FORMAT=console)
}

// Setup configures the global slog logger with correlation ID support.
func Setup(opts Options) {
	handlerOpts := &slog.HandlerOptions{
		Level:     parseLevel(opts.Level),
		AddSource: true,
	}

	var handler slog.Handler
	if opts.Console {
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	}

	// Wrap with correlation handler to auto-inject correlation_id from context
	handler = NewCorrelationHandler(handler)

	slog.SetDefault(slog.New(handler))
}

// parseLevel converts string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
