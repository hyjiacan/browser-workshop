package log

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLogger_Basic(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelDebug, &buf)

	logger.Info("hello %s", "world")
	logger.Debug("debug message")
	logger.Warn("warning message")
	logger.Error("error message")

	output := buf.String()

	if !strings.Contains(output, "INFO") {
		t.Error("expected INFO level in output")
	}
	if !strings.Contains(output, "DEBUG") {
		t.Error("expected DEBUG level in output")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("expected WARN level in output")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("expected ERROR level in output")
	}
	if !strings.Contains(output, "hello world") {
		t.Error("expected formatted message")
	}
}

func TestLogger_TraceLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelTrace, &buf)

	logger.Trace("trace message")
	output := buf.String()

	if !strings.Contains(output, "TRACE") {
		t.Error("expected TRACE level in output")
	}
	if !strings.Contains(output, "trace message") {
		t.Error("expected trace message content")
	}
}

func TestLogger_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelWarn, &buf)

	logger.Info("info message")
	logger.Debug("debug message")
	logger.Trace("trace message")
	logger.Warn("warning message")

	output := buf.String()

	if strings.Contains(output, "INFO") {
		t.Error("INFO should be filtered out at WARN level")
	}
	if strings.Contains(output, "DEBUG") {
		t.Error("DEBUG should be filtered out at WARN level")
	}
	if strings.Contains(output, "TRACE") {
		t.Error("TRACE should be filtered out at WARN level")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("WARN should be present at WARN level")
	}
}

func TestLogger_SetConsoleLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelError, &buf)

	logger.Warn("should not appear")
	if strings.Contains(buf.String(), "WARN") {
		t.Error("WARN should not appear at ERROR level")
	}

	// Since we created the logger with New(), add a console-level writer
	// and test that SetConsoleLevel works when os.Stderr is present.
	// For the test, we just verify the SetConsoleLevel method exists and doesn't panic.
	logger.SetConsoleLevel(LevelInfo)
	logger.Info("should appear with NewDualLogger")

	// For a proper test, use NewDualLogger which has a stderr writer
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	dl, err := NewDualLogger(logFile, LevelDebug, LevelError, false)
	if err != nil {
		t.Fatalf("NewDualLogger failed: %v", err)
	}
	defer dl.Close()

	// At LevelError console level, Warn should not appear on stderr
	// (we can't easily capture stderr, but we can verify SetConsoleLevel doesn't error)
	dl.SetConsoleLevel(LevelInfo)
	dl.Info("console level test")
}

