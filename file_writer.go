package glog

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileWriter struct {
	mu            sync.Mutex
	path          string
	dir           string
	fileName      string
	current       string
	file          *os.File
	buf           *bufio.Writer
	maxFiles      int           // max old files to keep; 0 = no limit
	flushInterval time.Duration // flush interval in seconds; 0 = flush on every write

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func NewFileWriter(path string, maxFiles int) *FileWriter {
	return NewFileWriterWithFlushInterval(path, maxFiles, 0)
}

func NewFileWriterWithFlushInterval(path string, maxFiles int, flushIntervalSeconds int) *FileWriter {
	ctx, cancel := context.WithCancel(context.Background())
	fw := &FileWriter{
		path:          path,
		dir:           filepath.Dir(path),
		fileName:      filepath.Base(path),
		maxFiles:      maxFiles,
		flushInterval: time.Duration(flushIntervalSeconds) * time.Second,
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}

	// open initial file
	fw.checkAndRotate()

	// start async rotation loop
	go fw.rotateLoop()

	return fw
}

func (f *FileWriter) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// if file is not open (e.g. after Close), try to reopen current file
	if f.file == nil {
		if err := f.openCurrentLocked(); err != nil {
			return 0, err
		}
	}

	// no flushInterval: write directly to file, no bufio
	if f.flushInterval == 0 {
		return f.file.Write(p)
	}

	// with flushInterval: use buffered write
	if f.buf == nil {
		f.buf = bufio.NewWriter(f.file)
	}
	n, err = f.buf.Write(p)
	return n, err
}

func (f *FileWriter) Close() error {
	// stop async rotation goroutine
	f.cancel()
	<-f.done

	f.mu.Lock()
	defer f.mu.Unlock()

	// flush buffer
	if f.buf != nil {
		if err := f.buf.Flush(); err != nil {
			return err
		}
		f.buf = nil
	}

	if f.file != nil {
		if err := f.file.Close(); err != nil {
			return err
		}
		f.file = nil
	}
	return nil
}

// rotateLoop runs the async rotation loop.
func (f *FileWriter) rotateLoop() {
	defer close(f.done)

	checkInterval := f.getCheckInterval()
	rotateTicker := time.NewTicker(checkInterval)
	defer rotateTicker.Stop()

	// if flush interval is set, use a ticker to flush
	var flushTicker *time.Ticker
	var flushChan <-chan time.Time
	if f.flushInterval > 0 {
		flushTicker = time.NewTicker(f.flushInterval)
		defer flushTicker.Stop()
		flushChan = flushTicker.C
	}

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-rotateTicker.C:
			f.checkAndRotate()
		case <-flushChan:
			f.flushBuffer()
		}
	}
}

func (f *FileWriter) flushBuffer() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.buf != nil {
		_ = f.buf.Flush() // ignore error; does not affect writes
	}
}

// getCheckInterval returns the rotation check interval based on the filename layout.
func (f *FileWriter) getCheckInterval() time.Duration {
	fileName := f.fileName
	if strings.Contains(fileName, "05") || strings.Contains(fileName, "5") {
		return time.Second
	}
	return time.Minute
}

// checkAndRotate checks and performs file rotation if needed.
func (f *FileWriter) checkAndRotate() {
	f.mu.Lock()
	defer f.mu.Unlock()

	formattedFileName := time.Now().Format(f.fileName)
	current := filepath.Join(f.dir, formattedFileName)

	if current != f.current {
		if f.buf != nil {
			if err := f.buf.Flush(); err != nil {
				return
			}
			f.buf = nil
		}

		if f.file != nil {
			if err := f.file.Close(); err != nil {
				return
			}
			f.file = nil
		}
		f.current = current
		if err := f.openCurrentLocked(); err != nil {
			return
		}

		if f.maxFiles > 0 {
			_ = f.cleanOldFiles()
		}
	}
}

// openCurrentLocked opens the file at f.current and initializes the buffer. Caller must hold f.mu.
func (f *FileWriter) openCurrentLocked() error {
	file, err := os.OpenFile(f.current, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		f.file = nil
		f.buf = nil
		return err
	}

	f.file = file
	if f.flushInterval > 0 {
		f.buf = bufio.NewWriter(f.file)
	} else {
		f.buf = nil
	}
	return nil
}

// cleanOldFiles removes old files beyond maxFiles. Caller must hold f.mu.
func (f *FileWriter) cleanOldFiles() error {
	if f.maxFiles <= 0 {
		return nil
	}

	matches, err := filepath.Glob(f.buildGlobPattern())
	if err != nil {
		return err
	}

	var files []struct {
		name    string
		modTime time.Time
	}
	for _, match := range matches {
		if match == f.current {
			continue
		}
		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, struct {
			name    string
			modTime time.Time
		}{
			name:    match,
			modTime: info.ModTime(),
		})
	}

	if len(files) <= f.maxFiles {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	for i := f.maxFiles; i < len(files); i++ {
		if err := os.Remove(files[i].name); err != nil {
			return err
		}
	}

	return nil
}

func (f *FileWriter) buildGlobPattern() string {
	// replace time placeholders (2006, 06, 01-05, 15, etc.) with * and collapse runs
	pattern := regexp.MustCompile(`2006|0[1-6]|[1-5]|15`).ReplaceAllString(f.fileName, "*")
	pattern = regexp.MustCompile(`\*+`).ReplaceAllString(pattern, "*")
	return filepath.Join(f.dir, pattern)
}
