package alog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNewSlogLogger(t *testing.T) {
	var buf bytes.Buffer
	prevW := getWriter()
	prevEnv := getLoggingEnvironment()
	prevLevel := getLoggingLevel()
	t.Cleanup(func() {
		SetWriter(prevW)
		SetLoggingEnvironment(prevEnv)
		SetLevel(prevLevel)
	})

	SetWriter(&buf)
	SetLoggingEnvironment(EnvironmentGoogle)
	SetLevel(LevelInfo)

	logger := NewSlogLogger(nil)
	logger.InfoContext(context.Background(), "hello", slog.String("key", "value"))

	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("json: %v", err)
	}
	if m["message"] != "hello" {
		t.Fatalf("message: %v", m["message"])
	}
	if m["key"] != "value" {
		t.Fatalf("attr key: %v", m["key"])
	}
}

func TestNewSlogLogger_defaultLevelerFromAlog(t *testing.T) {
	l := NewSlogLogger(nil)
	sh, ok := l.Handler().(*slogHandler)
	if !ok {
		t.Fatal("handler is not *slogHandler")
	}
	if _, ok := sh.opts.Level.(alogLeveler); !ok {
		t.Fatalf("expected alogDerivedLeveler when opts.Level unset, got %T", sh.opts.Level)
	}
}

func TestSlogHandler_Enabled_respectsAlogLevel(t *testing.T) {
	prev := getLoggingLevel()
	t.Cleanup(func() { SetLevel(prev) })

	SetLevel(LevelWarning)
	h := &slogHandler{}
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Info should be disabled when alog min is Warning")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("Error should be enabled when alog min is Warning")
	}
}
