package serve

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"time"

	bmlog "github.com/bws/bws/internal/log"
)

// responseWriterWrapper wraps http.ResponseWriter to capture the status code
// and response size, and implements http.Flusher and http.Hijacker for
// SSE/streaming support.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
	bytesWritten int64
}

// newResponseWriterWrapper creates a new responseWriterWrapper with default status 200.
func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code and delegates to the underlying ResponseWriter.
func (w *responseWriterWrapper) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.statusCode = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

// Write ensures WriteHeader is called before writing the body and tracks the
// number of bytes written for logging purposes.
func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// Flush implements http.Flusher for SSE/streaming support.
func (w *responseWriterWrapper) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker for WebSocket support.
func (w *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

// loggingHandler is an HTTP middleware that logs each request.
type loggingHandler struct {
	next   http.Handler
	logger *bmlog.Logger
	name   string
}

// ServeHTTP logs each HTTP request with a unified format.
// It skips /api/v1/status (health check) to avoid log noise.
// Format: [http] METHOD path status size duration clientIP "userAgent"
func (h *loggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip health check endpoint
	if r.URL.Path == "/api/v1/status" {
		h.next.ServeHTTP(w, r)
		return
	}

	start := time.Now()
	wrapped := newResponseWriterWrapper(w)

	h.next.ServeHTTP(wrapped, r)

	duration := time.Since(start)
	respSize := wrapped.bytesWritten

	// Build the request path with query string for more context.
	reqPath := r.URL.Path
	if r.URL.RawQuery != "" {
		reqPath = reqPath + "?" + r.URL.RawQuery
	}

	// Extract client IP from RemoteAddr (strip port).
	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	// Determine log level based on status code.
	status := wrapped.statusCode
	msg := "[http] %s %s %d %s %v %s"
	args := []interface{}{r.Method, reqPath, status, formatSize(respSize), duration.Round(time.Millisecond), clientIP}

	switch {
	case status >= 500:
		h.logger.Error(msg+" \"%s\"", append(args, r.UserAgent())...)
	case status >= 400:
		h.logger.Warn(msg+" \"%s\"", append(args, r.UserAgent())...)
	default:
		h.logger.Info(msg, args...)
	}
}
