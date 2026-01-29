package glog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLineHandler_BasicFormat(t *testing.T) {
	var buf bytes.Buffer

	h := NewLineHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(h)

	logger.Info("user login", slog.String("user_id", "123"))

	out := strings.TrimSpace(buf.String())
	if !strings.Contains(out, "] INFO: user login") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, `"user_id":"123"`) {
		t.Fatalf("expected user_id field in output, got: %s", out)
	}
}

func TestLineHandler_LevelFilter(t *testing.T) {
	var buf bytes.Buffer

	h := NewLineHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(h)

	logger.Debug("debug message")
	if buf.Len() != 0 {
		t.Fatalf("expected no output for debug level, got: %s", buf.String())
	}

	logger.Info("info message")
	if !strings.Contains(buf.String(), "INFO: info message") {
		t.Fatalf("expected info message in output, got: %s", buf.String())
	}
}

func TestLineHandler_ReplaceAttr_TimeAndLevel(t *testing.T) {
	var buf bytes.Buffer

	replace := func(groups []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case slog.TimeKey:
			return slog.String(a.Key, "CUSTOM_TIME")
		case slog.LevelKey:
			return slog.String(a.Key, "CUSTOM_LEVEL")
		default:
			return a
		}
	}

	h := NewLineHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: replace,
	})
	logger := slog.New(h)

	logger.LogAttrs(context.Background(), slog.LevelInfo, "msg",
		slog.Time(slog.TimeKey, time.Now()),
	)

	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "[CUSTOM_TIME]") {
		t.Fatalf("expected custom time in output, got: %s", out)
	}
	if !strings.Contains(out, "CUSTOM_LEVEL: msg") {
		t.Fatalf("expected custom level in output, got: %s", out)
	}
}

func TestLineHandler_WithAttrsAndWithGroup(t *testing.T) {
	var buf bytes.Buffer

	h := NewLineHandler(&buf, &slog.HandlerOptions{})
	h2 := h.WithAttrs([]slog.Attr{slog.String("app", "demo")})
	h3 := h2.WithGroup("http")

	logger := slog.New(h3)
	logger.Info("req",
		slog.String("method", "GET"),
	)

	out := strings.TrimSpace(buf.String())

	// app gets http group prefix
	if !strings.Contains(out, `"http.app":"demo"`) {
		t.Fatalf("expected http.app field in output, got: %s", out)
	}
	// http group prefix for method
	if !strings.Contains(out, `"http.method":"GET"`) {
		t.Fatalf("expected http.method field in output, got: %s", out)
	}
}
