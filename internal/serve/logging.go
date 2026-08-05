package serve

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/util"
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

// Unwrap returns the underlying ResponseWriter, allowing http.ResponseController
// to traverse the wrapper chain and access the original writer's capabilities
// (e.g., SetWriteDeadline for long-running downloads).
func (w *responseWriterWrapper) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
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
	args := []interface{}{r.Method, reqPath, status, util.FormatSize(respSize), duration.Round(time.Millisecond), clientIP}

	switch {
	case status >= 500:
		h.logger.Error(msg+" \"%s\"", append(args, r.UserAgent())...)
	case status >= 400:
		h.logger.Warn(msg+" \"%s\"", append(args, r.UserAgent())...)
	default:
		h.logger.Info(msg, args...)
	}
}

// authMiddleware enforces bearer token authentication for /api/ routes.
// The root HTML page ("/") is always accessible without authentication.
type authMiddleware struct {
	next   http.Handler
	token  string
	logger *bmlog.Logger
}

// ServeHTTP checks the Authorization header for /api/ requests.
// Non-API routes (e.g. "/") pass through without authentication.
func (a *authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only protect /api/ routes; root page is always open
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		a.next.ServeHTTP(w, r)
		return
	}

	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) <= len(prefix) || auth[:len(prefix)] != prefix || auth[len(prefix):] != a.token {
		w.Header().Set("WWW-Authenticate", `Bearer realm="bws serve"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		a.logger.Warn("[http] 认证失败: %s %s (来自 %s)", r.Method, r.URL.Path, r.RemoteAddr)
		return
	}

	a.next.ServeHTTP(w, r)
}
