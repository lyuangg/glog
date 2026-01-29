package glog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestNewHandler_DefaultOptions(t *testing.T) {
	handler := NewHandler(nil)
	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}
	defer handler.Close()

	if handler.opts == nil {
		t.Fatal("opts is nil")
	}
	if handler.opts.LogPath != "" {
		t.Errorf("expected default LogPath empty, got %q", handler.opts.LogPath)
	}
	if handler.opts.Format != FormatLine {
		t.Errorf("expected FormatLine, got %v", handler.opts.Format)
	}
	if handler.opts.Level != slog.LevelInfo {
		t.Errorf("expected LevelInfo, got %v", handler.opts.Level)
	}
}

func TestNewHandler_CustomOptions(t *testing.T) {
	opts := &Options{
		Format:           FormatJSON,
		Level:            slog.LevelDebug,
		AddSource:        true,
		TraceIDFieldName: "traceId",
		SpanIDFieldName:  "spanId",
	}

	handler := NewHandler(opts)
	defer handler.Close()

	if handler.traceIDFieldName != "traceId" {
		t.Errorf("expected traceId, got %s", handler.traceIDFieldName)
	}
	if handler.spanIDFieldName != "spanId" {
		t.Errorf("expected spanId, got %s", handler.spanIDFieldName)
	}
}

func TestHandler_WithWriterInOptions(t *testing.T) {
	var buf bytes.Buffer

	opts := &Options{
		Writer:           &buf,
		Format:           FormatJSON,
		Level:            slog.LevelInfo,
		TraceExtractor:   DefaultTraceExtractor,
		TraceIDFieldName: "trace_id",
		SpanIDFieldName:  "span_id",
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)

	ctx := context.WithValue(context.Background(), "trace_id", "options-writer-trace")
	ctx = context.WithValue(ctx, "span_id", "options-writer-span")

	logger.InfoContext(ctx, "test with writer in options")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}

	if logEntry["trace_id"] != "options-writer-trace" {
		t.Errorf("expected trace_id=options-writer-trace, got %v", logEntry["trace_id"])
	}
	if logEntry["span_id"] != "options-writer-span" {
		t.Errorf("expected span_id=options-writer-span, got %v", logEntry["span_id"])
	}
}

func TestHandler_WriterPriority(t *testing.T) {
	var buf1 bytes.Buffer

	// Writer takes priority over LogPath
	opts := &Options{
		Writer:  &buf1,
		LogPath: "/tmp/test.log",
		Format:  FormatJSON,
		Level:   slog.LevelInfo,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("test priority")

	output := strings.TrimSpace(buf1.String())
	if output == "" {
		t.Error("expected output in buf1, but it's empty")
	}

	// file should not be created when Writer is set
	if _, err := os.Stat("/tmp/test.log"); err == nil {
		t.Error("file should not be created when Writer is set")
	}
}

func TestHandler_DefaultTimeFormat(t *testing.T) {
	var buf bytes.Buffer

	opts := &Options{
		Writer: &buf,
		Format: FormatText,
		Level:  slog.LevelInfo,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("test time format")

	output := buf.String()
	if !strings.Contains(output, `time="`) {
		t.Error("output should contain time field")
	}
	timePattern := `time="\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}"`
	if matched, _ := regexp.MatchString(timePattern, output); !matched {
		t.Errorf("time format should be '2006-01-02 15:04:05', got: %s", output)
	}
}

func TestHandler_CustomReplaceAttrWithDefaultTimeFormat(t *testing.T) {
	var buf bytes.Buffer

	// custom ReplaceAttr is merged with default time format
	customReplaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) == 0 && a.Key == slog.LevelKey {
			return slog.String(a.Key, a.Value.String())
		}
		return a
	}

	opts := &Options{
		Writer:      &buf,
		Format:      FormatText,
		Level:       slog.LevelInfo,
		ReplaceAttr: customReplaceAttr,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("test custom replace attr")

	output := buf.String()
	timePattern := `time="\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}"`
	if matched, _ := regexp.MatchString(timePattern, output); !matched {
		t.Errorf("time format should still be '2006-01-02 15:04:05', got: %s", output)
	}
	if !strings.Contains(output, "level=") {
		t.Error("output should contain level field")
	}
}

func TestHandler_RecordHandler_Basic(t *testing.T) {
	var buf bytes.Buffer
	var handlerCalled bool
	var handlerCtx context.Context

	recordHandler := func(ctx context.Context, r *slog.Record) {
		handlerCalled = true
		handlerCtx = ctx
		r.AddAttrs(slog.String("service", "test-service"))
		r.AddAttrs(slog.Int("version", 1))
	}

	opts := &Options{
		Writer:        &buf,
		Format:        FormatJSON,
		Level:         slog.LevelInfo,
		RecordHandler: recordHandler,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("test message")

	if !handlerCalled {
		t.Error("RecordHandler was not called")
	}
	if handlerCtx == nil {
		t.Error("RecordHandler received nil context")
	}

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatal("output is empty")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}

	if logEntry["service"] != "test-service" {
		t.Errorf("expected service=test-service, got %v", logEntry["service"])
	}
	if logEntry["version"] != float64(1) {
		t.Errorf("expected version=1, got %v", logEntry["version"])
	}
}

func TestHandler_RecordHandler_WithContext(t *testing.T) {
	var buf bytes.Buffer

	recordHandler := func(ctx context.Context, r *slog.Record) {
		if userID := ctx.Value("user_id"); userID != nil {
			if uid, ok := userID.(string); ok {
				r.AddAttrs(slog.String("user_id", uid))
			}
		}
		if requestID := ctx.Value("request_id"); requestID != nil {
			if rid, ok := requestID.(string); ok {
				r.AddAttrs(slog.String("request_id", rid))
			}
		}
	}

	opts := &Options{
		Writer:        &buf,
		Format:        FormatJSON,
		Level:         slog.LevelInfo,
		RecordHandler: recordHandler,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)

	ctx := context.WithValue(context.Background(), "user_id", "user123")
	ctx = context.WithValue(ctx, "request_id", "req456")

	logger.InfoContext(ctx, "test with context")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}

	if logEntry["user_id"] != "user123" {
		t.Errorf("expected user_id=user123, got %v", logEntry["user_id"])
	}
	if logEntry["request_id"] != "req456" {
		t.Errorf("expected request_id=req456, got %v", logEntry["request_id"])
	}
}

