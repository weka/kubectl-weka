package logging

import (
	"context"
)

const ctxKeyLogger = "weka:logger"

func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, logger)
}

// GetLogger returns a n existing logger in context.
// If no logger exists in the context, it attempts to create a new logger
// Also, optional filename can be provided to create a new logger that logs to both console and file
func GetLogger(ctx context.Context, file ...string) *Logger {
	if logger, ok := ctx.Value(ctxKeyLogger).(*Logger); ok {
		return logger
	}
	if len(file) == 0 {
		return NewLoggerWithFile("")
	}
	return NewLoggerWithFile(file[0])
}
