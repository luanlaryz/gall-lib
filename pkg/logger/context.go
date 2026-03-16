package logger

import "context"

type contextKey struct{}

// WithContext attaches log to ctx so downstream runtimes can reuse the same
// local logger without depending on concrete backends.
func WithContext(ctx context.Context, log Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = Nop()
	}
	return context.WithValue(ctx, contextKey{}, log)
}

// FromContext returns the logger attached to ctx or Nop when absent.
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return Nop()
	}
	if log, ok := ctx.Value(contextKey{}).(Logger); ok && log != nil {
		return log
	}
	return Nop()
}