func TestHandler_RecordHandler_WithTraceExtractor(t *testing.T) {
	var buf bytes.Buffer

	recordHandler := func(ctx context.Context, r *slog.Record) {
		// RecordHandler runs after TraceExtractor, so it can see trace fields
		r.AddAttrs(slog.String("handler_type", "custom"))
	}

	opts := &Options{
		Writer:           &buf,
		Format:           FormatJSON,
		Level:            slog.LevelInfo,
		TraceExtractor:   DefaultTraceExtractor,
		TraceIDFieldName: "trace_id",
		SpanIDFieldName:  "span_id",
		RecordHandler:    recordHandler,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)

	ctx := context.WithValue(context.Background(), "trace_id", "trace-123")
	ctx = context.WithValue(ctx, "span_id", "span-456")

	logger.InfoContext(ctx, "test with trace and handler")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}

	if logEntry["trace_id"] != "trace-123" {
		t.Errorf("expected trace_id=trace-123, got %v", logEntry["trace_id"])
	}
	if logEntry["span_id"] != "span-456" {
		t.Errorf("expected span_id=span-456, got %v", logEntry["span_id"])
	}

	if logEntry["handler_type"] != "custom" {
		t.Errorf("expected handler_type=custom, got %v", logEntry["handler_type"])
	}
}

