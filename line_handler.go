package glog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// LineHandler implements slog.Handler and writes single-line text logs in the form:
// [2024-01-01 12:00:00] LEVEL: message {"key":"val",...}
//
// Time uses "2006-01-02 15:04:05"; level is string (INFO, ERROR, etc.); structured
// fields are collected as a JSON object at the end. Supports Level, ReplaceAttr, WithAttrs, WithGroup.
type LineHandler struct {
	w      io.Writer
	opts   slog.HandlerOptions
	mu     sync.Mutex  // guards concurrent writes
	attrs  []slog.Attr // attributes from WithAttrs
	groups []string    // group prefix from WithGroup
}

// NewLineHandler creates a new LineHandler.
func NewLineHandler(w io.Writer, opts *slog.HandlerOptions) *LineHandler {
	var o slog.HandlerOptions
	if opts != nil {
		o = *opts
	}
	return &LineHandler{
		w:    w,
		opts: o,
	}
}

// Enabled reports whether the given level is enabled.
func (h *LineHandler) Enabled(_ context.Context, level slog.Level) bool {
	if h.opts.Level == nil {
		return true
	}
	return level >= h.opts.Level.Level()
}

// Handle writes a log record as a single line.
func (h *LineHandler) Handle(_ context.Context, r slog.Record) error {
	timeAttr := slog.Time(slog.TimeKey, r.Time)
	if h.opts.ReplaceAttr != nil {
		timeAttr = h.opts.ReplaceAttr(nil, timeAttr)
	}
	timeStr := r.Time.Format("2006-01-02 15:04:05")
	if timeAttr.Value.Kind() == slog.KindString {
		timeStr = timeAttr.Value.String()
	}

	levelAttr := slog.String(slog.LevelKey, r.Level.String())
	if h.opts.ReplaceAttr != nil {
		levelAttr = h.opts.ReplaceAttr(nil, levelAttr)
	}
	levelStr := levelAttr.Value.String()

	fields := make(map[string]any, r.NumAttrs()+len(h.attrs))

	prefix := strings.Join(h.groups, ".")
	addAttr := func(groups []string, a slog.Attr) {
		if h.opts.ReplaceAttr != nil {
			a = h.opts.ReplaceAttr(groups, a)
		}
		if a.Key == "" {
			return
		}
		key := a.Key
		if prefix != "" {
			key = prefix + "." + key
		}
		fields[key] = a.Value.Any()
	}

	for _, a := range h.attrs {
		addAttr(h.groups, a)
	}

	r.Attrs(func(a slog.Attr) bool {
		addAttr(nil, a)
		return true
	})

	var contextJSON string
	if len(fields) > 0 {
		if b, err := json.Marshal(fields); err == nil {
			contextJSON = " " + string(b)
		}
	}

	line := fmt.Sprintf("[%s] %s: %s%s\n", timeStr, levelStr, r.Message, contextJSON)

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, line)
	return err
}

// WithAttrs returns a new LineHandler with the given attributes.
func (h *LineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LineHandler{
		w:      h.w,
		opts:   h.opts,
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
		groups: append([]string{}, h.groups...),
	}
}

// WithGroup returns a new LineHandler with the given group name prefix.
func (h *LineHandler) WithGroup(name string) slog.Handler {
	return &LineHandler{
		w:      h.w,
		opts:   h.opts,
		attrs:  append([]slog.Attr{}, h.attrs...),
		groups: append(append([]string{}, h.groups...), name),
	}
}