func TestLogger_DualOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewDualLogger(logFile, LevelDebug, LevelWarn, false)
	if err != nil {
		t.Fatalf("NewDualLogger failed: %v", err)
	}
	defer logger.Close()

	// This should go to file but not to stderr
	logger.Info("info to file only")
	// This should go to both
	logger.Warn("warn to both")
	logger.Close()

	// Check file has both
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "info to file only") {
		t.Error("log file should contain info message")
	}
	if !strings.Contains(content, "warn to both") {
		t.Error("log file should contain warn message")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"trace", LevelTrace},
		{"TRACE", LevelTrace},
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"fatal", LevelFatal},
		{"unknown", LevelInfo}, // default
	}

	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.expected {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelTrace, "TRACE"},
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
	}

	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.expected {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestProgressLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(LevelDebug, &buf)

	pl := NewProgressLogger(logger, "test", 1000)
	pl.interval = 0 // log every update for testing

	pl.Update(500)
	if !strings.Contains(buf.String(), "50.0%") {
		t.Error("expected 50% in progress log")
	}

	pl.Update(1000)
	pl.Done()
	if !strings.Contains(buf.String(), "100%") {
		t.Error("expected 100% in progress log")
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := Default()
	if logger == nil {
		t.Fatal("Default() returned nil")
	}
	// Default logger should be the same instance
	logger2 := Default()
	if logger != logger2 {
		t.Error("Default() should return the same instance")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	var buf bytes.Buffer
	old := Default()
	defer func() {
		// Reset default logger
		defaultLogger = old
		defaultOnce = sync.Once{}
	}()

	// Replace default logger
	defaultLogger = New(LevelTrace, &buf)

	Trace("trace test")
	Debug("debug test")
	Info("info test")
	Warn("warn test")
	Error("error test")

	output := buf.String()
	if !strings.Contains(output, "trace test") {
		t.Error("Trace() should log to default logger")
	}
	if !strings.Contains(output, "debug test") {
		t.Error("Debug() should log to default logger")
	}
	if !strings.Contains(output, "info test") {
		t.Error("Info() should log to default logger")
	}
	if !strings.Contains(output, "warn test") {
		t.Error("Warn() should log to default logger")
	}
	if !strings.Contains(output, "error test") {
		t.Error("Error() should log to default logger")
	}
}

func TestAddWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger := New(LevelInfo, &buf1)

	logger.Info("before add")
	if strings.Contains(buf2.String(), "before add") {
		t.Error("buf2 should not have message before AddWriter")
	}

	logger.AddWriter(&buf2, LevelInfo, false)
	logger.Info("after add")

	if !strings.Contains(buf1.String(), "after add") {
		t.Error("buf1 should have message after AddWriter")
	}
	if !strings.Contains(buf2.String(), "after add") {
		t.Error("buf2 should have message after AddWriter")
	}
}

func TestLogger_ShowSource(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewDualLogger(logFile, LevelDebug, LevelInfo, false)
	if err != nil {
		t.Fatalf("NewDualLogger failed: %v", err)
	}
	defer logger.Close()

	logger.SetShowSource(true)
	logger.Info("source test")
	logger.Close()

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)

	// File output should have source info
	if !strings.Contains(content, "log_test.go") {
		t.Errorf("log file should contain source file name, got: %s", content)
	}
}

// --- RotatingFileWriter tests ---

func TestRotatingFileWriter_BasicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	w, err := NewRotatingFileWriter(logFile, 1024, 3)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}
	defer w.Close()

	msg := []byte("hello world\n")
	n, err := w.Write(msg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned %d, want %d", n, len(msg))
	}

	// Verify file content
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("file content = %q, want %q", string(data), "hello world\n")
	}
}

func TestRotatingFileWriter_NoRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// maxSize = 0 means no rotation
	w, err := NewRotatingFileWriter(logFile, 0, 3)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}
	defer w.Close()

	// Write a lot of data
	for i := 0; i < 100; i++ {
		msg := []byte("this is a test line that should not cause rotation because max size is zero\n")
		_, err := w.Write(msg)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// No backup files should exist
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
	if entries[0].Name() != "test.log" {
		t.Errorf("expected test.log, got %s", entries[0].Name())
	}
}

func TestRotatingFileWriter_SingleRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// 100 bytes max size, 3 backups
	w, err := NewRotatingFileWriter(logFile, 100, 3)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}
	defer w.Close()

	// Write data that fits within the limit
	firstMsg := []byte("first message\n") // 15 bytes
	_, err = w.Write(firstMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Write data that exceeds the limit, triggering rotation
	secondMsg := []byte("this is a much longer message that will definitely exceed the one hundred byte limit for our test\n")
	_, err = w.Write(secondMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Check that .1 backup exists
	backupFile := logFile + ".1"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("expected backup file .1 to exist")
	}

	// Check that main log has the second message
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "this is a much longer message") {
		t.Errorf("main log should contain second message, got: %s", string(data))
	}

	// Check that backup has the first message
	backupData, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("ReadFile backup failed: %v", err)
	}
	if !strings.Contains(string(backupData), "first message") {
		t.Errorf("backup log should contain first message, got: %s", string(backupData))
	}
}

