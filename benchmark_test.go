package glog

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	benchmarkLogCount = 10000
	benchmarkMessage  = "This is a benchmark log message"
)

// BenchmarkGlog_File benchmarks glog writing to file.
func BenchmarkGlog_File(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "glog-2006-01-02-15.log")

	opts := &Options{
		LogPath:   logPath,
		MaxFiles:  0,
		Level:     slog.LevelInfo,
		Format:    FormatText,
		AddSource: false,
	}
	handler := NewHandler(opts)
	defer handler.Close()
	logger := slog.New(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < benchmarkLogCount; i++ {
				logger.Info(benchmarkMessage,
					"iteration", i,
					"timestamp", time.Now().UnixNano(),
					"key1", "value1",
					"key2", "value2",
					"key3", 123,
					"key4", true,
				)
			}
		}
	})
}

// BenchmarkGlog_File_Flush1s benchmarks glog writing to file with 1s buffer flush.
func BenchmarkGlog_File_Flush1s(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "glog-flush1s-2006-01-02-15.log")

	opts := &Options{
		LogPath:       logPath,
		MaxFiles:      0,
		Level:         slog.LevelInfo,
		Format:        FormatText,
		AddSource:     false,
		FlushInterval: 1,
	}
	handler := NewHandler(opts)
	defer handler.Close()
	logger := slog.New(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < benchmarkLogCount; i++ {
				logger.Info(benchmarkMessage,
					"iteration", i,
					"timestamp", time.Now().UnixNano(),
					"key1", "value1",
					"key2", "value2",
					"key3", 123,
					"key4", true,
				)
			}
		}
	})
}

// BenchmarkGlog_FileJSON benchmarks glog JSON format writing to file.
func BenchmarkGlog_FileJSON(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "glog-json-2006-01-02-15.log")

	opts := &Options{
		LogPath:   logPath,
		MaxFiles:  0,
		Level:     slog.LevelInfo,
		Format:    FormatJSON,
		AddSource: false,
	}
	handler := NewHandler(opts)
	defer handler.Close()
	logger := slog.New(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < benchmarkLogCount; i++ {
				logger.Info(benchmarkMessage,
					"iteration", i,
					"timestamp", time.Now().UnixNano(),
					"key1", "value1",
					"key2", "value2",
					"key3", 123,
					"key4", true,
				)
			}
		}
	})
}

// BenchmarkLogrus_File benchmarks logrus writing to file.
func BenchmarkLogrus_File(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "logrus.log")

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		b.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	logger := logrus.New()
	logger.SetOutput(file)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: false,
		FullTimestamp:    true,
	})
	logger.SetLevel(logrus.InfoLevel)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < benchmarkLogCount; i++ {
				logger.WithFields(logrus.Fields{
					"iteration": i,
					"timestamp": time.Now().UnixNano(),
					"key1":      "value1",
					"key2":      "value2",
					"key3":      123,
					"key4":      true,
				}).Info(benchmarkMessage)
			}
		}
	})
}

// BenchmarkLogrus_FileJSON benchmarks logrus JSON format writing to file.
func BenchmarkLogrus_FileJSON(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "logrus-json.log")

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		b.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	logger := logrus.New()
	logger.SetOutput(file)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < benchmarkLogCount; i++ {
				logger.WithFields(logrus.Fields{
					"iteration": i,
					"timestamp": time.Now().UnixNano(),
					"key1":      "value1",
					"key2":      "value2",
					"key3":      123,
					"key4":      true,
				}).Info(benchmarkMessage)
			}
		}
	})
}

// BenchmarkZap_File benchmarks zap writing to file.
func BenchmarkZap_File(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "zap.log")

	config := zap.NewProductionConfig()
	config.OutputPaths = []string{logPath}
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		b.Fatalf("Failed to build zap logger: %v", err)
	}
	defer logger.Sync()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < benchmarkLogCount; i++ {
				logger.Info(benchmarkMessage,
					zap.Int("iteration", i),
					zap.Int64("timestamp", time.Now().UnixNano()),
					zap.String("key1", "value1"),
					zap.String("key2", "value2"),
					zap.Int("key3", 123),
					zap.Bool("key4", true),
				)
			}
		}
	})
}

// BenchmarkZap_FileJSON benchmarks zap JSON format writing to file (zap default is JSON).
func BenchmarkZap_FileJSON(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "zap-json.log")

	config := zap.NewProductionConfig()
	config.OutputPaths = []string{logPath}
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		b.Fatalf("Failed to build zap logger: %v", err)
	}
	defer logger.Sync()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < benchmarkLogCount; i++ {
				logger.Info(benchmarkMessage,
					zap.Int("iteration", i),
					zap.Int64("timestamp", time.Now().UnixNano()),
					zap.String("key1", "value1"),
					zap.String("key2", "value2"),
					zap.Int("key3", 123),
					zap.Bool("key4", true),
				)
			}
		}
	})
}
