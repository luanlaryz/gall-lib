package logger

import (
	"io"
	"log/slog"
	"os"
)

// Level identifies the minimum log level emitted by the simple logger.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// SimpleOptions configures the local slog-backed logger helper.
type SimpleOptions struct {
	Writer    io.Writer
	Level     Level
	AddSource bool
}

// Default returns a simple stderr-backed logger suitable for local development.
func Default() Logger {
	return NewSimple(SimpleOptions{})
}

// NewSimple builds a local slog-backed logger without any hosted dependency.
func NewSimple(opts SimpleOptions) Logger {
	writer := opts.Writer
	if writer == nil {
		writer = os.Stderr
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		AddSource: opts.AddSource,
		Level:     toSlogLevel(opts.Level),
	})
	return New(slog.New(handler))
}

func toSlogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case "", LevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}
