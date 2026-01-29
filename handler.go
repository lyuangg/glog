package glog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// FormatType is the log output format type.
type FormatType int

const (
	FormatLine FormatType = iota // single-line text (Laravel-style)
	FormatJSON                   // JSON (slog JSONHandler)
	FormatText                   // text (slog TextHandler)
)

const (
	defaultTraceIDFieldName = "trace_id"
	defaultSpanIDFieldName  = "span_id"
)

// TraceInfo holds trace/span identifiers for log records.
type TraceInfo struct {
	TraceID string // trace ID
	SpanID  string // span ID
	// Other fields (e.g. ParentSpanID, Sampled) may be added.
}

// TraceExtractor extracts trace info from context. If it returns nil, no trace fields are added.
type TraceExtractor func(ctx context.Context) *TraceInfo

// RecordHandler lets callers add or modify attributes on a log record before it is written.
// ctx is the request context; r is the record (use r.AddAttrs() to add attributes).
// Note: r is a pointer, so AddAttrs modifications take effect; each Handle call has its own Record, so passing &r is safe; protect shared state with your own locking if needed.
type RecordHandler func(ctx context.Context, r *slog.Record)

// defaultTimeReplaceAttr formats the top-level time attribute as "2006-01-02 15:04:05".
func defaultTimeReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	// only handle top-level "time"
	if len(groups) == 0 && a.Key == slog.TimeKey {
		return slog.String(a.Key, a.Value.Time().Format("2006-01-02 15:04:05"))
	}
	return a
}

// mergeReplaceAttr composes two ReplaceAttr funcs: defaultReplace first, then userReplace if non-nil.
func mergeReplaceAttr(defaultReplace, userReplace func(groups []string, a slog.Attr) slog.Attr) func(groups []string, a slog.Attr) slog.Attr {
	if userReplace == nil {
		return defaultReplace
	}
	if defaultReplace == nil {
		return userReplace
	}
	return func(groups []string, a slog.Attr) slog.Attr {
		a = defaultReplace(groups, a)
		return userReplace(groups, a)
	}
}

// DefaultTraceExtractor reads trace_id and span_id from context. Supported keys:
// trace_id, traceId, TraceID, TRACE_ID; span_id, spanId, SpanID, SPAN_ID.
func DefaultTraceExtractor(ctx context.Context) *TraceInfo {
	var traceID, spanID string

	traceKeys := []interface{}{"trace_id", "traceId", "TraceID", "TRACE_ID"}
	for _, key := range traceKeys {
		if val := ctx.Value(key); val != nil {
			if tid, ok := val.(string); ok && tid != "" {
				traceID = tid
				break
			}
		}
	}

	spanKeys := []interface{}{"span_id", "spanId", "SpanID", "SPAN_ID"}
	for _, key := range spanKeys {
		if val := ctx.Value(key); val != nil {
			if sid, ok := val.(string); ok && sid != "" {
				spanID = sid
				break
			}
		}
	}

	if traceID == "" && spanID == "" {
		return nil
	}

	return &TraceInfo{
		TraceID: traceID,
		SpanID:  spanID,
	}
}

// ParseLevel parses a string into slog.Level. Supports "debug", "info", "warn", "error" (case-insensitive). Returns slog.LevelInfo for unknown values.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Options configures the handler.
type Options struct {
	// Writer overrides LogPath when set (e.g. for tests or custom output). If nil, log goes to file when LogPath is set, otherwise stdout.
	Writer io.Writer
	// LogPath is the log file path; supports Go time layout (e.g. app-2006-01-02-15-04-05.log). Used when Writer is nil.
	LogPath string
	// MaxFiles is the max number of old log files to keep; 0 means no limit.
	MaxFiles int
	// FlushInterval is the buffer flush interval in seconds; 0 means flush on every write; >0 means periodic flush.
	FlushInterval int
	// Level filters out log records below this level.
	Level slog.Level
	// Format is the output format (text or JSON).
	Format FormatType
	// AddSource adds source file/line to log records when true.
	AddSource bool
	// ReplaceAttr replaces or modifies log attributes; nil means no replacement.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
	// TraceExtractor extracts trace info from context; nil means no trace injection.
	TraceExtractor TraceExtractor
	// TraceIDFieldName is the log field name for trace_id; default "trace_id".
	TraceIDFieldName string
	// SpanIDFieldName is the log field name for span_id; default "span_id".
	SpanIDFieldName string
	// RecordHandler is called after trace injection and before writing; nil means no extra processing.
	RecordHandler RecordHandler
}

