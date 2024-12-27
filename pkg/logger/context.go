package logger

import (
	"context"
)

type loggerKey struct{}

// FromContext retrieves the logger from the context
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return Get()
	}
	if logger, ok := ctx.Value(loggerKey{}).(Logger); ok {
		return logger
	}
	return Get()
}

// IntoContext injects a logger into the context
func IntoContext(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}
