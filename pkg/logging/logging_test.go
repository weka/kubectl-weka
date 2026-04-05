package logging

import (
	"context"
	"testing"
)

func TestWithLogger(t *testing.T) {
	ctx := context.Background()
	logger := NewLoggerWithFile("")

	newCtx := WithLogger(ctx, logger)

	if newCtx == ctx {
		t.Error("WithLogger should return a new context")
	}

	retrieved := newCtx.Value(ctxKeyLogger)
	if retrieved != logger {
		t.Error("Logger should be stored in context")
	}
}

func TestGetLogger(t *testing.T) {
	t.Run("get logger from context", func(t *testing.T) {
		ctx := context.Background()
		logger := NewLoggerWithFile("")
		ctx = WithLogger(ctx, logger)

		retrieved := GetLogger(ctx)
		if retrieved != logger {
			t.Error("GetLogger should return logger from context")
		}
	})

	t.Run("create logger when not in context", func(t *testing.T) {
		ctx := context.Background()
		logger := GetLogger(ctx)

		if logger == nil {
			t.Error("GetLogger should create a new logger when not in context")
		}
	})

	t.Run("create logger with file", func(t *testing.T) {
		ctx := context.Background()
		logger := GetLogger(ctx, "/tmp/test.log")

		if logger == nil {
			t.Error("GetLogger should create a logger with file")
		}
	})
}