func TestHandler_RecordHandler_TextFormat(t *testing.T) {
	var buf bytes.Buffer

	recordHandler := func(ctx context.Context, r *slog.Record) {
		r.AddAttrs(slog.String("env", "test"))
		r.AddAttrs(slog.String("region", "us-east-1"))
	}

	opts := &Options{
		Writer:        &buf,
		Format:        FormatText,
		Level:         slog.LevelInfo,
		RecordHandler: recordHandler,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("test text format")

	output := buf.String()

	if !strings.Contains(output, "env=test") {
		t.Errorf("output should contain env=test, got: %s", output)
	}
	if !strings.Contains(output, "region=us-east-1") {
		t.Errorf("output should contain region=us-east-1, got: %s", output)
	}
}

func TestHandler_RecordHandler_MultipleAttributes(t *testing.T) {
	var buf bytes.Buffer

	recordHandler := func(ctx context.Context, r *slog.Record) {
		r.AddAttrs(
			slog.String("string_field", "value1"),
			slog.Int("int_field", 42),
			slog.Bool("bool_field", true),
			slog.Float64("float_field", 3.14),
		)
	}

	opts := &Options{
		Writer:        &buf,
		Format:        FormatJSON,
		Level:         slog.LevelInfo,
		RecordHandler: recordHandler,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("test multiple attributes")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}

	if logEntry["string_field"] != "value1" {
		t.Errorf("expected string_field=value1, got %v", logEntry["string_field"])
	}
	if logEntry["int_field"] != float64(42) {
		t.Errorf("expected int_field=42, got %v", logEntry["int_field"])
	}
	if logEntry["bool_field"] != true {
		t.Errorf("expected bool_field=true, got %v", logEntry["bool_field"])
	}
	if logEntry["float_field"] != 3.14 {
		t.Errorf("expected float_field=3.14, got %v", logEntry["float_field"])
	}
}

func TestHandler_RecordHandler_NilHandler(t *testing.T) {
	var buf bytes.Buffer

	opts := &Options{
		Writer:        &buf,
		Format:        FormatJSON,
		Level:         slog.LevelInfo,
		RecordHandler: nil,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("test without handler")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}

	if logEntry["msg"] != "test without handler" {
		t.Errorf("expected msg=test without handler, got %v", logEntry["msg"])
	}
}

func TestHandler_RecordHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer

	recordHandler := func(ctx context.Context, r *slog.Record) {
		r.AddAttrs(slog.String("global_field", "global_value"))
	}

	opts := &Options{
		Writer:        &buf,
		Format:        FormatJSON,
		Level:         slog.LevelInfo,
		RecordHandler: recordHandler,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	newHandler := handler.WithGroup("request")
	logger := slog.New(newHandler)

	logger.Info("test with group", "method", "GET", "path", "/api")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}

	requestGroup, ok := logEntry["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected request group, got %v", logEntry["request"])
	}

	if requestGroup["method"] != "GET" {
		t.Errorf("expected method=GET, got %v", requestGroup["method"])
	}

	// RecordHandler-added fields go into the current group (request)
	if requestGroup["global_field"] != "global_value" {
		t.Errorf("expected global_field=global_value in request group, got %v", requestGroup["global_field"])
	}
}

func TestHandler_RecordHandler_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	var callCount int64
	var mu sync.Mutex

	recordHandler := func(ctx context.Context, r *slog.Record) {
		mu.Lock()
		callCount++
		currentCallID := callCount
		mu.Unlock()
		r.AddAttrs(slog.Int("call_id", int(currentCallID)))
	}

	opts := &Options{
		Writer:        &buf,
		Format:        FormatJSON,
		Level:         slog.LevelInfo,
		RecordHandler: recordHandler,
	}

	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)

	const numGoroutines = 50
	const logsPerGoroutine = 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < logsPerGoroutine; j++ {
				logger.Info("concurrent test", "goroutine", id, "log", j)
			}
		}(i)
	}

	wg.Wait()

	mu.Lock()
	expectedCalls := numGoroutines * logsPerGoroutine
	actualCalls := callCount
	mu.Unlock()

	if actualCalls != int64(expectedCalls) {
		t.Errorf("expected RecordHandler to be called %d times, got %d", expectedCalls, actualCalls)
	}

	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) != expectedCalls {
		t.Errorf("expected %d log lines, got %d", expectedCalls, len(lines))
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"DEBUG", "DEBUG", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"Info", "Info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"warning", "warning", slog.LevelWarn},
		{"WARN", "WARN", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"ERROR", "ERROR", slog.LevelError},
		{"trim space", "  info  ", slog.LevelInfo},
		{"unknown default", "unknown", slog.LevelInfo},
		{"empty default", "", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLevel(tt.input); got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandler_Enabled(t *testing.T) {
	var buf bytes.Buffer
	opts := &Options{
		Writer: &buf,
		Format: FormatJSON,
		Level:  slog.LevelInfo,
	}
	handler := NewHandler(opts)
	defer handler.Close()
	ctx := context.Background()

	if handler.Enabled(ctx, slog.LevelDebug) {
		t.Error("LevelDebug should be disabled when Level is Info")
	}
	if !handler.Enabled(ctx, slog.LevelInfo) {
		t.Error("LevelInfo should be enabled")
	}
	if !handler.Enabled(ctx, slog.LevelWarn) {
		t.Error("LevelWarn should be enabled")
	}
	if !handler.Enabled(ctx, slog.LevelError) {
		t.Error("LevelError should be enabled")
	}

	optsDebug := &Options{Writer: &buf, Format: FormatJSON, Level: slog.LevelDebug}
	hDebug := NewHandler(optsDebug)
	defer hDebug.Close()
	if !hDebug.Enabled(ctx, slog.LevelDebug) {
		t.Error("LevelDebug should be enabled when Level is Debug")
	}
}

func TestHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	opts := &Options{
		Writer: &buf,
		Format: FormatJSON,
		Level:  slog.LevelInfo,
	}
	handler := NewHandler(opts)
	defer handler.Close()

	newHandler := handler.WithAttrs([]slog.Attr{
		slog.String("app", "glog-test"),
		slog.Int("pid", 12345),
	})
	logger := slog.New(newHandler)
	logger.Info("message")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}
	if logEntry["app"] != "glog-test" {
		t.Errorf("expected app=glog-test, got %v", logEntry["app"])
	}
	if logEntry["pid"] != float64(12345) {
		t.Errorf("expected pid=12345, got %v", logEntry["pid"])
	}
}

