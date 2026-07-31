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

	"github.com/bws/bws/internal/util"
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
	writer   io.Writer
	level    Level
	useColor bool
	showSrc  bool
	showTime bool
	isFile   bool // whether this writer is the file writer (for SetFileLevel etc.)
}

// Logger is the main logger struct.
type Logger struct {
	mu       sync.Mutex
	writers  []writerConfig
	file     io.WriteCloser // the file writer (could be *os.File or *RotatingFileWriter), used for Close
	filePath string         // path of the log file, used to identify the file writer
}

var (
	defaultMu     sync.RWMutex
	defaultLogger *Logger
	defaultOnce   sync.Once
)

// Default returns the default logger, initialized on first use.
// The default logger writes to stderr at INFO level.
func Default() *Logger {
	defaultOnce.Do(func() {
		defaultMu.Lock()
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
		defaultMu.Unlock()
	})
	defaultMu.RLock()
	l := defaultLogger
	defaultMu.RUnlock()
	return l
}

// SetDefault replaces the default logger with the given one.
// This affects all package-level log functions (Debug, Info, Warn, Error, etc.).
func SetDefault(l *Logger) {
	defaultMu.Lock()
	defaultLogger = l
	defaultMu.Unlock()
	// Ensure defaultOnce is marked as done so Default() doesn't overwrite our logger.
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

// DualLoggerOption is a function that configures the dual logger.
type DualLoggerOption func(*dualLoggerConfig)

// dualLoggerConfig holds optional configuration for NewDualLogger.
type dualLoggerConfig struct {
	maxSize    int64 // max bytes before rotation, 0 = no rotation
	maxBackups int   // max number of backup files, 0 = no backups
	showSrc    bool  // whether to show source file/line in file output
}

// WithMaxSize sets the maximum log file size in bytes before rotation.
// 0 means no rotation (infinite append).
func WithMaxSize(maxSize int64) DualLoggerOption {
	return func(c *dualLoggerConfig) {
		c.maxSize = maxSize
	}
}

// WithMaxBackups sets the maximum number of backup log files to keep.
// 0 means no backups are kept when rotating.
func WithMaxBackups(maxBackups int) DualLoggerOption {
	return func(c *dualLoggerConfig) {
		c.maxBackups = maxBackups
	}
}

// WithShowSource controls whether source file and line info is shown in file output.
// Default is true for file output.
func WithShowSource(show bool) DualLoggerOption {
	return func(c *dualLoggerConfig) {
		c.showSrc = show
	}
}

// NewDualLogger creates a logger that writes to both file and console with separate levels.
// fileLevel controls what goes to the file, consoleLevel controls what goes to stderr.
// consoleColor enables colored output on the console.
// Use DualLoggerOption functions to configure optional settings like log rotation.
func NewDualLogger(logFile string, fileLevel Level, consoleLevel Level, consoleColor bool, opts ...DualLoggerOption) (*Logger, error) {
	// Apply default options
	cfg := &dualLoggerConfig{
		maxSize:    0,
		maxBackups: 0,
		showSrc:    true,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	var fileWriter io.WriteCloser
	var writer io.Writer

	if cfg.maxSize > 0 {
		// Use rotating file writer
		rfw, err := NewRotatingFileWriter(logFile, cfg.maxSize, cfg.maxBackups)
		if err != nil {
			return nil, fmt.Errorf("创建轮转日志文件失败: %w", err)
		}
		fileWriter = rfw
		writer = rfw
	} else {
		// Use plain file
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		fileWriter = f
		writer = f
	}

	logger := &Logger{
		file:     fileWriter,
		filePath: logFile,
		writers: []writerConfig{
			{
				writer:   writer,
				level:    fileLevel,
				useColor: false,
				showSrc:  cfg.showSrc,
				showTime: true,
				isFile:   true,
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

// Close closes the default logger.
func Close() error {
	return Default().Close()
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
		if l.writers[i].isFile {
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
		if l.writers[i].isFile {
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

// --- RotatingFileWriter ---

// RotatingFileWriter writes to a log file and rotates it when it reaches a size limit.
// Old log files are kept as .1, .2, .3, etc.
type RotatingFileWriter struct {
	path        string
	maxSize     int64 // max bytes before rotation
	maxBackups  int   // max number of backup files to keep
	file        *os.File
	currentSize int64
	mu          sync.Mutex
}

// NewRotatingFileWriter creates a new RotatingFileWriter.
// path is the path to the log file.
// maxSize is the maximum size in bytes before rotation (0 = no rotation).
// maxBackups is the maximum number of backup files to keep (0 = no backups).
func NewRotatingFileWriter(path string, maxSize int64, maxBackups int) (*RotatingFileWriter, error) {
	w := &RotatingFileWriter{
		path:       path,
		maxSize:    maxSize,
		maxBackups: maxBackups,
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// Open or create the file
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}
	w.file = f

	// Get current file size
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("获取日志文件信息失败: %w", err)
	}
	w.currentSize = fi.Size()

	return w, nil
}

// Write writes data to the log file, rotating if necessary.
func (w *RotatingFileWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return 0, fmt.Errorf("日志文件已关闭")
	}

	// Check if rotation is needed
	if w.maxSize > 0 && w.currentSize+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("轮转日志文件失败: %w", err)
		}
	}

	n, err = w.file.Write(p)
	w.currentSize += int64(n)
	return n, err
}

// Close closes the log file.
func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// rotate rotates the log file.
// Current file is renamed to .1, .1 to .2, etc.
// If maxBackups is 0, the current file is simply truncated.
// Caller must hold w.mu.
func (w *RotatingFileWriter) rotate() error {
	// Close current file
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}

	if w.maxBackups <= 0 {
		// No backups: just truncate the file
		f, err := os.Create(w.path)
		if err != nil {
			return fmt.Errorf("创建日志文件失败: %w", err)
		}
		w.file = f
		w.currentSize = 0
		return nil
	}

	// Delete the oldest backup (slot maxBackups)
	oldest := fmt.Sprintf("%s.%d", w.path, w.maxBackups)
	os.Remove(oldest)

	// Shift backups: move from slot i-1 to slot i, from high to low
	// This way we don't overwrite files we still need to move
	for i := w.maxBackups; i > 1; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i-1)
		dst := fmt.Sprintf("%s.%d", w.path, i)
		if _, err := os.Stat(src); err == nil {
			// Remove destination if it exists (shouldn't after deleting oldest)
			os.Remove(dst)
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("重命名备份文件失败: %w", err)
			}
		}
	}

	// Shift current log file to .1
	backup1 := fmt.Sprintf("%s.1", w.path)
	if _, err := os.Stat(w.path); err == nil {
		os.Remove(backup1)
		if err := os.Rename(w.path, backup1); err != nil {
			return fmt.Errorf("重命名当前日志文件失败: %w", err)
		}
	}

	// Create new log file
	f, err := os.Create(w.path)
	if err != nil {
		return fmt.Errorf("创建新日志文件失败: %w", err)
	}
	w.file = f
	w.currentSize = 0

	return nil
}

// --- Progress logging ---

// ProgressLogger provides progress logging for long-running operations.
type ProgressLogger struct {
	mu       sync.Mutex
	logger   *Logger
	prefix   string
	total    int64
	current  int64
	lastPct  float64
	lastTime time.Time
	interval time.Duration // minimum time between progress logs
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
	p.mu.Lock()
	p.current = current

	now := time.Now()
	if now.Sub(p.lastTime) < p.interval && current < p.total {
		p.mu.Unlock()
		return
	}
	p.lastTime = now

	var pct float64
	if p.total > 0 {
		pct = float64(current) / float64(p.total) * 100
	}

	// Only log if percentage changed significantly
	if p.total > 0 && pct-p.lastPct < 1 && current < p.total {
		p.mu.Unlock()
		return
	}
	p.lastPct = pct
	prefix := p.prefix
	total := p.total
	p.mu.Unlock()

	if total > 0 {
		p.logger.Info("%s: %.1f%% (%s / %s)", prefix, pct, util.FormatSize(current), util.FormatSize(total))
	} else {
		p.logger.Info("%s: %s", prefix, util.FormatSize(current))
	}
}

// Done marks the progress as complete.
func (p *ProgressLogger) Done() {
	p.mu.Lock()
	p.current = p.total
	prefix := p.prefix
	total := p.total
	p.mu.Unlock()
	if total > 0 {
		p.logger.Info("%s: 100%% 完成 (%s)", prefix, util.FormatSize(total))
	} else {
		p.logger.Info("%s: 完成", prefix)
	}
}
