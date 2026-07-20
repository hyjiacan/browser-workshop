package log

import (
	"bytes"
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