func TestDefaultTraceExtractor(t *testing.T) {
	keyTests := []struct {
		name    string
		ctx     context.Context
		wantNil bool
		traceID string
		spanID  string
	}{
		{"trace_id span_id", context.WithValue(context.WithValue(context.Background(), "trace_id", "t1"), "span_id", "s1"), false, "t1", "s1"},
		{"traceId spanId", context.WithValue(context.WithValue(context.Background(), "traceId", "t2"), "spanId", "s2"), false, "t2", "s2"},
		{"TraceID SpanID", context.WithValue(context.WithValue(context.Background(), "TraceID", "t3"), "SpanID", "s3"), false, "t3", "s3"},
		{"TRACE_ID SPAN_ID", context.WithValue(context.WithValue(context.Background(), "TRACE_ID", "t4"), "SPAN_ID", "s4"), false, "t4", "s4"},
		{"no keys", context.Background(), true, "", ""},
		{"empty strings", context.WithValue(context.WithValue(context.Background(), "trace_id", ""), "span_id", ""), true, "", ""},
	}
	for _, tt := range keyTests {
		t.Run(tt.name, func(t *testing.T) {
			info := DefaultTraceExtractor(tt.ctx)
			if tt.wantNil {
				if info != nil {
					t.Errorf("expected nil, got %+v", info)
				}
				return
			}
			if info == nil {
				t.Fatal("expected non-nil TraceInfo")
			}
			if info.TraceID != tt.traceID || info.SpanID != tt.spanID {
				t.Errorf("got TraceID=%q SpanID=%q, want %q %q", info.TraceID, info.SpanID, tt.traceID, tt.spanID)
			}
		})
	}
}

func TestHandler_TraceExtractorReturnsNil(t *testing.T) {
	var buf bytes.Buffer
	opts := &Options{
		Writer:           &buf,
		Format:           FormatJSON,
		Level:            slog.LevelInfo,
		TraceExtractor:   DefaultTraceExtractor,
		TraceIDFieldName: "trace_id",
		SpanIDFieldName:  "span_id",
	}
	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "no trace")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON: %v, output: %s", err, output)
	}
	if _, has := logEntry["trace_id"]; has {
		t.Error("should not have trace_id when extractor returns nil")
	}
	if _, has := logEntry["span_id"]; has {
		t.Error("should not have span_id when extractor returns nil")
	}
}

func TestHandler_FormatLine_Output(t *testing.T) {
	var buf bytes.Buffer
	opts := &Options{
		Writer: &buf,
		Format: FormatLine,
		Level:  slog.LevelInfo,
	}
	handler := NewHandler(opts)
	defer handler.Close()

	logger := slog.New(handler)
	logger.Info("line format test")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Error("Line format should contain level INFO")
	}
	if !strings.Contains(output, "line format test") {
		t.Error("Line format should contain message")
	}
	timePattern := `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`
	if matched, _ := regexp.MatchString(timePattern, output); !matched {
		t.Errorf("Line format should contain time like 2006-01-02 15:04:05, got: %s", output)
	}
}

func TestHandler_Close_NonCloserWriter(t *testing.T) {
	var buf bytes.Buffer
	opts := &Options{Writer: &buf, Format: FormatJSON, Level: slog.LevelInfo}
	handler := NewHandler(opts)
	// bytes.Buffer is not io.Closer, Close should return nil
	if err := handler.Close(); err != nil {
		t.Errorf("Close with non-Closer writer should return nil, got %v", err)
	}
}