func TestRotatingFileWriter_MultipleRotations(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// 50 bytes max size, 3 backups
	w, err := NewRotatingFileWriter(logFile, 50, 3)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}
	defer w.Close()

	// Write multiple messages, each triggering a rotation
	messages := []string{
		"message 1: first content\n",
		"message 2: second content\n",
		"message 3: third content\n",
		"message 4: fourth content\n",
		"message 5: fifth content\n",
	}

	for _, msg := range messages {
		_, err := w.Write([]byte(msg))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Should have: test.log (msg5), test.log.1 (msg4), test.log.2 (msg3), test.log.3 (msg2)
	// msg1 should have been deleted (exceeded maxBackups=3)

	// Check file count
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 4 {
		t.Errorf("expected 4 files (1 main + 3 backups), got %d", len(entries))
		for _, e := range entries {
			t.Logf("  file: %s", e.Name())
		}
	}

	// Verify content of each file
	mainData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile main failed: %v", err)
	}
	if !strings.Contains(string(mainData), "message 5") {
		t.Errorf("main log should contain message 5, got: %s", string(mainData))
	}

	data1, err := os.ReadFile(logFile + ".1")
	if err != nil {
		t.Fatalf("ReadFile .1 failed: %v", err)
	}
	if !strings.Contains(string(data1), "message 4") {
		t.Errorf(".1 log should contain message 4, got: %s", string(data1))
	}

	data2, err := os.ReadFile(logFile + ".2")
	if err != nil {
		t.Fatalf("ReadFile .2 failed: %v", err)
	}
	if !strings.Contains(string(data2), "message 3") {
		t.Errorf(".2 log should contain message 3, got: %s", string(data2))
	}

	data3, err := os.ReadFile(logFile + ".3")
	if err != nil {
		t.Fatalf("ReadFile .3 failed: %v", err)
	}
	if !strings.Contains(string(data3), "message 2") {
		t.Errorf(".3 log should contain message 2, got: %s", string(data3))
	}

	// message 1 should be gone
	if _, err := os.Stat(logFile + ".4"); !os.IsNotExist(err) {
		t.Error(".4 backup should not exist (exceeds maxBackups)")
	}
}

func TestRotatingFileWriter_ZeroBackups(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// 50 bytes max size, 0 backups (just truncate on rotation)
	w, err := NewRotatingFileWriter(logFile, 50, 0)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}
	defer w.Close()

	firstMsg := []byte("first message that will be lost on rotation\n")
	_, err = w.Write(firstMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	secondMsg := []byte("second message that triggers rotation\n")
	_, err = w.Write(secondMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// No backup files should exist
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file (no backups), got %d", len(entries))
		for _, e := range entries {
			t.Logf("  file: %s", e.Name())
		}
	}

	// Main file should only have the second message
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if strings.Contains(string(data), "first message") {
		t.Errorf("main log should not contain first message (should have been truncated), got: %s", string(data))
	}
	if !strings.Contains(string(data), "second message") {
		t.Errorf("main log should contain second message, got: %s", string(data))
	}
}

