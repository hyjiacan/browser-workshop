// Package log provides structured logging for bm.
// It supports separate log levels for file and console output.
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents the log level.
type Level int

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a log level string.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// color returns the ANSI color code for a log level.
func (l Level) color() string {
	switch l {
	case LevelTrace:
		return "\x1b[90m" // gray
	case LevelDebug:
		return "\x1b[36m" // cyan
	case LevelInfo:
		return "\x1b[32m" // green
	case LevelWarn:
		return "\x1b[33m" // yellow
	case LevelError:
		return "\x1b[31m" // red
	case LevelFatal:
		return "\x1b[35m" // magenta
	default:
		return ""
	}
}

const colorReset = "\x1b[0m"

// writerConfig holds configuration for a single output writer.
type writerConfig struct {
	writer    io.Writer
	level     Level
	useColor  bool
	showSrc   bool
	showTime  bool
}

// Logger is the main logger struct.
type Logger struct {
	mu      sync.Mutex
	writers []writerConfig
	file    *os.File
}

var (
	defaultLogger *Logger
	defaultOnce   sync.Once
)

// Default returns the default logger, initialized on first use.
// The default logger writes to stderr at INFO level.
func Default() *Logger {
	defaultOnce.Do(func() {
		defaultLogger = &Logger{
			writers: []writerConfig{
				{
					writer:   os.Stderr,
					level:    LevelInfo,
					useColor: isTerminal(os.Stderr),
					showSrc:  false,
					showTime: true,
				},
			},
		}
	})
	return defaultLogger
}

// SetDefault replaces the default logger with the given one.
// This affects all package-level log functions (Debug, Info, Warn, Error, etc.).
func SetDefault(l *Logger) {
	defaultLogger = l
	// Ensure defaultOnce is marked as done so Default() returns our logger.
	// We trigger the Once with a no-op if it hasn't been triggered yet,
	// but since we've already set defaultLogger directly, it won't be overwritten.
	defaultOnce.Do(func() {})
}

// New creates a new logger with a single writer at the given level.
func New(level Level, writer io.Writer) *Logger {
	return &Logger{
		writers: []writerConfig{
			{
				writer:   writer,
				level:    level,
				useColor: false,
				showSrc:  false,
				showTime: true,
			},
		},
	}
}

// NewDualLogger creates a logger that writes to both file and console with separate levels.
// fileLevel controls what goes to the file, consoleLevel controls what goes to stderr.
// consoleColor enables colored output on the console.
func NewDualLogger(logFile string, fileLevel Level, consoleLevel Level, consoleColor bool) (*Logger, error) {
	// Ensure log directory exists
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	// Open log file in append mode
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	logger := &Logger{
		file: f,
		writers: []writerConfig{
			{
				writer:   f,
				level:    fileLevel,
				useColor: false,
				showSrc:  true,
				showTime: true,
			},
			{
				writer:   os.Stderr,
				level:    consoleLevel,
				useColor: consoleColor && isTerminal(os.Stderr),
				showSrc:  false,
				showTime: false,
			},
		},
	}

	return logger, nil
}

// Close closes the log file if one is open.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetConsoleLevel sets the log level for the console (stderr) writer.
func (l *Logger) SetConsoleLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.writers {
		if l.writers[i].writer == os.Stderr {
			l.writers[i].level = level
			break
		}
	}
}

// SetFileLevel sets the log level for the file writer.
func (l *Logger) SetFileLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.writers {
		if l.writers[i].writer == l.file {
			l.writers[i].level = level
			break
		}
	}
}

// SetShowSource enables or disables showing source file and line in file output.
func (l *Logger) SetShowSource(show bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.writers {
		if l.writers[i].writer == l.file {
			l.writers[i].showSrc = show
			break
		}
	}
}

// AddWriter adds an additional writer with the given level.
func (l *Logger) AddWriter(w io.Writer, level Level, useColor bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writers = append(l.writers, writerConfig{
		writer:   w,
		level:    level,
		useColor: useColor,
		showSrc:  false,
		showTime: true,
	})
}

// Trace logs a trace message.
func (l *Logger) Trace(msg string, args ...interface{}) {
	l.log(LevelTrace, msg, args...)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message.
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(LevelFatal, msg, args...)
	os.Exit(1)
}

