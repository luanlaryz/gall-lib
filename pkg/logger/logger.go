// Package logger provides the small logging facade used by public APIs.
package logger

import (
	"context"
	"io"
	"log/slog"
)

// Logger is the small logging contract shared by gaal-lib packages.
type Logger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
}

type slogLogger struct {
	logger *slog.Logger
}

// New wraps a slog logger with the public Logger contract.
func New(log *slog.Logger) Logger {
	if log == nil {
		return Nop()
	}

	return slogLogger{logger: log}
}

// Nop returns a logger that discards all output.
func Nop() Logger {
	return slogLogger{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func (l slogLogger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, args...)
}

func (l slogLogger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

func (l slogLogger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

func (l slogLogger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}
