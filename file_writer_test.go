package glog

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFileWriter_Write(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.log")

	testData := []byte("hello world\n")
	fw := NewFileWriter(filePath, 0)
	n, err := fw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write returned %d, expected %d", n, len(testData))
	}

	time.Sleep(1200 * time.Millisecond)

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("file content mismatch: got %q, expected %q", string(content), string(testData))
	}

	moreData := []byte("second line\n")
	n, err = fw.Write(moreData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(moreData) {
		t.Errorf("Write returned %d, expected %d", n, len(moreData))
	}

	time.Sleep(1200 * time.Millisecond)

	content, err = os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expected := string(testData) + string(moreData)
	if string(content) != expected {
		t.Errorf("file content mismatch: got %q, expected %q", string(content), expected)
	}

	if err := fw.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestFileWriter_RotateFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	timeFormat := filepath.Join(tmpDir, "test-2006-01-02-15-04-05.log")
	fw := NewFileWriter(timeFormat, 0)

	testData1 := []byte("first write\n")
	_, err = fw.Write(testData1)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	firstFile := fw.current
	if firstFile == "" {
		t.Fatal("current file should not be empty")
	}

	time.Sleep(2 * time.Second)

	testData2 := []byte("second write\n")
	_, err = fw.Write(testData2)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	secondFile := fw.current
	if firstFile == secondFile {
		t.Error("file should have rotated, but got same filename")
	}

	content1, err := os.ReadFile(firstFile)
	if err != nil {
		t.Fatalf("failed to read first file: %v", err)
	}
	if string(content1) != string(testData1) {
		t.Errorf("first file content mismatch: got %q, expected %q", string(content1), string(testData1))
	}

	content2, err := os.ReadFile(secondFile)
	if err != nil {
		t.Fatalf("failed to read second file: %v", err)
	}
	if string(content2) != string(testData2) {
		t.Errorf("second file content mismatch: got %q, expected %q", string(content2), string(testData2))
	}

	if err := fw.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestFileWriter_ConcurrentWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "concurrent_test.log")
	fw := NewFileWriter(filePath, 0)
	defer fw.Close()

	const numGoroutines = 100
	const writesPerGoroutine = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*writesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				data := []byte(strings.Repeat("x", 100) + "\n")
				_, err := fw.Write(data)
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expectedLines := numGoroutines * writesPerGoroutine
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != expectedLines {
		t.Errorf("expected %d lines, got %d", expectedLines, len(lines))
	}
}

func TestFileWriter_ConcurrentWriteWithRotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	timeFormat := filepath.Join(tmpDir, "rotate-2006-01-02-15-04-05.log")
	fw := NewFileWriter(timeFormat, 0)
	defer fw.Close()

	const numGoroutines = 50
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*10)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				data := []byte(strings.Repeat("x", 50) + "\n")
				_, err := fw.Write(data)
				if err != nil {
					errors <- err
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent write with rotation error: %v", err)
	}
}

func TestFileWriter_Close(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "close_test.log")
	fw := NewFileWriter(filePath, 0)

	testData := []byte("test data\n")
	_, err = fw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := fw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if err := fw.Close(); err != nil {
		t.Errorf("second Close should not fail: %v", err)
	}

	_, err = fw.Write([]byte("after close\n"))
	if err != nil {
		t.Errorf("Write after Close should succeed (reopens file): %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(content), "after close") {
		t.Error("file should contain data written after close")
	}
}

func TestFileWriter_CloseConcurrent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "close_concurrent_test.log")
	fw := NewFileWriter(filePath, 0)

	const numGoroutines = 20
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, _ = fw.Write([]byte("test\n"))
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		_ = fw.Close()
	}()

	wg.Wait()
}

func TestFileWriter_EmptyWrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "empty_test.log")
	fw := NewFileWriter(filePath, 0)
	defer fw.Close()

	n, err := fw.Write([]byte{})
	if err != nil {
		t.Fatalf("Write empty data failed: %v", err)
	}
	if n != 0 {
		t.Errorf("Write empty data returned %d, expected 0", n)
	}
}

func TestFileWriter_MultipleRotations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	timeFormat := filepath.Join(tmpDir, "multi-2006-01-02-15-04-05.log")
	fw := NewFileWriter(timeFormat, 0)
	defer fw.Close()

	files := make(map[string]bool)

	for i := 0; i < 3; i++ {
		data := []byte(strings.Repeat("x", 10) + "\n")
		_, err := fw.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if fw.current != "" {
			files[fw.current] = true
		}

		if i < 2 {
			time.Sleep(2 * time.Second)
		}
	}

	if len(files) < 1 {
		t.Error("expected at least one file to be created")
	}

	for file := range files {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("file %s should exist: %v", file, err)
		}
	}
}

func TestFileWriter_DirectoryNotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nonExistentDir := filepath.Join(tmpDir, "nonexistent", "subdir", "test.log")
	fw := NewFileWriter(nonExistentDir, 0)

	testData := []byte("test data\n")
	_, err = fw.Write(testData)
	if err == nil {
		t.Error("Write should fail when directory does not exist")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected directory not exist error, got: %v", err)
	}
}

func TestFileWriter_WithSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "logs", "app")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	filePath := filepath.Join(subDir, "app.log")
	fw := NewFileWriter(filePath, 0)
	defer fw.Close()

	testData := []byte("log message\n")
	n, err := fw.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write returned %d, expected %d", n, len(testData))
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("file content mismatch: got %q, expected %q", string(content), string(testData))
	}
}

func TestFileWriter_CleanOldFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glog_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	timeFormat := filepath.Join(tmpDir, "clean-2006-01-02-15-04-05.log")
	fw := NewFileWriter(timeFormat, 3)
	defer fw.Close()

	for i := 0; i < 5; i++ {
		data := []byte(strings.Repeat("x", 10) + "\n")
		_, err := fw.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if i < 4 {
			time.Sleep(2 * time.Second)
		}
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	var fileCount int
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "clean-") {
			fileCount++
		}
	}

	if fileCount > 4 {
		t.Errorf("expected at most 4 files (3 old + 1 current), got %d", fileCount)
	}
	if fileCount < 3 {
		t.Errorf("expected at least 3 files, got %d", fileCount)
	}
}