// log is the internal logging function.
func (l *Logger) log(level Level, msg string, args ...interface{}) {
	// Fast path: check if any writer would accept this level
	l.mu.Lock()
	anyAccepts := false
	for _, wc := range l.writers {
		if level >= wc.level {
			anyAccepts = true
			break
		}
	}
	if !anyAccepts {
		l.mu.Unlock()
		return
	}
	l.mu.Unlock()

	// Format message
	formatted := msg
	if len(args) > 0 {
		formatted = fmt.Sprintf(msg, args...)
	}

	// Get source info (once, for all writers that need it)
	var srcInfo string
	l.mu.Lock()
	needSrc := false
	for _, wc := range l.writers {
		if level >= wc.level && wc.showSrc {
			needSrc = true
			break
		}
	}
	l.mu.Unlock()

	if needSrc {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			shortFile := filepath.Base(file)
			srcInfo = fmt.Sprintf(" [%s:%d]", shortFile, line)
		}
	}

	now := time.Now().Format("2006-01-02 15:04:05.000")

	// Write to each writer
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, wc := range l.writers {
		if level < wc.level {
			continue
		}

		var line string
		if wc.useColor {
			if wc.showTime {
				line = fmt.Sprintf("%s%s [%s]%s %s%s\n",
					level.color(), now, level.String(), srcInfo, formatted, colorReset)
			} else {
				line = fmt.Sprintf("%s%s%s\n", level.color(), formatted, colorReset)
			}
		} else {
			if wc.showTime {
				line = fmt.Sprintf("%s [%s]%s %s\n", now, level.String(), srcInfo, formatted)
			} else {
				line = fmt.Sprintf("%s\n", formatted)
			}
		}

		fmt.Fprint(wc.writer, line)
	}
}

// isTerminal checks if a file is a terminal (supports colors).
func isTerminal(f *os.File) bool {
	// Simple heuristic: check if it's /dev/tty or CONIN$/CONOUT$ on Windows
	// In practice, we can use the file descriptor check
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	// On Unix, character devices have ModeCharDevice set
	// On Windows, this is less reliable, but we default to no color for safety
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// --- Convenience functions using the default logger ---

// Trace logs a trace message using the default logger.
func Trace(msg string, args ...interface{}) {
	Default().Trace(msg, args...)
}

// Debug logs a debug message using the default logger.
func Debug(msg string, args ...interface{}) {
	Default().Debug(msg, args...)
}

// Info logs an info message using the default logger.
func Info(msg string, args ...interface{}) {
	Default().Info(msg, args...)
}

// Warn logs a warning message using the default logger.
func Warn(msg string, args ...interface{}) {
	Default().Warn(msg, args...)
}

// Error logs an error message using the default logger.
func Error(msg string, args ...interface{}) {
	Default().Error(msg, args...)
}

// Fatal logs a fatal message using the default logger and exits.
func Fatal(msg string, args ...interface{}) {
	Default().Fatal(msg, args...)
}

// --- Progress logging ---

// ProgressLogger provides progress logging for long-running operations.
type ProgressLogger struct {
	logger    *Logger
	prefix    string
	total     int64
	current   int64
	lastPct   float64
	lastTime  time.Time
	interval  time.Duration // minimum time between progress logs
}

// NewProgressLogger creates a new progress logger.
func NewProgressLogger(logger *Logger, prefix string, total int64) *ProgressLogger {
	if logger == nil {
		logger = Default()
	}
	return &ProgressLogger{
		logger:   logger,
		prefix:   prefix,
		total:    total,
		interval: 2 * time.Second,
		lastTime: time.Now(),
	}
}

// Update updates the current progress and logs if enough time has passed.
func (p *ProgressLogger) Update(current int64) {
	p.current = current

	now := time.Now()
	if now.Sub(p.lastTime) < p.interval && current < p.total {
		return
	}
	p.lastTime = now

	var pct float64
	if p.total > 0 {
		pct = float64(current) / float64(p.total) * 100
	}

	// Only log if percentage changed significantly
	if p.total > 0 && pct-p.lastPct < 1 && current < p.total {
		return
	}
	p.lastPct = pct

	if p.total > 0 {
		p.logger.Info("%s: %.1f%% (%s / %s)", p.prefix, pct, formatSize(current), formatSize(p.total))
	} else {
		p.logger.Info("%s: %s", p.prefix, formatSize(current))
	}
}

// Done marks the progress as complete.
func (p *ProgressLogger) Done() {
	p.current = p.total
	if p.total > 0 {
		p.logger.Info("%s: 100%% 完成 (%s)", p.prefix, formatSize(p.total))
	} else {
		p.logger.Info("%s: 完成", p.prefix)
	}
}

// formatSize formats a byte size into a human-readable string.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
