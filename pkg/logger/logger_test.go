package logger_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/logger"
)

func TestWithContextRoundTripsLogger(t *testing.T) {
	t.Parallel()

	log := logger.Default()
	ctx := logger.WithContext(context.Background(), log)

	if got := logger.FromContext(ctx); got == nil {
		t.Fatal("FromContext() returned nil")
	}
}

func TestNewSimpleWritesStructuredOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logger.NewSimple(logger.SimpleOptions{
		Writer: &buf,
		Level:  logger.LevelDebug,
	})

	log.InfoContext(context.Background(), "hello", "component", "test", "event_type", "logger.test")

	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Fatalf("output = %q want message", output)
	}
	if !strings.Contains(output, "component=test") {
		t.Fatalf("output = %q want component field", output)
	}
	if !strings.Contains(output, "event_type=logger.test") {
		t.Fatalf("output = %q want event_type field", output)
	}
}
