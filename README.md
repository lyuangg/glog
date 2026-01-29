## glog

[中文文档](README.zh.md)

A logging library built on Go’s standard `log/slog`, providing:

- **Multiple output formats**: single-line text, JSON, and Text
- **File rotation and retention**: rotate by time-based path patterns and keep at most N old files
- **Buffered flush**: optional periodic flush to reduce disk I/O
- **Trace injection**: extract `trace_id` / `span_id` from `context.Context` and add them to log fields
- **Custom record handling**: add or modify attributes before a record is written

### Install

```bash
go get github.com/lyuangg/glog
```

### Quick start

Create an `slog.Logger` that uses the single-line text format:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/lyuangg/glog"
)

func main() {
	handler := glog.NewHandler(&glog.Options{
		// Writes to stdout when Writer is nil and LogPath is empty
		Level:  slog.LevelInfo,
		Format: glog.FormatLine, // single-line text format
	})
	defer handler.Close()

	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "hello glog", slog.String("foo", "bar"))
}
```

### File output and rotation

`Options.LogPath` supports Go time layout patterns in the path. Example:

```go
handler := glog.NewHandler(&glog.Options{
	LogPath:       "logs/app-2006-01-02-15.log", // rotate by hour
	MaxFiles:      24,                           // keep at most 24 files
	FlushInterval: 5,                            // flush buffer every 5 seconds
	Level:         slog.LevelInfo,
	Format:        glog.FormatLine,
})
defer handler.Close()
```

### Trace injection

Use `TraceExtractor` to read trace data from `context` and add it to each log record:

```go
handler := glog.NewHandler(&glog.Options{
	TraceExtractor: glog.DefaultTraceExtractor,
})
logger := slog.New(handler)

ctx := context.WithValue(context.Background(), "trace_id", "trace-123")
ctx = context.WithValue(ctx, "span_id", "span-456")

logger.InfoContext(ctx, "with trace")
```

The default extractor looks for string values under these context keys: `trace_id` / `traceId` / `TraceID` / `TRACE_ID` and the corresponding `span_*` variants.

### Custom record handling

Use `RecordHandler` to add or change attributes before a record is written (e.g. app name, environment):

```go
handler := glog.NewHandler(&glog.Options{
	RecordHandler: func(ctx context.Context, r *slog.Record) {
		r.AddAttrs(
			slog.String("app", "demo"),
			slog.String("env", "dev"),
		)
	},
})
logger := slog.New(handler)
```

### Format options

`Options.Format` supports:

- `glog.FormatLine`: single-line text, e.g. `[2024-01-01 12:00:00] INFO: message {"key":"val"}` (default)
- `glog.FormatJSON`: uses `slog.NewJSONHandler`
- `glog.FormatText`: uses `slog.NewTextHandler`

### Tests and benchmarks

```bash
go test ./...
```

To run only benchmarks and see detailed ns/op, B/op, allocs/op output, use:

```bash
go test -run='^$' -bench=. -benchmem
```

Example benchmark comparison of glog vs logrus vs zap (darwin/arm64, Apple M1):

```
goos: darwin
goarch: arm64
pkg: github.com/lyuangg/glog
cpu: Apple M1
BenchmarkGlog_File-8                  28          40556146 ns/op         3842938 B/op      59764 allocs/op
BenchmarkGlog_File_Flush1s-8          73          15706089 ns/op         3842852 B/op      59768 allocs/op
BenchmarkGlog_FileJSON-8              28          40788869 ns/op         4967191 B/op      79777 allocs/op
BenchmarkLogrus_File-8                25          50671235 ns/op        14902521 B/op     259857 allocs/op
BenchmarkLogrus_FileJSON-8            21          53053474 ns/op        24210556 B/op     390007 allocs/op
BenchmarkZap_File-8                  814           1601175 ns/op         3885309 B/op      10264 allocs/op
BenchmarkZap_FileJSON-8              811           1445360 ns/op         3885427 B/op      10265 allocs/op
PASS
ok      github.com/lyuangg/glog 11.127s
```

## License

MIT License.