// defaultOptions returns default Options.
func defaultOptions() *Options {
	return &Options{
		LogPath:          "",
		MaxFiles:         0,
		FlushInterval:    0,
		Level:            slog.LevelInfo,
		Format:           FormatLine,
		AddSource:        false,
		ReplaceAttr:      nil,
		TraceExtractor:   nil,
		TraceIDFieldName: defaultTraceIDFieldName,
		SpanIDFieldName:  defaultSpanIDFieldName,
		RecordHandler:    nil,
	}
}

// Handler implements slog.Handler.
type Handler struct {
	opts             *Options
	writer           io.Writer
	handler          slog.Handler
	traceExtractor   TraceExtractor
	traceIDFieldName string
	spanIDFieldName  string
	recordHandle     RecordHandler
}

// NewHandler creates a new Handler.
func NewHandler(opts *Options) *Handler {
	if opts == nil {
		opts = defaultOptions()
	}

	h := &Handler{
		opts:             opts,
		traceExtractor:   opts.TraceExtractor,
		traceIDFieldName: opts.TraceIDFieldName,
		spanIDFieldName:  opts.SpanIDFieldName,
		recordHandle:     opts.RecordHandler,
	}

	// Writer takes precedence; else use file when LogPath is set, else stdout
	if opts.Writer != nil {
		h.writer = opts.Writer
	} else if opts.LogPath != "" {
		h.writer = NewFileWriterWithFlushInterval(opts.LogPath, opts.MaxFiles, opts.FlushInterval)
	} else {
		h.writer = os.Stdout
	}

	replaceAttr := mergeReplaceAttr(defaultTimeReplaceAttr, opts.ReplaceAttr)
	handlerOpts := &slog.HandlerOptions{
		Level:       opts.Level,
		AddSource:   opts.AddSource,
		ReplaceAttr: replaceAttr,
	}

	switch opts.Format {
	case FormatJSON:
		h.handler = slog.NewJSONHandler(h.writer, handlerOpts)
	case FormatText:
		h.handler = slog.NewTextHandler(h.writer, handlerOpts)
	case FormatLine:
		h.handler = NewLineHandler(h.writer, handlerOpts)
	default:
		h.handler = NewLineHandler(h.writer, handlerOpts)
	}

	return h
}

// Enabled reports whether the given level is enabled.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle processes a log record.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if h.traceExtractor != nil {
		if traceInfo := h.traceExtractor(ctx); traceInfo != nil {
			traceKey := h.traceIDFieldName
			if traceKey == "" {
				traceKey = defaultTraceIDFieldName
			}
			spanKey := h.spanIDFieldName
			if spanKey == "" {
				spanKey = defaultSpanIDFieldName
			}
			if traceInfo.TraceID != "" {
				r.AddAttrs(slog.String(traceKey, traceInfo.TraceID))
			}
			if traceInfo.SpanID != "" {
				r.AddAttrs(slog.String(spanKey, traceInfo.SpanID))
			}
		}
	}
	if h.recordHandle != nil {
		h.recordHandle(ctx, &r)
	}
	return h.handler.Handle(ctx, r)
}

// WithAttrs returns a new Handler with the given attributes.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		opts:             h.opts,
		writer:           h.writer,
		handler:          h.handler.WithAttrs(attrs),
		traceExtractor:   h.traceExtractor,
		traceIDFieldName: h.traceIDFieldName,
		spanIDFieldName:  h.spanIDFieldName,
		recordHandle:     h.recordHandle,
	}
}

// WithGroup returns a new Handler with the given group name.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		opts:             h.opts,
		writer:           h.writer,
		handler:          h.handler.WithGroup(name),
		traceExtractor:   h.traceExtractor,
		traceIDFieldName: h.traceIDFieldName,
		spanIDFieldName:  h.spanIDFieldName,
		recordHandle:     h.recordHandle,
	}
}

// Close closes the Handler and releases resources.
func (h *Handler) Close() error {
	if closer, ok := h.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
