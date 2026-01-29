## glog

基于 Go 标准库 `log/slog` 封装的日志组件，提供：

- **多种输出格式**：单行文本、JSON、Text
- **文件轮转与保留策略**：按时间格式自动切换文件、最多保留 N 个历史文件
- **异步缓冲刷新**：支持定时 Flush，减少磁盘 IO
- **Trace 信息注入**：支持从 `context.Context` 提取 `trace_id` / `span_id`
- **自定义 Record 处理**

### 安装

```bash
go get github.com/lyuangg/glog
```

### 快速上手

创建一个使用单行文本格式的 `slog.Logger`：

```go
package main

import (
	"context"
	"log/slog"

	"github.com/lyuangg/glog"
)

func main() {
	handler := glog.NewHandler(&glog.Options{
		// 写入标准输出（Writer 为 nil 且 LogPath 为空时）
		Level:  slog.LevelInfo,
		Format: glog.FormatLine, // 单行文本格式
	})
	defer handler.Close()

	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "hello glog", slog.String("foo", "bar"))
}
```

### 写入文件并自动轮转

`Options.LogPath` 支持使用时间格式来控制文件名，如：

```go
handler := glog.NewHandler(&glog.Options{
	LogPath:       "logs/app-2006-01-02-15.log", // 按小时轮转
	MaxFiles:      24,                           // 最多保留 24 个文件
	FlushInterval: 5,                            // 每 5 秒刷新一次缓冲
	Level:         slog.LevelInfo,
	Format:        glog.FormatLine,
})
defer handler.Close()
```

### Trace 信息注入

可以通过 `TraceExtractor` 从 `context` 中提取跟踪信息并自动注入到日志字段中：

```go
handler := glog.NewHandler(&glog.Options{
	TraceExtractor: glog.DefaultTraceExtractor,
})
logger := slog.New(handler)

ctx := context.WithValue(context.Background(), "trace_id", "trace-123")
ctx = context.WithValue(ctx, "span_id", "span-456")

logger.InfoContext(ctx, "with trace")
```

默认会从以下 key 获取值（字符串）：`trace_id` / `traceId` / `TraceID` / `TRACE_ID` 以及对应的 `span_*`。

### 自定义 Record 处理

通过 `RecordHandler` 可以在日志写出前动态添加字段，例如统一追加应用名称、环境等：

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

### 日志格式选项

`Options.Format` 支持：

- `glog.FormatLine`：单行文本，形如 `[2024-01-01 12:00:00] INFO: message {"key":"val"}`（默认）
- `glog.FormatJSON`：使用 `slog.NewJSONHandler`
- `glog.FormatText`：使用 `slog.NewTextHandler`

### 测试与基准

```bash
go test ./...
```


```bash
go test -run='^$' -bench=. -benchmem
```

对比 `glog` 与 `logrus`、`zap` 等不同日志库性能结果示例： 

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

## 许可证

MIT License。