func TestRotatingFileWriter_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Pre-create a file with some content
	initialContent := "pre-existing log content line 1\npre-existing log content line 2\n"
	err := os.WriteFile(logFile, []byte(initialContent), 0o644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Open with rotating writer - should pick up current size
	w, err := NewRotatingFileWriter(logFile, 100, 3)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}
	defer w.Close()

	// Write a small message that fits within remaining space
	smallMsg := []byte("small\n")
	_, err = w.Write(smallMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should not have rotated yet (file still within size limit)
	if _, err := os.Stat(logFile + ".1"); !os.IsNotExist(err) {
		t.Error("backup file should not exist yet")
	}

	// Now write something that pushes past the limit
	largeMsg := []byte("this is a much larger message that will push past the size limit and trigger rotation\n")
	_, err = w.Write(largeMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should have rotated now
	if _, err := os.Stat(logFile + ".1"); os.IsNotExist(err) {
		t.Error("backup file .1 should exist after rotation")
	}

	// Backup should contain the initial content + small message
	backupData, err := os.ReadFile(logFile + ".1")
	if err != nil {
		t.Fatalf("ReadFile backup failed: %v", err)
	}
	if !strings.Contains(string(backupData), "pre-existing") {
		t.Errorf("backup should contain pre-existing content, got: %s", string(backupData))
	}
	if !strings.Contains(string(backupData), "small") {
		t.Errorf("backup should contain small message, got: %s", string(backupData))
	}
}

func TestRotatingFileWriter_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	w, err := NewRotatingFileWriter(logFile, 1024, 3)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}

	// Write something
	_, err = w.Write([]byte("test\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Close
	err = w.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Writing after close should fail
	_, err = w.Write([]byte("after close\n"))
	if err == nil {
		t.Error("Write after close should return error")
	}
}

func TestRotatingFileWriter_ConcurrentWrite(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Use a large enough maxSize that rotation won't happen during the test.
	// We're testing concurrency safety, not rotation correctness.
	maxSize := int64(1024 * 1024) // 1MB
	w, err := NewRotatingFileWriter(logFile, maxSize, 3)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter failed: %v", err)
	}
	defer w.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	numWrites := 100

	// Track expected unique IDs
	type msgID struct {
		goroutine int
		message   int
	}
	expected := make(map[msgID]bool)
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numWrites; j++ {
			expected[msgID{i, j}] = true
		}
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				msg := fmt.Sprintf("goroutine %d message %d\n", id, j)
				_, err := w.Write([]byte(msg))
				if err != nil {
					t.Errorf("Write failed in goroutine %d: %v", id, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	w.Close()

	// Read the main log file and verify all messages are present
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	content := string(data)

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != numGoroutines*numWrites {
		t.Errorf("total lines = %d, want %d", len(lines), numGoroutines*numWrites)
	}

	// Verify all expected messages are present
	found := make(map[msgID]bool)
	for _, line := range lines {
		var gid, mid int
		n, _ := fmt.Sscanf(line, "goroutine %d message %d", &gid, &mid)
		if n == 2 {
			found[msgID{gid, mid}] = true
		}
	}

	for id := range expected {
		if !found[id] {
			t.Errorf("missing message: goroutine %d message %d", id.goroutine, id.message)
		}
	}
	if len(found) != len(expected) {
		t.Errorf("found %d unique messages, want %d", len(found), len(expected))
	}
}

func TestNewDualLogger_WithRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Create dual logger with rotation
	logger, err := NewDualLogger(logFile, LevelDebug, LevelInfo, false,
		WithMaxSize(100),
		WithMaxBackups(3),
	)
	if err != nil {
		t.Fatalf("NewDualLogger with rotation failed: %v", err)
	}
	defer logger.Close()

	// Write enough to trigger rotation
	for i := 0; i < 10; i++ {
		logger.Info("this is a test message that is long enough to eventually trigger log file rotation number %d", i)
	}
	logger.Close()

	// Verify backup files exist
	if _, err := os.Stat(logFile + ".1"); os.IsNotExist(err) {
		t.Error("expected backup file .1 to exist after rotation")
	}

	// Verify main log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("expected main log file to exist")
	}
}

func TestNewDualLogger_SetFileLevelWithRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewDualLogger(logFile, LevelWarn, LevelInfo, false,
		WithMaxSize(1024),
		WithMaxBackups(3),
	)
	if err != nil {
		t.Fatalf("NewDualLogger failed: %v", err)
	}
	defer logger.Close()

	// At Warn level, Info should not be written to file
	logger.Info("should not appear in file")
	logger.Warn("should appear in file")
	logger.Close()

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "should not appear") {
		t.Error("file should not contain info-level message at warn level")
	}
	if !strings.Contains(content, "should appear") {
		t.Error("file should contain warn-level message")
	}
}

func TestClose_DefaultLogger(t *testing.T) {
	// Default logger has no file, so Close should be a no-op without error
	err := Close()
	if err != nil {
		t.Errorf("Close() on default logger should return nil, got: %v", err)
	}
}
